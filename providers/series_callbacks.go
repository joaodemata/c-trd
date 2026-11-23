package providers

import (
	cmm "c_trd/common"
	"fmt"
	"math"
	"time"
)

// Decisiones de los callbacks de reglas sobre series: leen los umbrales de parameters_callback, arman la regla y la
// evaluan con Persist a la fecha asOf. Cada callback registrado solo junta el extractor de su proveedor con una de
// estas decisiones. El parametro points (puntos seguidos) es opcional en todas menos en el cruce, que solo se cumple
// en un punto: sin el o con 1 la regla se evalua solo en el ultimo punto

// Nombres de los parametros de los callbacks de reglas sobre series
const (
	seriesOnLevelParameterName   = "onLevel"
	seriesOffLevelParameterName  = "offLevel"
	seriesThresholdParameterName = "threshold"
	seriesLevelParameterName     = "level"
	seriesMinSlopeParameterName  = "minSlope"
	seriesMinR2ParameterName     = "minR2"
	seriesPercentileName         = "percentile"
	seriesWindowParameterName    = "window"
	seriesZScoreParameterName    = "zScore"
	seriesPointsParameterName    = "points"

	seriesDefaultPersistPoints = 1
)

// Rangos de los parametros al crear la condicion. Son variables porque CallbackParameter guarda punteros
var (
	seriesMinR2MinValue      = 0.0
	seriesMinR2MaxValue      = 1.0
	seriesPercentileMinValue = 0.0
	seriesPercentileMaxValue = 100.0
	seriesWindowMinValue     = 2.0
	seriesWindowMaxValue     = 1000.0
	seriesZScoreMinValue     = -10.0
	seriesZScoreMaxValue     = 10.0
	seriesPointsMinValue     = 1.0
	seriesPointsMaxValue     = 100.0

	seriesDefaultMinR2  = "0.5"
	seriesDefaultWindow = "20"
	seriesDefaultPoints = "1"
)

// seriesPointsParameter cantidad de puntos seguidos en los que se debe cumplir la regla (Persist)
var seriesPointsParameter = CallbackParameter{
	Name:         seriesPointsParameterName,
	Label:        "Puntos seguidos",
	Description:  "Cantidad de observaciones seguidas, hasta la ultima, en las que se debe cumplir la regla (1 a 100). La respuesta debe traer esos puntos extra",
	DataType:     cmm.ApiParameterTypeInteger,
	IsRequired:   false,
	DefaultValue: &seriesDefaultPoints,
	MinValue:     &seriesPointsMinValue,
	MaxValue:     &seriesPointsMaxValue,
}

// seriesHysteresisParameters parametros de los callbacks AboveWithHysteresis
var seriesHysteresisParameters = []CallbackParameter{
	{
		Name:        seriesOnLevelParameterName,
		Label:       "Nivel de encendido",
		Description: "La condicion se cumple cuando un valor de la serie supera este nivel",
		DataType:    cmm.ApiParameterTypeFloat,
		IsRequired:  true,
	},
	{
		Name:        seriesOffLevelParameterName,
		Label:       "Nivel de apagado",
		Description: "La condicion deja de cumplirse solo cuando un valor baja de este nivel (menor o igual al de encendido)",
		DataType:    cmm.ApiParameterTypeFloat,
		IsRequired:  true,
	},
	seriesPointsParameter,
}

// seriesBelowParameters parametros de los callbacks Below
var seriesBelowParameters = []CallbackParameter{
	{
		Name:        seriesThresholdParameterName,
		Label:       "Umbral",
		Description: "La condicion se cumple cuando el ultimo valor de la serie es menor a este umbral",
		DataType:    cmm.ApiParameterTypeFloat,
		IsRequired:  true,
	},
	seriesPointsParameter,
}

// seriesCrossParameters parametros de los callbacks de cruce. No llevan points porque un cruce dura un solo punto
var seriesCrossParameters = []CallbackParameter{
	{
		Name:        seriesLevelParameterName,
		Label:       "Nivel",
		Description: "Nivel que la serie debe cruzar entre la penultima y la ultima observacion",
		DataType:    cmm.ApiParameterTypeFloat,
		IsRequired:  true,
	},
}

// seriesSlopeParameters parametros de los callbacks SlopeAbove
var seriesSlopeParameters = []CallbackParameter{
	{
		Name:        seriesMinSlopeParameterName,
		Label:       "Pendiente minima (por dia)",
		Description: "La pendiente de la regresion lineal, en unidades de la serie por dia, debe ser mayor a este valor (negativo para tolerar caidas leves)",
		DataType:    cmm.ApiParameterTypeFloat,
		IsRequired:  true,
	},
	{
		Name:         seriesMinR2ParameterName,
		Label:        "R² minimo",
		Description:  "Proporcion de la varianza que debe explicar la tendencia (0 a 1); mas alto exige una serie menos ruidosa",
		DataType:     cmm.ApiParameterTypeFloat,
		IsRequired:   true,
		DefaultValue: &seriesDefaultMinR2,
		MinValue:     &seriesMinR2MinValue,
		MaxValue:     &seriesMinR2MaxValue,
	},
	seriesPointsParameter,
}

// seriesPercentileParameters parametros de los callbacks PercentileAbove y PercentileBelow
var seriesPercentileParameters = []CallbackParameter{
	{
		Name:        seriesPercentileName,
		Label:       "Percentil",
		Description: "Percentil (0 a 100) del ultimo valor respecto a los anteriores de la serie contra el que se compara",
		DataType:    cmm.ApiParameterTypeFloat,
		IsRequired:  true,
		MinValue:    &seriesPercentileMinValue,
		MaxValue:    &seriesPercentileMaxValue,
	},
	seriesPointsParameter,
}

// seriesZScoreParameters parametros de los callbacks ZScoreAbove y ZScoreBelow
var seriesZScoreParameters = []CallbackParameter{
	{
		Name:         seriesWindowParameterName,
		Label:        "Ventana (puntos)",
		Description:  "Cantidad de observaciones anteriores a la ultima (2 a 1000) con las que se calculan la media y la desviacion",
		DataType:     cmm.ApiParameterTypeInteger,
		IsRequired:   true,
		DefaultValue: &seriesDefaultWindow,
		MinValue:     &seriesWindowMinValue,
		MaxValue:     &seriesWindowMaxValue,
	},
	{
		Name:        seriesZScoreParameterName,
		Label:       "Z-score",
		Description: "Cantidad de desviaciones estandar (-10 a 10) contra la que se compara el z-score del ultimo valor",
		DataType:    cmm.ApiParameterTypeFloat,
		IsRequired:  true,
		MinValue:    &seriesZScoreMinValue,
		MaxValue:    &seriesZScoreMaxValue,
	},
	seriesPointsParameter,
}

// aboveWithHysteresisDecision AboveWithHysteresis con onLevel y offLevel, durante points puntos seguidos
func aboveWithHysteresisDecision(series []Observation, parameters map[string]any, asOf time.Time) (CallbackResult, *cmm.ErrorHandler) {
	onLevel, err := seriesFloatParameter(parameters, seriesOnLevelParameterName)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}
	offLevel, err := seriesFloatParameter(parameters, seriesOffLevelParameterName)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}
	points, err := seriesPersistPoints(parameters)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	rule := func(visible []Observation, at time.Time) (RuleResult, *cmm.ErrorHandler) {
		ruleResult, errRule := AboveWithHysteresis(visible, at, onLevel, offLevel)
		return ruleResult, errRule
	}
	ruleResult, err := Persist(series, asOf, points, rule)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	result := callbackResultFromRule(ruleResult)
	return result, cmm.NewEmptyErrorHandler()
}

// belowDecision Below con threshold, durante points puntos seguidos
func belowDecision(series []Observation, parameters map[string]any, asOf time.Time) (CallbackResult, *cmm.ErrorHandler) {
	threshold, err := seriesFloatParameter(parameters, seriesThresholdParameterName)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}
	points, err := seriesPersistPoints(parameters)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	rule := func(visible []Observation, at time.Time) (RuleResult, *cmm.ErrorHandler) {
		ruleResult, errRule := Below(visible, at, threshold)
		return ruleResult, errRule
	}
	ruleResult, err := Persist(series, asOf, points, rule)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	result := callbackResultFromRule(ruleResult)
	return result, cmm.NewEmptyErrorHandler()
}

// crossDecision Cross con level en el sentido indicado. El sentido lo fija cada callback, no es un parametro
func crossDecision(series []Observation, parameters map[string]any, asOf time.Time, direction CrossDirection) (CallbackResult, *cmm.ErrorHandler) {
	level, err := seriesFloatParameter(parameters, seriesLevelParameterName)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	ruleResult, err := Cross(series, asOf, level, direction)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	result := callbackResultFromRule(ruleResult)
	return result, cmm.NewEmptyErrorHandler()
}

// slopeAboveDecision SlopeAbove con minSlope y minR2, durante points puntos seguidos
func slopeAboveDecision(series []Observation, parameters map[string]any, asOf time.Time) (CallbackResult, *cmm.ErrorHandler) {
	minSlope, err := seriesFloatParameter(parameters, seriesMinSlopeParameterName)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}
	minR2, err := seriesFloatParameter(parameters, seriesMinR2ParameterName)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}
	points, err := seriesPersistPoints(parameters)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	rule := func(visible []Observation, at time.Time) (RuleResult, *cmm.ErrorHandler) {
		ruleResult, errRule := SlopeAbove(visible, at, minSlope, minR2)
		return ruleResult, errRule
	}
	ruleResult, err := Persist(series, asOf, points, rule)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	result := callbackResultFromRule(ruleResult)
	return result, cmm.NewEmptyErrorHandler()
}

// percentileAboveDecision PercentileAbove con percentile, durante points puntos seguidos
func percentileAboveDecision(series []Observation, parameters map[string]any, asOf time.Time) (CallbackResult, *cmm.ErrorHandler) {
	percentile, err := seriesFloatParameter(parameters, seriesPercentileName)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}
	points, err := seriesPersistPoints(parameters)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	rule := func(visible []Observation, at time.Time) (RuleResult, *cmm.ErrorHandler) {
		ruleResult, errRule := PercentileAbove(visible, at, percentile)
		return ruleResult, errRule
	}
	ruleResult, err := Persist(series, asOf, points, rule)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	result := callbackResultFromRule(ruleResult)
	return result, cmm.NewEmptyErrorHandler()
}

// percentileBelowDecision PercentileBelow con percentile, durante points puntos seguidos
func percentileBelowDecision(series []Observation, parameters map[string]any, asOf time.Time) (CallbackResult, *cmm.ErrorHandler) {
	percentile, err := seriesFloatParameter(parameters, seriesPercentileName)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}
	points, err := seriesPersistPoints(parameters)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	rule := func(visible []Observation, at time.Time) (RuleResult, *cmm.ErrorHandler) {
		ruleResult, errRule := PercentileBelow(visible, at, percentile)
		return ruleResult, errRule
	}
	ruleResult, err := Persist(series, asOf, points, rule)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	result := callbackResultFromRule(ruleResult)
	return result, cmm.NewEmptyErrorHandler()
}

// zScoreAboveDecision ZScoreAbove con window y zScore, durante points puntos seguidos
func zScoreAboveDecision(series []Observation, parameters map[string]any, asOf time.Time) (CallbackResult, *cmm.ErrorHandler) {
	window, err := seriesIntegerParameter(parameters, seriesWindowParameterName)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}
	threshold, err := seriesFloatParameter(parameters, seriesZScoreParameterName)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}
	points, err := seriesPersistPoints(parameters)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	rule := func(visible []Observation, at time.Time) (RuleResult, *cmm.ErrorHandler) {
		ruleResult, errRule := ZScoreAbove(visible, at, window, threshold)
		return ruleResult, errRule
	}
	ruleResult, err := Persist(series, asOf, points, rule)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	result := callbackResultFromRule(ruleResult)
	return result, cmm.NewEmptyErrorHandler()
}

// zScoreBelowDecision ZScoreBelow con window y zScore, durante points puntos seguidos
func zScoreBelowDecision(series []Observation, parameters map[string]any, asOf time.Time) (CallbackResult, *cmm.ErrorHandler) {
	window, err := seriesIntegerParameter(parameters, seriesWindowParameterName)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}
	threshold, err := seriesFloatParameter(parameters, seriesZScoreParameterName)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}
	points, err := seriesPersistPoints(parameters)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	rule := func(visible []Observation, at time.Time) (RuleResult, *cmm.ErrorHandler) {
		ruleResult, errRule := ZScoreBelow(visible, at, window, threshold)
		return ruleResult, errRule
	}
	ruleResult, err := Persist(series, asOf, points, rule)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	result := callbackResultFromRule(ruleResult)
	return result, cmm.NewEmptyErrorHandler()
}

// callbackResultFromRule convierte el resultado de la regla al del callback. AsOf es la fecha de la ultima observacion
// que vio la regla, asi MaxDataAge valida la antiguedad del dato del proveedor
func callbackResultFromRule(ruleResult RuleResult) CallbackResult {
	asOf := ruleResult.At.UTC()
	return CallbackResult{Accepted: ruleResult.Accepted, Value: ruleResult.Value, AsOf: &asOf}
}

// seriesFloatParameter parametro numerico de parameters_callback. Los rangos ya se validaron al crear la condicion
func seriesFloatParameter(parameters map[string]any, name string) (float64, *cmm.ErrorHandler) {
	number, ok := biquoteParameterNumber(parameters[name])
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, cmm.NewErrorHandler(fmt.Errorf("%s %T %v", name, parameters[name], parameters[name]), "A callback parameter of the series rule is missing or is not a number", cmm.LevelError, "PCSCBE001")
	}
	return number, cmm.NewEmptyErrorHandler()
}

// seriesIntegerParameter parametro entero de parameters_callback
func seriesIntegerParameter(parameters map[string]any, name string) (int, *cmm.ErrorHandler) {
	number, ok := biquoteParameterNumber(parameters[name])
	if !ok || number != math.Trunc(number) {
		return 0, cmm.NewErrorHandler(fmt.Errorf("%s %T %v", name, parameters[name], parameters[name]), "A callback parameter of the series rule is missing or is not an integer", cmm.LevelError, "PCSCBE002")
	}
	return int(number), cmm.NewEmptyErrorHandler()
}

// seriesPersistPoints parametro points: 1 si la condicion no lo trae (es opcional)
func seriesPersistPoints(parameters map[string]any) (int, *cmm.ErrorHandler) {
	if _, exists := parameters[seriesPointsParameterName]; !exists {
		return seriesDefaultPersistPoints, cmm.NewEmptyErrorHandler()
	}

	number, ok := biquoteParameterNumber(parameters[seriesPointsParameterName])
	if !ok || number != math.Trunc(number) {
		return 0, cmm.NewErrorHandler(fmt.Errorf("points %T %v", parameters[seriesPointsParameterName], parameters[seriesPointsParameterName]), "The callback points parameter is not an integer", cmm.LevelError, "PCSCBE003")
	}
	return int(number), cmm.NewEmptyErrorHandler()
}
