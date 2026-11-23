package providers_test

import (
	"testing"
	"time"

	"c_trd/providers"
	"c_trd/tests/testutil"
)

// Cada callback reconstruible existe en FunctionRegistry y tiene extractor y decision
func TestPointInTimeRegistry(t *testing.T) {
	t.Parallel()

	for id, definition := range providers.PointInTimeRegistry {
		if _, registered := providers.FunctionRegistry[id]; !registered {
			t.Errorf("%s no existe en FunctionRegistry", id)
		}
		if definition.Extract == nil || definition.Decide == nil {
			t.Errorf("%s no tiene extractor o decision", id)
		}
	}
}

// La decision solo ve las observaciones hasta la fecha: UNRATE 2026-08-01 = 4.1 y 2026-09-01 = 4.2
func TestPointInTimeFredBelow(t *testing.T) {
	t.Parallel()

	definition := providers.PointInTimeRegistry["FredSeriesBelow"]
	series, err := definition.Extract(testutil.ReadFixture(t, "fred_unrate_observations.json"))
	testutil.RequireEmptyError(t, err)

	parameters := map[string]any{"threshold": 4.15}
	tests := []struct {
		name     string
		asOf     time.Time
		accepted bool
		value    float64
	}{
		{"agosto: el ultimo dato es 4.1", time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC), true, 4.1},
		{"septiembre: el ultimo dato es 4.2", time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), false, 4.2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := definition.Decide(series, parameters, tt.asOf)
			testutil.RequireEmptyError(t, err)
			if result.Accepted != tt.accepted || result.Value != tt.value {
				t.Errorf("resultado = %+v, se esperaba accepted=%v value=%v", result, tt.accepted, tt.value)
			}
			if result.AsOf == nil || result.AsOf.After(tt.asOf) {
				t.Errorf("as_of = %v, debe ser una fecha hasta %s", result.AsOf, tt.asOf)
			}
		})
	}

	// Antes de la primera observacion no hay datos
	_, err = definition.Decide(series, parameters, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err.IsEmtpy() {
		t.Error("se esperaba error sin observaciones visibles")
	}
}

// El CSV de Google Trends para el backtest no valida que el archivo este al dia
func TestExtractGoogleTrendsFileHistory(t *testing.T) {
	t.Parallel()

	body := []byte(`{"status":"SUCCESS","data":{"uploadedAt":"2020-01-01T00:00:00Z","periodSeconds":604800,"points":[{"date":"2019-12-15T00:00:00Z","value":40},{"date":"2019-12-22T00:00:00Z","value":55}]}}`)
	series, err := providers.ExtractGoogleTrendsFileHistory(body)
	testutil.RequireEmptyError(t, err)
	if len(series) != 2 || series[1].Value != 55 {
		t.Errorf("serie = %+v", series)
	}

	_, err = providers.ExtractGoogleTrendsFileObservations(body)
	testutil.RequireTrackingCode(t, err, "PCGTFE003")

	_, err = providers.ExtractGoogleTrendsFileHistory([]byte("no json"))
	testutil.RequireTrackingCode(t, err, "PCGTFE004")
}
