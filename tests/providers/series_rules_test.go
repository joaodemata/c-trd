package providers_test

import (
	"math"
	"slices"
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/providers"
	"c_trd/tests/testutil"
)

// seriesStart fecha de la primera observacion de las series de prueba
var seriesStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// daily serie con una observacion por dia desde seriesStart con los valores dados
func daily(values ...float64) []providers.Observation {
	series := make([]providers.Observation, len(values))
	for i, value := range values {
		series[i] = providers.Observation{At: seriesStart.AddDate(0, 0, i), Value: value}
	}
	return series
}

// dayAt fecha de la observacion i de una serie daily
func dayAt(i int) time.Time {
	return seriesStart.AddDate(0, 0, i)
}

// farFuture asOf que ve todas las observaciones de la serie
var farFuture = time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

// approxEqual compara floats con tolerancia para los calculos de regresion, percentil y z-score
func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// requireRuleResult valida la decision, el valor medido y la fecha de la ultima observacion vista
func requireRuleResult(t *testing.T, got providers.RuleResult, wantAccepted bool, wantValue float64, wantAt time.Time) {
	t.Helper()

	if got.Accepted != wantAccepted {
		t.Errorf("accepted = %v, want %v", got.Accepted, wantAccepted)
	}
	if !approxEqual(got.Value, wantValue) {
		t.Errorf("value = %v, want %v", got.Value, wantValue)
	}
	if !got.At.Equal(wantAt) {
		t.Errorf("at = %s, want %s", got.At.Format(time.RFC3339), wantAt.Format(time.RFC3339))
	}
}

func TestAboveWithHysteresis(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		series       []providers.Observation
		asOf         time.Time
		onLevel      float64
		offLevel     float64
		wantAccepted bool
		wantValue    float64
		wantAt       time.Time
	}{
		{"supera el encendido", daily(10, 50, 71), farFuture, 70, 40, true, 71, dayAt(2)},
		{"igual al encendido no enciende", daily(10, 70), farFuture, 70, 40, false, 70, dayAt(1)},
		// Encendida en 75, el 50 queda entre los niveles y conserva el estado
		{"entre niveles conserva encendida", daily(75, 50), farFuture, 70, 40, true, 50, dayAt(1)},
		{"entre niveles conserva apagada", daily(30, 50), farFuture, 70, 40, false, 50, dayAt(1)},
		{"igual al apagado no apaga", daily(75, 40), farFuture, 70, 40, true, 40, dayAt(1)},
		{"baja del apagado", daily(75, 39), farFuture, 70, 40, false, 39, dayAt(1)},
		{"apagada y vuelve a encender", daily(75, 30, 50, 80), farFuture, 70, 40, true, 80, dayAt(3)},
		{"un solo umbral sin banda", daily(71, 69), farFuture, 70, 70, false, 69, dayAt(1)},
		// A la fecha del dia 1 todavia no llego el 30 que la apaga
		{"as of antes del apagado", daily(75, 50, 30), dayAt(1), 70, 40, true, 50, dayAt(1)},
		{"serie desordenada", []providers.Observation{{At: dayAt(1), Value: 50}, {At: dayAt(0), Value: 75}}, farFuture, 70, 40, true, 50, dayAt(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.AboveWithHysteresis(tt.series, tt.asOf, tt.onLevel, tt.offLevel)

			testutil.RequireEmptyError(t, err)
			requireRuleResult(t, got, tt.wantAccepted, tt.wantValue, tt.wantAt)
		})
	}
}

func TestAboveWithHysteresis_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		series   []providers.Observation
		asOf     time.Time
		onLevel  float64
		offLevel float64
		wantCode string
	}{
		{"apagado mayor al encendido", daily(50), farFuture, 40, 70, "PCSRLE003"},
		{"serie vacia", nil, farFuture, 70, 40, "PCSRLE002"},
		{"as of antes de la primera observacion", daily(50), seriesStart.Add(-time.Hour), 70, 40, "PCSRLE002"},
		{"valor NaN", daily(50, math.NaN()), farFuture, 70, 40, "PCSRLE001"},
		{"valor infinito", daily(math.Inf(1)), farFuture, 70, 40, "PCSRLE001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.AboveWithHysteresis(tt.series, tt.asOf, tt.onLevel, tt.offLevel)

			testutil.RequireTrackingCode(t, err, tt.wantCode)
			if got != (providers.RuleResult{}) {
				t.Errorf("result = %+v, un error debe retornar RuleResult{}", got)
			}
		})
	}
}

func TestBelow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		series       []providers.Observation
		asOf         time.Time
		threshold    float64
		wantAccepted bool
		wantValue    float64
		wantAt       time.Time
	}{
		{"menor al umbral", daily(30, 19.9), farFuture, 20, true, 19.9, dayAt(1)},
		{"igual al umbral", daily(30, 20), farFuture, 20, false, 20, dayAt(1)},
		{"mayor al umbral", daily(10, 21), farFuture, 20, false, 21, dayAt(1)},
		{"umbral negativo", daily(-3), farFuture, -2, true, -3, dayAt(0)},
		// El 10 del dia 1 todavia no existe a la fecha del dia 0
		{"as of ignora observaciones futuras", daily(30, 10), dayAt(0), 20, false, 30, dayAt(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.Below(tt.series, tt.asOf, tt.threshold)

			testutil.RequireEmptyError(t, err)
			requireRuleResult(t, got, tt.wantAccepted, tt.wantValue, tt.wantAt)
		})
	}

	t.Run("serie vacia", func(t *testing.T) {
		t.Parallel()

		_, err := providers.Below(nil, farFuture, 20)
		testutil.RequireTrackingCode(t, err, "PCSRLE002")
	})
}

func TestCross(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		series       []providers.Observation
		asOf         time.Time
		level        float64
		direction    providers.CrossDirection
		wantAccepted bool
	}{
		{"cruza hacia arriba", daily(40, 49, 51), farFuture, 50, providers.CrossUp, true},
		{"desde el nivel hacia arriba", daily(50, 51), farFuture, 50, providers.CrossUp, true},
		{"llega al nivel sin superarlo", daily(49, 50), farFuture, 50, providers.CrossUp, false},
		{"ya estaba arriba", daily(51, 52), farFuture, 50, providers.CrossUp, false},
		{"cruce hacia abajo no cuenta como arriba", daily(51, 49), farFuture, 50, providers.CrossUp, false},
		{"cruza hacia abajo", daily(51, 49), farFuture, 50, providers.CrossDown, true},
		{"desde el nivel hacia abajo", daily(50, 49), farFuture, 50, providers.CrossDown, true},
		{"ya estaba abajo", daily(49, 48), farFuture, 50, providers.CrossDown, false},
		// El cruce fue en el dia 1; a la fecha del dia 2 la serie sigue arriba pero ya no cruza
		{"el cruce dura un punto", daily(49, 51, 52), farFuture, 50, providers.CrossUp, false},
		{"as of en el punto del cruce", daily(49, 51, 52), dayAt(1), 50, providers.CrossUp, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.Cross(tt.series, tt.asOf, tt.level, tt.direction)

			testutil.RequireEmptyError(t, err)
			if got.Accepted != tt.wantAccepted {
				t.Errorf("accepted = %v, want %v", got.Accepted, tt.wantAccepted)
			}
		})
	}

	errorTests := []struct {
		name      string
		series    []providers.Observation
		direction providers.CrossDirection
		wantCode  string
	}{
		{"sentido desconocido", daily(49, 51), providers.CrossDirection("SIDEWAYS"), "PCSRLE004"},
		{"un solo punto", daily(51), providers.CrossUp, "PCSRLE002"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := providers.Cross(tt.series, farFuture, 50, tt.direction)
			testutil.RequireTrackingCode(t, err, tt.wantCode)
		})
	}
}

func TestSlopeAbove(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		series       []providers.Observation
		minSlope     float64
		minR2        float64
		wantAccepted bool
		wantSlope    float64
	}{
		{"recta que sube 1 por dia", daily(10, 11, 12, 13), 0.5, 0.9, true, 1},
		{"pendiente igual al minimo", daily(10, 11, 12, 13), 1, 0, false, 1},
		{"recta que baja", daily(13, 12, 11, 10), 0, 0, false, -1},
		{"pendiente negativa tolerada", daily(13, 12, 11, 10), -2, 0.9, true, -1},
		// Sube en promedio (0.4 por dia) pero con mucho ruido: R² = 16 / (10 * 160.8) ≈ 0.01 no alcanza 0.5
		{"sube con ruido y R² bajo", daily(10, 20, 8, 22, 11), 0, 0.5, false, 0.4},
		{"sube con ruido sin exigir R²", daily(10, 20, 8, 22, 11), 0, 0, true, 0.4},
		{"serie plana", daily(5, 5, 5), -1, 0, true, 0},
		{"serie plana con R² exigido", daily(5, 5, 5), -1, 0.1, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.SlopeAbove(tt.series, farFuture, tt.minSlope, tt.minR2)

			testutil.RequireEmptyError(t, err)
			requireRuleResult(t, got, tt.wantAccepted, tt.wantSlope, tt.series[len(tt.series)-1].At)
		})
	}

	t.Run("el eje x son dias aunque falten observaciones", func(t *testing.T) {
		t.Parallel()

		// Sube 1 por dia con el dia 2 faltante: la pendiente sigue siendo 1 y no 1.5
		series := []providers.Observation{{At: dayAt(0), Value: 0}, {At: dayAt(1), Value: 1}, {At: dayAt(3), Value: 3}}
		got, err := providers.SlopeAbove(series, farFuture, 0, 0.99)

		testutil.RequireEmptyError(t, err)
		requireRuleResult(t, got, true, 1, dayAt(3))
	})

	errorTests := []struct {
		name     string
		series   []providers.Observation
		minR2    float64
		wantCode string
	}{
		{"R² negativo", daily(1, 2, 3), -0.1, "PCSRLE005"},
		{"R² mayor a 1", daily(1, 2, 3), 1.1, "PCSRLE005"},
		{"R² NaN", daily(1, 2, 3), math.NaN(), "PCSRLE005"},
		{"dos puntos", daily(1, 2), 0.5, "PCSRLE002"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := providers.SlopeAbove(tt.series, farFuture, 0, tt.minR2)
			testutil.RequireTrackingCode(t, err, tt.wantCode)
		})
	}
}

func TestPercentileRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		series     []providers.Observation
		percentile float64
		wantRank   float64
		wantAbove  bool
		wantBelow  bool
	}{
		{"nuevo maximo", daily(10, 20, 30, 40), 90, 100, true, false},
		{"nuevo minimo", daily(10, 20, 30, 5), 10, 0, false, true},
		// 2 anteriores debajo y 1 igual de 4: (2 + 0.5) / 4 = 62.5
		{"empate cuenta la mitad", daily(10, 20, 30, 40, 30), 50, 62.5, true, false},
		{"igual al percentil no cumple ninguna", daily(10, 30, 20), 50, 50, false, false},
		{"todos iguales", daily(7, 7, 7), 50, 50, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lastAt := tt.series[len(tt.series)-1].At

			above, err := providers.PercentileAbove(tt.series, farFuture, tt.percentile)
			testutil.RequireEmptyError(t, err)
			requireRuleResult(t, above, tt.wantAbove, tt.wantRank, lastAt)

			below, err := providers.PercentileBelow(tt.series, farFuture, tt.percentile)
			testutil.RequireEmptyError(t, err)
			requireRuleResult(t, below, tt.wantBelow, tt.wantRank, lastAt)
		})
	}

	t.Run("as of compara contra los anteriores a la fecha", func(t *testing.T) {
		t.Parallel()

		// A la fecha del dia 2 el 30 es maximo; el 50 del dia 3 aun no existe
		got, err := providers.PercentileAbove(daily(10, 20, 30, 50), dayAt(2), 99)

		testutil.RequireEmptyError(t, err)
		requireRuleResult(t, got, true, 100, dayAt(2))
	})

	errorTests := []struct {
		name       string
		rule       func([]providers.Observation, time.Time, float64) (providers.RuleResult, *cmm.ErrorHandler)
		series     []providers.Observation
		percentile float64
		wantCode   string
	}{
		{"above percentil negativo", providers.PercentileAbove, daily(1, 2), -1, "PCSRLE006"},
		{"above percentil mayor a 100", providers.PercentileAbove, daily(1, 2), 101, "PCSRLE006"},
		{"above un solo punto", providers.PercentileAbove, daily(1), 50, "PCSRLE002"},
		{"below percentil negativo", providers.PercentileBelow, daily(1, 2), -1, "PCSRLE007"},
		{"below percentil NaN", providers.PercentileBelow, daily(1, 2), math.NaN(), "PCSRLE007"},
		{"below un solo punto", providers.PercentileBelow, daily(1), 50, "PCSRLE002"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.rule(tt.series, farFuture, tt.percentile)
			testutil.RequireTrackingCode(t, err, tt.wantCode)
		})
	}
}

func TestZScoreRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		series    []providers.Observation
		window    int
		threshold float64
		wantZ     float64
		wantAbove bool
		wantBelow bool
	}{
		// Ventana 1, 2, 3: media 2 y desviacion muestral 1
		{"muy por encima", daily(1, 2, 3, 5), 3, 2, 3, true, false},
		{"igual al umbral", daily(1, 2, 3, 5), 3, 3, 3, false, false},
		{"muy por debajo", daily(1, 2, 3, -1), 3, -2, -3, false, true},
		{"en la media", daily(1, 2, 3, 2), 3, 0, 0, false, false},
		// Solo cuentan los 3 valores anteriores: el 1000 del inicio queda fuera de la ventana movil
		{"ventana movil", daily(1000, 1, 2, 3, 5), 3, 2, 3, true, false},
		{"ventana plana no tiene z-score", daily(4, 4, 4, 9), 3, -100, 0, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lastAt := tt.series[len(tt.series)-1].At

			above, err := providers.ZScoreAbove(tt.series, farFuture, tt.window, tt.threshold)
			testutil.RequireEmptyError(t, err)
			requireRuleResult(t, above, tt.wantAbove, tt.wantZ, lastAt)

			below, err := providers.ZScoreBelow(tt.series, farFuture, tt.window, tt.threshold)
			testutil.RequireEmptyError(t, err)
			requireRuleResult(t, below, tt.wantBelow, tt.wantZ, lastAt)
		})
	}

	errorTests := []struct {
		name     string
		rule     func([]providers.Observation, time.Time, int, float64) (providers.RuleResult, *cmm.ErrorHandler)
		series   []providers.Observation
		window   int
		wantCode string
	}{
		{"above ventana de 1", providers.ZScoreAbove, daily(1, 2, 3), 1, "PCSRLE008"},
		{"above sin puntos para la ventana", providers.ZScoreAbove, daily(1, 2, 3), 3, "PCSRLE002"},
		{"below ventana de 0", providers.ZScoreBelow, daily(1, 2, 3), 0, "PCSRLE009"},
		{"below sin puntos para la ventana", providers.ZScoreBelow, daily(1, 2, 3), 3, "PCSRLE002"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.rule(tt.series, farFuture, tt.window, 2)
			testutil.RequireTrackingCode(t, err, tt.wantCode)
		})
	}
}

func TestPersist(t *testing.T) {
	t.Parallel()

	below20 := func(series []providers.Observation, asOf time.Time) (providers.RuleResult, *cmm.ErrorHandler) {
		result, err := providers.Below(series, asOf, 20)
		return result, err
	}
	hysteresis := func(series []providers.Observation, asOf time.Time) (providers.RuleResult, *cmm.ErrorHandler) {
		result, err := providers.AboveWithHysteresis(series, asOf, 70, 40)
		return result, err
	}

	tests := []struct {
		name         string
		series       []providers.Observation
		asOf         time.Time
		points       int
		rule         providers.SeriesRule
		wantAccepted bool
		wantValue    float64
		wantAt       time.Time
	}{
		{"un punto es la regla tal cual", daily(30, 10), farFuture, 1, below20, true, 10, dayAt(1)},
		{"los ultimos 3 cumplen", daily(30, 15, 12, 10), farFuture, 3, below20, true, 10, dayAt(3)},
		{"el cuarto hacia atras no cumple", daily(30, 15, 12, 10), farFuture, 4, below20, false, 10, dayAt(3)},
		{"el ultimo no cumple", daily(10, 12, 25), farFuture, 2, below20, false, 25, dayAt(2)},
		{"un punto intermedio no cumple", daily(10, 25, 12), farFuture, 3, below20, false, 12, dayAt(2)},
		// A la fecha del dia 2 los dias 1 y 2 cumplen; el 30 del dia 3 aun no existe
		{"as of ignora observaciones futuras", daily(30, 15, 12, 30), dayAt(2), 2, below20, true, 12, dayAt(2)},
		// La histeresis se recalcula en cada punto: encendida en 75 y se mantiene en 50 y 45
		{"histeresis encendida 3 puntos", daily(75, 50, 45), farFuture, 3, hysteresis, true, 45, dayAt(2)},
		{"histeresis encendida solo los ultimos 2", daily(30, 75, 50), farFuture, 3, hysteresis, false, 50, dayAt(2)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.Persist(tt.series, tt.asOf, tt.points, tt.rule)

			testutil.RequireEmptyError(t, err)
			requireRuleResult(t, got, tt.wantAccepted, tt.wantValue, tt.wantAt)
		})
	}

	// La ventana de 3 alcanza en el ultimo punto pero no en el anterior: la serie debe traer points - 1 extra
	zScoreWindow3 := func(series []providers.Observation, asOf time.Time) (providers.RuleResult, *cmm.ErrorHandler) {
		result, err := providers.ZScoreAbove(series, asOf, 3, 0)
		return result, err
	}

	errorTests := []struct {
		name     string
		series   []providers.Observation
		points   int
		rule     providers.SeriesRule
		wantCode string
	}{
		{"cero puntos", daily(10), 0, below20, "PCSRLE010"},
		{"sin regla", daily(10), 1, nil, "PCSRLE011"},
		{"menos puntos que points", daily(10, 12), 3, below20, "PCSRLE002"},
		{"la regla falla en un punto anterior", daily(1, 2, 3, 5), 2, zScoreWindow3, "PCSRLE002"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.Persist(tt.series, farFuture, tt.points, tt.rule)

			testutil.RequireTrackingCode(t, err, tt.wantCode)
			if got != (providers.RuleResult{}) {
				t.Errorf("result = %+v, un error debe retornar RuleResult{}", got)
			}
		})
	}
}

// TestSeriesRules_DoNotModifyInput las reglas ordenan una copia: la serie del extractor no cambia de orden
func TestSeriesRules_DoNotModifyInput(t *testing.T) {
	t.Parallel()

	series := []providers.Observation{{At: dayAt(2), Value: 3}, {At: dayAt(0), Value: 1}, {At: dayAt(1), Value: 2}}
	original := slices.Clone(series)

	_, err := providers.SlopeAbove(series, farFuture, 0, 0)

	testutil.RequireEmptyError(t, err)
	if !slices.Equal(series, original) {
		t.Errorf("series = %+v, want %+v sin modificar", series, original)
	}
}
