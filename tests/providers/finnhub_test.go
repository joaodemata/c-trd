package providers_test

import (
	"testing"

	"c_trd/providers"
	"c_trd/tests/testutil"
)

func TestFinnhubQuotePrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         string
		want         float64
		wantTracking string
	}{
		{"precio actual", `{"c":261.74,"d":0.89,"dp":0.3412,"h":263.31,"l":260.68,"o":261.07,"pc":260.85,"t":1761336000}`, 261.74, ""},
		{"simbolo desconocido responde ceros", `{"c":0,"d":null,"dp":null,"h":0,"l":0,"o":0,"pc":0,"t":0}`, 0, "PCFINE002"},
		{"sin precio", `{"t":1761336000}`, 0, "PCFINE002"},
		{"precio negativo", `{"c":-1,"t":1761336000}`, 0, "PCFINE002"},
		{"sin fecha", `{"c":261.74}`, 0, "PCFINE002"},
		{"respuesta de error", `{"error":"You don't have access to this resource."}`, 0, "PCFINE002"},
		{"no es JSON", `<html></html>`, 0, "PCFINE001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.FinnhubQuotePrice([]byte(tt.body))

			if tt.wantTracking != "" {
				testutil.RequireTrackingCode(t, err, tt.wantTracking)
				return
			}
			testutil.RequireEmptyError(t, err)
			if got != tt.want {
				t.Errorf("price = %v, want %v", got, tt.want)
			}
		})
	}
}
