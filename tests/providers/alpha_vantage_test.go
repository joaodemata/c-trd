package providers_test

import (
	"testing"

	"c_trd/providers"
	"c_trd/tests/testutil"
)

func TestRealtimePutCallRatioCallback_Fixture(t *testing.T) {
	t.Parallel()

	// Respuesta real de Alpha Vantage con ratio 1.27 (> 1 es bullish)
	accepted, err := providers.RealtimePutCallRatioCallback(testutil.ReadFixture(t, "alpha_vantage_put_call_ratio.json"))

	testutil.RequireEmptyError(t, err)
	if !accepted {
		t.Error("un ratio de 1.27 deberia ser bullish")
	}
}

func TestRealtimePutCallRatioCallback_Thresholds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ratio string
		want  bool
	}{
		{"1.5", true},
		{"1.01", true},
		{"1.0", false}, // el limite superior es exclusivo
		{"0.8", false},
		{"0.61", false},
		{"0.6", true}, // el limite inferior es inclusivo
		{"0.3", true},
	}

	for _, tt := range tests {
		t.Run(tt.ratio, func(t *testing.T) {
			t.Parallel()

			body := []byte(`{"symbol":"SPY","put_call_ratio_full_chain":"` + tt.ratio + `"}`)
			accepted, err := providers.RealtimePutCallRatioCallback(body)

			testutil.RequireEmptyError(t, err)
			if accepted != tt.want {
				t.Errorf("ratio %s: accepted = %v, se esperaba %v", tt.ratio, accepted, tt.want)
			}
		})
	}
}

func TestRealtimePutCallRatioCallback_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		wantTracking string
	}{
		{"JSON invalido", `{`, "PCALVE001"},
		{"limite de peticiones (Note)", `{"Note":"Thank you for using Alpha Vantage!"}`, "PCALVE003"},
		{"limite de peticiones (Information)", `{"Information":"rate limit"}`, "PCALVE003"},
		{"mensaje de error", `{"Error Message":"Invalid API call"}`, "PCALVE003"},
		{"ratio vacio", `{"symbol":"SPY"}`, "PCALVE002"},
		{"ratio no numerico", `{"put_call_ratio_full_chain":"abc"}`, "PCALVE002"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accepted, err := providers.RealtimePutCallRatioCallback([]byte(tt.body))

			testutil.RequireTrackingCode(t, err, tt.wantTracking)
			if accepted {
				t.Error("un error nunca debe aceptar la condicion")
			}
		})
	}
}
