package providers_test

import (
	"slices"
	"testing"

	cmm "c_trd/common"
	"c_trd/providers"
)

// TestFunctionRegistry_Definitions todo callback debe tener funcion, descripcion y un formato de respuesta conocido,
// si no nunca se ofreceria para ninguna API
func TestFunctionRegistry_Definitions(t *testing.T) {
	t.Parallel()

	known := []string{cmm.ResponseFormatAlphaVantagePutCallRatio, cmm.ResponseFormatGoogleTrendsSeries, cmm.ResponseFormatGoogleTrendsFile}
	for name, definition := range providers.FunctionRegistry {
		if definition.Fn == nil || definition.Description == "" {
			t.Errorf("%s: funcion o descripcion vacia", name)
		}
		if !slices.Contains(known, definition.ResponseFormat) {
			t.Errorf("%s: response_format %q desconocido", name, definition.ResponseFormat)
		}
	}
}

func TestListCallbacksByResponseFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format string
		want   []string
	}{
		{"alpha vantage", cmm.ResponseFormatAlphaVantagePutCallRatio, []string{"ProcessAlphaVantageData"}},
		{"google trends", cmm.ResponseFormatGoogleTrendsSeries, []string{"GoogleTrendsHigh", "GoogleTrendsLow"}},
		{"csv de google trends ordenado por nombre", cmm.ResponseFormatGoogleTrendsFile, []string{"GoogleTrendsFileHigh", "GoogleTrendsFileLow", "GoogleTrendsFileNewMax", "GoogleTrendsFileNewMin"}},
		{"formato desconocido", "UNKNOWN", []string{}},
		{"formato vacio", "", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			callbacks := providers.ListCallbacksByResponseFormat(tt.format)
			if callbacks == nil {
				t.Fatal("se esperaba [] y no nil")
			}

			names := make([]string, len(callbacks))
			for i, callback := range callbacks {
				names[i] = callback.Name
				if callback.ResponseFormat != tt.format || callback.Description == "" {
					t.Errorf("callback = %+v, se esperaba el formato %s con descripcion", callback, tt.format)
				}
			}
			if !slices.Equal(names, tt.want) {
				t.Errorf("callbacks = %v, want %v", names, tt.want)
			}
		})
	}
}
