package providers

import (
	cmm "c_trd/common"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

// Reglas genericas sobre series de observaciones. Separan el "decidir" del "extraer": cada proveedor tiene un
// extractor (responseBody -> []Observation) y las reglas no conocen ningun proveedor. Todas evaluan la serie a una
// fecha (asOf): solo ven las observaciones con At menor o igual a asOf, asi la misma regla sirve para la evaluacion
// del cron (asOf = ahora) y para recalcular el pasado (backtesting, Persist)

// Observation punto de una serie de un proveedor: la fecha propia del dato en UTC y su valor
type Observation struct {
	At    time.Time
	Value float64
}

// RuleResult decision de una regla: si se cumple, el valor medido (depende de la regla: ultimo valor, pendiente,
// percentil o z-score) y la fecha de la ultima observacion que vio la regla
type RuleResult struct {
	Accepted bool
	Value    float64
	At       time.Time
}

// SeriesRule regla ya configurada (con sus umbrales) que se evalua sobre la serie a la fecha asOf. Persist la recibe
// para evaluarla en cada uno de los ultimos puntos
type SeriesRule func(series []Observation, asOf time.Time) (RuleResult, *cmm.ErrorHandler)

// CrossDirection sentido del cruce de Cross
type CrossDirection string

const (
	// CrossUp el penultimo punto esta en el nivel o debajo y el ultimo arriba
	CrossUp CrossDirection = "UP"
	// CrossDown el penultimo punto esta en el nivel o arriba y el ultimo debajo
	CrossDown CrossDirection = "DOWN"
)

// AboveWithHysteresis recorre la serie con dos niveles: se enciende cuando un valor supera onLevel y solo se apaga
// cuando un valor baja de offLevel; entre ambos conserva el estado anterior. Asi un valor que oscila alrededor de un
// unico umbral no hace cambiar la condicion en cada evaluacion. Empieza apagada. Value es el ultimo valor
func AboveWithHysteresis(series []Observation, asOf time.Time, onLevel, offLevel float64) (RuleResult, *cmm.ErrorHandler) {
	if offLevel > onLevel {
		return RuleResult{}, cmm.NewErrorHandler(fmt.Errorf("onLevel %v offLevel %v", onLevel, offLevel), "The off level of the hysteresis can not be greater than the on level", cmm.LevelError, "PCSRLE003")
	}

	visible, err := visibleSeries(series, asOf, 1)
	if !err.IsEmtpy() {
		return RuleResult{}, err
	}

	isOn := false
	for _, observation := range visible {
		turnsOn := !isOn && observation.Value > onLevel
		turnsOff := isOn && observation.Value < offLevel
		if turnsOn {
			isOn = true
		}
		if turnsOff {
			isOn = false
		}
	}

	last := visible[len(visible)-1]
	result := RuleResult{Accepted: isOn, Value: last.Value, At: last.At}
	return result, cmm.NewEmptyErrorHandler()
}

// Below el ultimo valor es menor al umbral. Value es el ultimo valor
func Below(series []Observation, asOf time.Time, threshold float64) (RuleResult, *cmm.ErrorHandler) {
	visible, err := visibleSeries(series, asOf, 1)
	if !err.IsEmtpy() {
		return RuleResult{}, err
	}

	last := visible[len(visible)-1]
	isBelow := last.Value < threshold

	result := RuleResult{Accepted: isBelow, Value: last.Value, At: last.At}
	return result, cmm.NewEmptyErrorHandler()
}

// Cross el ultimo punto cruzo el nivel en el sentido indicado respecto al penultimo. Solo se cumple en el punto del
// cruce: si la serie se queda del otro lado, la siguiente observacion ya no cruza. Value es el ultimo valor
func Cross(series []Observation, asOf time.Time, level float64, direction CrossDirection) (RuleResult, *cmm.ErrorHandler) {
	if direction != CrossUp && direction != CrossDown {
		return RuleResult{}, cmm.NewErrorHandler(fmt.Errorf("direction %q", direction), "The cross direction must be UP or DOWN", cmm.LevelError, "PCSRLE004")
	}

	visible, err := visibleSeries(series, asOf, 2)
	if !err.IsEmtpy() {
		return RuleResult{}, err
	}

	previous := visible[len(visible)-2]
	last := visible[len(visible)-1]
	crossedUp := direction == CrossUp && previous.Value <= level && last.Value > level
	crossedDown := direction == CrossDown && previous.Value >= level && last.Value < level

	result := RuleResult{Accepted: crossedUp || crossedDown, Value: last.Value, At: last.At}
	return result, cmm.NewEmptyErrorHandler()
}

// SlopeAbove la pendiente de la regresion lineal de la serie (unidades por dia) es mayor a minSlope y el ajuste explica
// al menos minR2 de la varianza (R² de 0 a 1), asi una serie con mucho ruido no cuenta como tendencia. El eje x son los
// dias desde la primera observacion. Pide 3 puntos porque con 2 el R² siempre es 1. Value es la pendiente
func SlopeAbove(series []Observation, asOf time.Time, minSlope, minR2 float64) (RuleResult, *cmm.ErrorHandler) {
	if minR2 < 0 || minR2 > 1 || math.IsNaN(minR2) {
		return RuleResult{}, cmm.NewErrorHandler(fmt.Errorf("minR2 %v", minR2), "The minimum R² must be between 0 and 1", cmm.LevelError, "PCSRLE005")
	}

	visible, err := visibleSeries(series, asOf, 3)
	if !err.IsEmtpy() {
		return RuleResult{}, err
	}

	points := make([]seriesPoint, len(visible))
	for i, observation := range visible {
		points[i] = seriesPoint{X: observation.At.Sub(visible[0].At).Hours() / 24, Y: observation.Value}
	}
	slope, rSquared := linearRegression(points)
	isRising := slope > minSlope && rSquared >= minR2

	last := visible[len(visible)-1]
	result := RuleResult{Accepted: isRising, Value: slope, At: last.At}
	return result, cmm.NewEmptyErrorHandler()
}

// PercentileAbove el percentil del ultimo valor respecto a los anteriores (0 a 100, ver percentileRank) es mayor a
// percentile. Value es el percentil
func PercentileAbove(series []Observation, asOf time.Time, percentile float64) (RuleResult, *cmm.ErrorHandler) {
	if percentile < 0 || percentile > 100 || math.IsNaN(percentile) {
		return RuleResult{}, cmm.NewErrorHandler(fmt.Errorf("percentile %v", percentile), "The percentile must be between 0 and 100", cmm.LevelError, "PCSRLE006")
	}

	visible, err := visibleSeries(series, asOf, 2)
	if !err.IsEmtpy() {
		return RuleResult{}, err
	}

	last := visible[len(visible)-1]
	rank := percentileRank(observationValues(visible[:len(visible)-1]), last.Value)
	isAbove := rank > percentile

	result := RuleResult{Accepted: isAbove, Value: rank, At: last.At}
	return result, cmm.NewEmptyErrorHandler()
}

// PercentileBelow el percentil del ultimo valor respecto a los anteriores (0 a 100) es menor a percentile.
// Value es el percentil
func PercentileBelow(series []Observation, asOf time.Time, percentile float64) (RuleResult, *cmm.ErrorHandler) {
	if percentile < 0 || percentile > 100 || math.IsNaN(percentile) {
		return RuleResult{}, cmm.NewErrorHandler(fmt.Errorf("percentile %v", percentile), "The percentile must be between 0 and 100", cmm.LevelError, "PCSRLE007")
	}

	visible, err := visibleSeries(series, asOf, 2)
	if !err.IsEmtpy() {
		return RuleResult{}, err
	}

	last := visible[len(visible)-1]
	rank := percentileRank(observationValues(visible[:len(visible)-1]), last.Value)
	isBelow := rank < percentile

	result := RuleResult{Accepted: isBelow, Value: rank, At: last.At}
	return result, cmm.NewEmptyErrorHandler()
}

// ZScoreAbove el z-score del ultimo valor respecto a la ventana movil de los window valores anteriores (sin incluirlo)
// es mayor a threshold. Una ventana plana (desviacion 0) no tiene z-score y no se cumple con Value 0.
// Value es el z-score
func ZScoreAbove(series []Observation, asOf time.Time, window int, threshold float64) (RuleResult, *cmm.ErrorHandler) {
	if window < 2 {
		return RuleResult{}, cmm.NewErrorHandler(fmt.Errorf("window %d", window), "The z-score window must have at least 2 points", cmm.LevelError, "PCSRLE008")
	}

	visible, err := visibleSeries(series, asOf, window+1)
	if !err.IsEmtpy() {
		return RuleResult{}, err
	}

	last := visible[len(visible)-1]
	reference := observationValues(visible[len(visible)-1-window : len(visible)-1])
	mean, stdDev := meanAndStdDev(reference)
	if stdDev == 0 {
		return RuleResult{Accepted: false, Value: 0, At: last.At}, cmm.NewEmptyErrorHandler()
	}

	zScore := (last.Value - mean) / stdDev
	isAbove := zScore > threshold

	result := RuleResult{Accepted: isAbove, Value: zScore, At: last.At}
	return result, cmm.NewEmptyErrorHandler()
}

// ZScoreBelow el z-score del ultimo valor respecto a la ventana movil de los window valores anteriores es menor a
// threshold (ej: -2). Una ventana plana no se cumple con Value 0. Value es el z-score
func ZScoreBelow(series []Observation, asOf time.Time, window int, threshold float64) (RuleResult, *cmm.ErrorHandler) {
	if window < 2 {
		return RuleResult{}, cmm.NewErrorHandler(fmt.Errorf("window %d", window), "The z-score window must have at least 2 points", cmm.LevelError, "PCSRLE009")
	}

	visible, err := visibleSeries(series, asOf, window+1)
	if !err.IsEmtpy() {
		return RuleResult{}, err
	}

	last := visible[len(visible)-1]
	reference := observationValues(visible[len(visible)-1-window : len(visible)-1])
	mean, stdDev := meanAndStdDev(reference)
	if stdDev == 0 {
		return RuleResult{Accepted: false, Value: 0, At: last.At}, cmm.NewEmptyErrorHandler()
	}

	zScore := (last.Value - mean) / stdDev
	isBelow := zScore < threshold

	result := RuleResult{Accepted: isBelow, Value: zScore, At: last.At}
	return result, cmm.NewEmptyErrorHandler()
}

// Persist la regla se cumple a la fecha de cada uno de los ultimos points puntos de la serie (inclusive el ultimo).
// Con points 1 es la regla tal cual. Value y At son los de la regla en el ultimo punto. Si la regla falla en algun
// punto (ej: la ventana de un z-score no alcanza) retorna ese error: la serie debe traer los puntos que pide la regla
// mas points - 1
func Persist(series []Observation, asOf time.Time, points int, rule SeriesRule) (RuleResult, *cmm.ErrorHandler) {
	if points < 1 {
		return RuleResult{}, cmm.NewErrorHandler(fmt.Errorf("points %d", points), "Persist needs at least 1 point", cmm.LevelError, "PCSRLE010")
	}
	if rule == nil {
		return RuleResult{}, cmm.NewErrorHandler(errors.New("rule is nil"), "Persist needs a rule to evaluate", cmm.LevelFatal, "PCSRLE011")
	}

	visible, err := visibleSeries(series, asOf, points)
	if !err.IsEmtpy() {
		return RuleResult{}, err
	}

	// Primero el ultimo punto, que da el valor del resultado, y luego hacia atras hasta el primero que no cumpla
	latest, err := rule(visible, visible[len(visible)-1].At)
	if !err.IsEmtpy() {
		return RuleResult{}, err
	}

	persisted := latest.Accepted
	for i := len(visible) - 2; persisted && i >= len(visible)-points; i-- {
		previous, err := rule(visible, visible[i].At)
		if !err.IsEmtpy() {
			return RuleResult{}, err
		}
		persisted = previous.Accepted
	}

	result := RuleResult{Accepted: persisted, Value: latest.Value, At: latest.At}
	return result, cmm.NewEmptyErrorHandler()
}

// visibleSeries retorna las observaciones con At menor o igual a asOf ordenadas por fecha (sin modificar la serie
// recibida). Falla si algun valor no es finito o si quedan menos de minPoints
func visibleSeries(series []Observation, asOf time.Time, minPoints int) ([]Observation, *cmm.ErrorHandler) {
	visible := make([]Observation, 0, len(series))
	for _, observation := range series {
		if observation.At.After(asOf) {
			continue
		}
		if math.IsNaN(observation.Value) || math.IsInf(observation.Value, 0) {
			return nil, cmm.NewErrorHandler(fmt.Errorf("at %s value %v", observation.At.Format(time.RFC3339), observation.Value), "The series has a value that is not a finite number", cmm.LevelError, "PCSRLE001")
		}
		visible = append(visible, observation)
	}

	if len(visible) < minPoints {
		return nil, cmm.NewErrorHandler(fmt.Errorf("got %d points until %s, need %d", len(visible), asOf.Format(time.RFC3339), minPoints), "The series has not enough observations for the rule", cmm.LevelError, "PCSRLE002")
	}

	sort.SliceStable(visible, func(i, j int) bool { return visible[i].At.Before(visible[j].At) })
	return visible, cmm.NewEmptyErrorHandler()
}

// observationValues valores de las observaciones en el mismo orden
func observationValues(series []Observation) []float64 {
	values := make([]float64, len(series))
	for i, observation := range series {
		values[i] = observation.Value
	}
	return values
}
