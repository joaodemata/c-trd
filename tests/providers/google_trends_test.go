package providers_test

import (
	"strings"
	"testing"

	"c_trd/providers"
	"c_trd/tests/testutil"
)

func TestGoogleTrendsCallbacks_Fixture(t *testing.T) {
	t.Parallel()

	// Serie completa 20, 60 ("60" llega como string), 100, 97 y un ultimo punto partial_data = 30.
	// Rango 20-100: el umbral alto es 92 y el bajo 28. Si el punto parcial no se ignorara,
	// el ultimo valor seria 30 y la zona alta no se cumpliria
	body := testutil.ReadFixture(t, "serpapi_google_trends.json")

	high, err := providers.GoogleTrendsHighCallback(body, nil)
	testutil.RequireEmptyError(t, err)
	if !high {
		t.Error("GoogleTrendsHighCallback deberia aceptar 97 en el rango 20-100")
	}

	low, err := providers.GoogleTrendsLowCallback(body, nil)
	testutil.RequireEmptyError(t, err)
	if low {
		t.Error("GoogleTrendsLowCallback no deberia aceptar 97 en el rango 20-100")
	}
}

func TestGoogleTrendsCallbacks_Zones(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		callback providers.CallbackFunc
		values   []string
		want     bool
	}{
		{"zona alta", providers.GoogleTrendsHighCallback, []string{`0`, `100`, `95`}, true},
		{"fuera de la zona alta", providers.GoogleTrendsHighCallback, []string{`0`, `100`, `89`}, false},
		{"zona baja con valores string", providers.GoogleTrendsLowCallback, []string{`"0"`, `"100"`, `"5"`}, true},
		{"fuera de la zona baja", providers.GoogleTrendsLowCallback, []string{`0`, `100`, `11`}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accepted, err := tt.callback([]byte(timeline(tt.values...)), nil)

			testutil.RequireEmptyError(t, err)
			if accepted != tt.want {
				t.Errorf("valores %v: accepted = %v, se esperaba %v", tt.values, accepted, tt.want)
			}
		})
	}
}

func TestGoogleTrendsCallbacks_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		wantTracking string
	}{
		{"JSON invalido", `{`, "PCGTRE001"},
		{"error del proveedor", `{"error":"Your account has run out of searches."}`, "PCGTRE002"},
		{"valor que no es numero ni string", timeline(`true`, `1`), "PCGTRE003"},
		{"string no numerico", timeline(`"abc"`, `1`), "PCGTRE005"},
		{"menos de 2 puntos completos", timeline(`1`), "PCGTRE004"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accepted, err := providers.GoogleTrendsHighCallback([]byte(tt.body), nil)

			testutil.RequireTrackingCode(t, err, tt.wantTracking)
			if accepted {
				t.Error("un error nunca debe aceptar la condicion")
			}
		})
	}
}

// timeline arma una respuesta de SerpApi con un punto completo por cada extracted_value (JSON crudo)
func timeline(extractedValues ...string) string {
	points := make([]string, len(extractedValues))
	for i, value := range extractedValues {
		points[i] = `{"date":"d","values":[{"query":"q","extracted_value":` + value + `}]}`
	}

	return `{"interest_over_time":{"timeline_data":[` + strings.Join(points, ",") + `]}}`
}
