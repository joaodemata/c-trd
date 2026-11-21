package providers_test

import (
	"strconv"
	"testing"
	"time"

	"c_trd/providers"
	"c_trd/tests/testutil"
)

func TestRealtimePutCallRatioCallback_Fixture(t *testing.T) {
	t.Parallel()

	// Respuesta real de Alpha Vantage con ratio 1.27 (> 1 es bullish) y fecha 2026-10-14
	result, err := providers.RealtimePutCallRatioCallback(testutil.ReadFixture(t, "alpha_vantage_put_call_ratio.json"), nil)

	testutil.RequireEmptyError(t, err)
	if !result.Accepted {
		t.Error("un ratio de 1.27 deberia ser bullish")
	}
	if result.Value != 1.27 {
		t.Errorf("value = %v, want 1.27", result.Value)
	}
	requireAsOf(t, result.AsOf, time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC))
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
			result, err := providers.RealtimePutCallRatioCallback(body, nil)

			testutil.RequireEmptyError(t, err)
			if result.Accepted != tt.want {
				t.Errorf("ratio %s: accepted = %v, se esperaba %v", tt.ratio, result.Accepted, tt.want)
			}
			wantValue, _ := strconv.ParseFloat(tt.ratio, 64)
			if result.Value != wantValue {
				t.Errorf("ratio %s: value = %v, se esperaba %v", tt.ratio, result.Value, wantValue)
			}
			// Sin campo date la fecha del dato queda nil
			requireAsOf(t, result.AsOf, time.Time{})
		})
	}
}

func TestRealtimePutCallRatioCallback_InvalidDate(t *testing.T) {
	t.Parallel()

	// Una fecha que no se puede leer no hace fallar la evaluacion, solo deja AsOf en nil
	body := []byte(`{"symbol":"SPY","date":"14/10/2026","put_call_ratio_full_chain":"1.5"}`)
	result, err := providers.RealtimePutCallRatioCallback(body, nil)

	testutil.RequireEmptyError(t, err)
	if !result.Accepted || result.Value != 1.5 {
		t.Errorf("accepted, value = %v, %v; want true, 1.5", result.Accepted, result.Value)
	}
	requireAsOf(t, result.AsOf, time.Time{})
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

			result, err := providers.RealtimePutCallRatioCallback([]byte(tt.body), nil)

			testutil.RequireTrackingCode(t, err, tt.wantTracking)
			requireEmptyResult(t, result)
		})
	}
}
