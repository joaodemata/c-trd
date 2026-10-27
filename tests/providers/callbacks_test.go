package providers_test

import (
	"slices"
	"testing"

	cmm "c_trd/common"
	"c_trd/providers"
)

// TestFunctionRegistry_Definitions todo callback debe tener funcion, descripcion, nombre, un tag unico, el ID igual a su
// llave en el registro y un formato de respuesta conocido, si no nunca se ofreceria para ninguna API
func TestFunctionRegistry_Definitions(t *testing.T) {
	t.Parallel()

	known := []string{cmm.ResponseFormatAlphaVantagePutCallRatio, cmm.ResponseFormatGoogleTrendsSeries, cmm.ResponseFormatGoogleTrendsFile, cmm.ResponseFormatFredSeriesObservations, cmm.ResponseFormatPolymarketFedDecisions, cmm.ResponseFormatCoinMarketCapFearGreed, cmm.ResponseFormatCoinMarketCapGlobal, cmm.ResponseFormatCoinMarketCapUsdtQuote}
	tags := make(map[string]string)
	for name, definition := range providers.FunctionRegistry {
		if definition.Fn == nil || definition.Description == "" || definition.Name == "" || definition.Tag == "" {
			t.Errorf("%s: funcion, descripcion, nombre o tag vacio", name)
		}
		if definition.ID != name {
			t.Errorf("%s: ID = %q, debe ser igual a su llave en el registro", name, definition.ID)
		}
		if other, exists := tags[definition.Tag]; exists {
			t.Errorf("%s: tag %q repetido con %s", name, definition.Tag, other)
		}
		tags[definition.Tag] = name
		if !slices.Contains(known, definition.ResponseFormat) {
			t.Errorf("%s: response_format %q desconocido", name, definition.ResponseFormat)
		}

		// Los parametros se validan al crear la condicion, deben tener nombre unico y un tipo de dato soportado
		dataTypes := []string{cmm.ApiParameterTypeString, cmm.ApiParameterTypeInteger, cmm.ApiParameterTypeFloat, cmm.ApiParameterTypeBoolean, cmm.ApiParameterTypeDate}
		parameterNames := make(map[string]bool)
		for _, parameter := range definition.Parameters {
			if parameter.Name == "" || parameter.Label == "" || parameter.Description == "" {
				t.Errorf("%s: parametro %+v con nombre, label o descripcion vacio", name, parameter)
			}
			if parameterNames[parameter.Name] {
				t.Errorf("%s: parametro %q repetido", name, parameter.Name)
			}
			parameterNames[parameter.Name] = true
			if !slices.Contains(dataTypes, parameter.DataType) {
				t.Errorf("%s: parametro %q con data_type %q no soportado", name, parameter.Name, parameter.DataType)
			}
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
		{"fred", cmm.ResponseFormatFredSeriesObservations, []string{"FredSeriesFalling", "FredSeriesRising"}},
		{"polymarket fed", cmm.ResponseFormatPolymarketFedDecisions, []string{"FedNextMeetingCutLikely", "FedNextMeetingHikeLikely", "FedNextMeetingHoldLikely"}},
		{"coinmarketcap fear and greed", cmm.ResponseFormatCoinMarketCapFearGreed, []string{"FearAndGreedAbove", "FearAndGreedBelow", "FearAndGreedExtremeFear", "FearAndGreedExtremeGreed", "FearAndGreedFear", "FearAndGreedGreed", "FearAndGreedNeutral"}},
		{"coinmarketcap global metrics", cmm.ResponseFormatCoinMarketCapGlobal, []string{"BtcDominanceAbove", "BtcDominanceAbove60", "BtcDominanceBelow", "Others2DominanceAbove", "Others2DominanceBelow", "Others3DominanceAbove", "Others3DominanceBelow", "StablecoinDominanceAbove12"}},
		{"coinmarketcap usdt", cmm.ResponseFormatCoinMarketCapUsdtQuote, []string{"UsdtDominanceAbove", "UsdtDominanceBelow"}},
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
				names[i] = callback.ID
				if callback.ResponseFormat != tt.format || callback.Description == "" || callback.Name == "" || callback.Tag == "" {
					t.Errorf("callback = %+v, se esperaba el formato %s con descripcion, nombre y tag", callback, tt.format)
				}
				if callback.Parameters == nil {
					t.Errorf("callback %s: parameters debe ser [] y no nil", callback.ID)
				}
			}
			if !slices.Equal(names, tt.want) {
				t.Errorf("callbacks = %v, want %v", names, tt.want)
			}
		})
	}
}
