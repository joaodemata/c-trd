package providers_test

import (
	"fmt"
	"testing"

	"c_trd/providers"
	"c_trd/tests/testutil"
)

// fearAndGreedBody respuesta exitosa de /v3/fear-and-greed/latest con el valor indicado
func fearAndGreedBody(value string) []byte {
	return []byte(`{"data":{"value":` + value + `,"update_time":"2026-10-18T12:53:10.022Z","value_classification":"Neutral"},"status":{"error_code":"0","error_message":""}}`)
}

// globalMetricsBody respuesta exitosa de /v1/global-metrics/quotes/latest con la dominancia de BTC y los market caps
func globalMetricsBody(btcDominance, totalMarketCap, stablecoinMarketCap float64) []byte {
	return fmt.Appendf(nil, `{"data":{"btc_dominance":%v,"quote":{"USD":{"total_market_cap":%v,"stablecoin_market_cap":%v}}},"status":{"error_code":0,"error_message":null}}`, btcDominance, totalMarketCap, stablecoinMarketCap)
}

// fearAndGreedZones ejecuta los cinco callbacks sobre la misma respuesta
func fearAndGreedZones(t *testing.T, body []byte) [5]bool {
	t.Helper()

	callbacks := []providers.CallbackFunc{
		providers.FearAndGreedExtremeFearCallback,
		providers.FearAndGreedFearCallback,
		providers.FearAndGreedNeutralCallback,
		providers.FearAndGreedGreedCallback,
		providers.FearAndGreedExtremeGreedCallback,
	}

	var zones [5]bool
	for i, callback := range callbacks {
		accepted, err := callback(body, nil)
		testutil.RequireEmptyError(t, err)
		zones[i] = accepted
	}
	return zones
}

func TestFearAndGreedCallbacks_Fixture(t *testing.T) {
	t.Parallel()

	// Respuesta real con valor 49 (Neutral)
	got := fearAndGreedZones(t, testutil.ReadFixture(t, "coinmarketcap_fear_and_greed_latest.json"))
	want := [5]bool{false, false, true, false, false}
	if got != want {
		t.Errorf("zones = %v, want %v", got, want)
	}
}

func TestFearAndGreedCallbacks_Zones(t *testing.T) {
	t.Parallel()

	// Orden de las zonas: extreme fear, fear, neutral, greed, extreme greed
	tests := []struct {
		name  string
		value string
		want  [5]bool
	}{
		{"cero es extreme fear", "0", [5]bool{true, false, false, false, false}},
		{"19 es extreme fear", "19", [5]bool{true, false, false, false, false}},
		{"20 es fear", "20", [5]bool{false, true, false, false, false}},
		{"39 es fear", "39", [5]bool{false, true, false, false, false}},
		{"40 es neutral", "40", [5]bool{false, false, true, false, false}},
		{"59 es neutral", "59", [5]bool{false, false, true, false, false}},
		{"60 es greed", "60", [5]bool{false, false, false, true, false}},
		{"79 es greed", "79", [5]bool{false, false, false, true, false}},
		{"80 es extreme greed", "80", [5]bool{false, false, false, false, true}},
		{"100 es extreme greed", "100", [5]bool{false, false, false, false, true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := fearAndGreedZones(t, fearAndGreedBody(tt.value))
			if got != tt.want {
				t.Errorf("zones = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFearAndGreedCallbacks_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body []byte
		code string
	}{
		{"json invalido", []byte(`{`), "PCCMCE001"},
		{"limite de peticiones con error_code numerico", testutil.ReadFixture(t, "coinmarketcap_rate_limit_error.json"), "PCCMCE002"},
		{"limite de peticiones con error_code string", []byte(`{"status":{"error_code":"1022","error_message":"You've reached the limit for anonymous access."}}`), "PCCMCE002"},
		{"sin data", []byte(`{"status":{"error_code":"0","error_message":""}}`), "PCCMCE003"},
		{"valor null", fearAndGreedBody("null"), "PCCMCE003"},
		{"valor negativo", fearAndGreedBody("-1"), "PCCMCE003"},
		{"valor mayor a 100", fearAndGreedBody("101"), "PCCMCE003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accepted, err := providers.FearAndGreedNeutralCallback(tt.body, nil)

			testutil.RequireTrackingCode(t, err, tt.code)
			if accepted {
				t.Error("un error no debe aceptar la condicion")
			}
		})
	}
}

func TestDominanceCallbacks_Fixture(t *testing.T) {
	t.Parallel()

	// Respuesta real: dominancia de BTC 60.14% y stablecoins 282.4B de 2736.1B (10.32%)
	body := testutil.ReadFixture(t, "coinmarketcap_global_metrics_latest.json")

	btcAbove, err := providers.BtcDominanceAbove60Callback(body, nil)
	testutil.RequireEmptyError(t, err)
	stablecoinAbove, err := providers.StablecoinDominanceAbove12Callback(body, nil)
	testutil.RequireEmptyError(t, err)

	if !btcAbove || stablecoinAbove {
		t.Errorf("btc, stablecoin = %v, %v, want true, false", btcAbove, stablecoinAbove)
	}
}

func TestBtcDominanceAbove60Callback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		btcDominance float64
		want         bool
	}{
		{"mayor al umbral", 60.01, true},
		{"en el umbral no es mayor", 60, false},
		{"menor al umbral", 59.99, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.BtcDominanceAbove60Callback(globalMetricsBody(tt.btcDominance, 1000, 100), nil)

			testutil.RequireEmptyError(t, err)
			if got != tt.want {
				t.Errorf("btc_dominance %v: got %v, want %v", tt.btcDominance, got, tt.want)
			}
		})
	}
}

func TestStablecoinDominanceAbove12Callback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		totalMarketCap      float64
		stablecoinMarketCap float64
		want                bool
	}{
		{"mayor al umbral", 1000, 121, true},
		{"en el umbral no es mayor", 1000, 120, false},
		{"menor al umbral", 1000, 119, false},
		{"sin stablecoins", 1000, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.StablecoinDominanceAbove12Callback(globalMetricsBody(50, tt.totalMarketCap, tt.stablecoinMarketCap), nil)

			testutil.RequireEmptyError(t, err)
			if got != tt.want {
				t.Errorf("stablecoin %v de %v: got %v, want %v", tt.stablecoinMarketCap, tt.totalMarketCap, got, tt.want)
			}
		})
	}
}

func TestDominanceCallbacks_Errors(t *testing.T) {
	t.Parallel()

	rateLimit := testutil.ReadFixture(t, "coinmarketcap_rate_limit_error.json")
	noData := []byte(`{"status":{"error_code":0,"error_message":null}}`)

	tests := []struct {
		name     string
		callback providers.CallbackFunc
		body     []byte
		code     string
	}{
		{"btc json invalido", providers.BtcDominanceAbove60Callback, []byte(`[`), "PCCMCE004"},
		{"btc limite de peticiones", providers.BtcDominanceAbove60Callback, rateLimit, "PCCMCE002"},
		{"btc sin data", providers.BtcDominanceAbove60Callback, noData, "PCCMCE005"},
		{"btc sin dominancia", providers.BtcDominanceAbove60Callback, []byte(`{"data":{"quote":{}},"status":{"error_code":0}}`), "PCCMCE006"},
		{"btc dominancia mayor a 100", providers.BtcDominanceAbove60Callback, globalMetricsBody(100.5, 1000, 100), "PCCMCE006"},
		{"stablecoin json invalido", providers.StablecoinDominanceAbove12Callback, []byte(`[`), "PCCMCE004"},
		{"stablecoin limite de peticiones", providers.StablecoinDominanceAbove12Callback, rateLimit, "PCCMCE002"},
		{"stablecoin sin data", providers.StablecoinDominanceAbove12Callback, noData, "PCCMCE005"},
		{"stablecoin sin cotizacion USD", providers.StablecoinDominanceAbove12Callback, []byte(`{"data":{"quote":{"EUR":{"total_market_cap":1000,"stablecoin_market_cap":100}}},"status":{"error_code":0}}`), "PCCMCE007"},
		{"stablecoin sin market cap de stablecoins", providers.StablecoinDominanceAbove12Callback, []byte(`{"data":{"quote":{"USD":{"total_market_cap":1000}}},"status":{"error_code":0}}`), "PCCMCE008"},
		{"stablecoin market cap total cero", providers.StablecoinDominanceAbove12Callback, globalMetricsBody(50, 0, 100), "PCCMCE008"},
		{"stablecoin market cap negativo", providers.StablecoinDominanceAbove12Callback, globalMetricsBody(50, 1000, -1), "PCCMCE008"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accepted, err := tt.callback(tt.body, nil)

			testutil.RequireTrackingCode(t, err, tt.code)
			if accepted {
				t.Error("un error no debe aceptar la condicion")
			}
		})
	}
}

// coinMarketCapQuotesBody respuesta de /v2/cryptocurrency/quotes/latest?symbol=BTC con dos monedas que comparten
// simbolo: la de mejor rank y una inactiva sin rank
const coinMarketCapQuotesBody = `{"data":{"BTC":[` +
	`{"id":31469,"symbol":"BTC","is_active":0,"cmc_rank":null,"quote":{"USD":{"price":null}}},` +
	`{"id":1,"symbol":"BTC","is_active":1,"cmc_rank":1,"quote":{"USD":{"price":67012.34}}}` +
	`]},"status":{"error_code":0,"error_message":null}}`

func TestCoinMarketCapQuotePrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		symbol       string
		body         string
		want         float64
		wantTracking string
	}{
		{"ignora la moneda sin rank", "BTC", coinMarketCapQuotesBody, 67012.34, ""},
		{"simbolo en minusculas", "btc", coinMarketCapQuotesBody, 67012.34, ""},
		{"usa el mejor rank", "ETH", `{"data":{"ETH":[` +
			`{"cmc_rank":3500,"quote":{"USD":{"price":0.01}}},` +
			`{"cmc_rank":2,"quote":{"USD":{"price":2450.5}}}` +
			`]},"status":{"error_code":"0"}}`, 2450.5, ""},
		{"simbolo no encontrado", "XYZ", `{"data":{},"status":{"error_code":0}}`, 0, "PCCMCE010"},
		{"solo monedas sin rank", "BTC", `{"data":{"BTC":[{"cmc_rank":null,"quote":{}}]},"status":{"error_code":0}}`, 0, "PCCMCE010"},
		{"sin cotizacion USD", "BTC", `{"data":{"BTC":[{"cmc_rank":1,"quote":{"EUR":{"price":60000}}}]},"status":{"error_code":0}}`, 0, "PCCMCE011"},
		{"precio cero", "BTC", `{"data":{"BTC":[{"cmc_rank":1,"quote":{"USD":{"price":0}}}]},"status":{"error_code":0}}`, 0, "PCCMCE011"},
		{"error de la API", "BTC", `{"status":{"error_code":1002,"error_message":"API key missing."}}`, 0, "PCCMCE002"},
		{"no es JSON", "BTC", `<html></html>`, 0, "PCCMCE009"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.CoinMarketCapQuotePrice([]byte(tt.body), tt.symbol)

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

// thresholdParameters parameters_callback de una condicion con el umbral indicado
func thresholdParameters(threshold any) map[string]any {
	return map[string]any{"threshold": threshold}
}

// dominanceGlobalMetricsBody respuesta exitosa de /v1/global-metrics/quotes/latest con las dominancias de BTC y ETH
func dominanceGlobalMetricsBody(btcDominance, ethDominance float64) []byte {
	return fmt.Appendf(nil, `{"data":{"btc_dominance":%v,"eth_dominance":%v,"quote":{"USD":{"total_market_cap":1000,"stablecoin_market_cap":100}}},"status":{"error_code":0,"error_message":null}}`, btcDominance, ethDominance)
}

// usdtQuotesBody respuesta de /v2/cryptocurrency/quotes/latest?symbol=USDT con el market_cap_dominance indicado y una
// moneda inactiva sin rank que comparte el simbolo
func usdtQuotesBody(dominance string) []byte {
	return []byte(`{"data":{"USDT":[` +
		`{"id":99999,"symbol":"USDT","cmc_rank":null,"quote":{"USD":{"price":null,"market_cap_dominance":null}}},` +
		`{"id":825,"symbol":"USDT","cmc_rank":3,"quote":{"USD":{"price":1.0002,"market_cap_dominance":` + dominance + `}}}` +
		`]},"status":{"error_code":0,"error_message":null}}`)
}

func TestFearAndGreedThresholdCallbacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     string
		threshold any
		wantAbove bool
		wantBelow bool
	}{
		{"mayor al umbral", "26", 25.0, true, false},
		{"en el umbral no es mayor ni menor", "25", 25.0, false, false},
		{"menor al umbral", "24", 25.0, false, true},
		{"umbral decimal", "75", 74.5, true, false},
		{"umbral int32 guardado a mano en mongo", "10", int32(20), false, true},
		{"umbral int64", "90", int64(80), true, false},
		{"umbral cero", "0", 0.0, false, false},
		{"umbral 100", "100", 100.0, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := fearAndGreedBody(tt.value)
			above, err := providers.FearAndGreedAboveCallback(body, thresholdParameters(tt.threshold))
			testutil.RequireEmptyError(t, err)
			below, err := providers.FearAndGreedBelowCallback(body, thresholdParameters(tt.threshold))
			testutil.RequireEmptyError(t, err)

			if above != tt.wantAbove || below != tt.wantBelow {
				t.Errorf("valor %s umbral %v: above, below = %v, %v, want %v, %v", tt.value, tt.threshold, above, below, tt.wantAbove, tt.wantBelow)
			}
		})
	}
}

func TestFearAndGreedThresholdCallbacks_Fixture(t *testing.T) {
	t.Parallel()

	// Respuesta real con valor 49
	body := testutil.ReadFixture(t, "coinmarketcap_fear_and_greed_latest.json")

	above, err := providers.FearAndGreedAboveCallback(body, thresholdParameters(45.0))
	testutil.RequireEmptyError(t, err)
	below, err := providers.FearAndGreedBelowCallback(body, thresholdParameters(45.0))
	testutil.RequireEmptyError(t, err)

	if !above || below {
		t.Errorf("above, below = %v, %v, want true, false", above, below)
	}
}

func TestDominanceThresholdCallbacks_Fixture(t *testing.T) {
	t.Parallel()

	// Respuesta real: BTC.D 60.14%, ETH.D 10.78%, OTHERS2 39.86% y OTHERS3 29.08%
	body := testutil.ReadFixture(t, "coinmarketcap_global_metrics_latest.json")

	tests := []struct {
		name      string
		callback  providers.CallbackFunc
		threshold float64
		want      bool
	}{
		{"btc mayor a 60", providers.BtcDominanceAboveCallback, 60, true},
		{"btc menor a 60", providers.BtcDominanceBelowCallback, 60, false},
		{"others2 mayor a 39", providers.Others2DominanceAboveCallback, 39, true},
		{"others2 menor a 40", providers.Others2DominanceBelowCallback, 40, true},
		{"others3 mayor a 29", providers.Others3DominanceAboveCallback, 29, true},
		{"others3 menor a 29", providers.Others3DominanceBelowCallback, 29, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.callback(body, thresholdParameters(tt.threshold))

			testutil.RequireEmptyError(t, err)
			if got != tt.want {
				t.Errorf("umbral %v: got %v, want %v", tt.threshold, got, tt.want)
			}
		})
	}
}

func TestDominanceThresholdCallbacks(t *testing.T) {
	t.Parallel()

	// Con BTC.D 55 y ETH.D 15: OTHERS2 = 45 y OTHERS3 = 30
	body := dominanceGlobalMetricsBody(55, 15)

	tests := []struct {
		name      string
		above     providers.CallbackFunc
		below     providers.CallbackFunc
		threshold float64
		wantAbove bool
		wantBelow bool
	}{
		{"btc mayor al umbral", providers.BtcDominanceAboveCallback, providers.BtcDominanceBelowCallback, 54.99, true, false},
		{"btc en el umbral", providers.BtcDominanceAboveCallback, providers.BtcDominanceBelowCallback, 55, false, false},
		{"btc menor al umbral", providers.BtcDominanceAboveCallback, providers.BtcDominanceBelowCallback, 55.01, false, true},
		{"others2 mayor al umbral", providers.Others2DominanceAboveCallback, providers.Others2DominanceBelowCallback, 44.99, true, false},
		{"others2 en el umbral", providers.Others2DominanceAboveCallback, providers.Others2DominanceBelowCallback, 45, false, false},
		{"others2 menor al umbral", providers.Others2DominanceAboveCallback, providers.Others2DominanceBelowCallback, 45.01, false, true},
		{"others3 mayor al umbral", providers.Others3DominanceAboveCallback, providers.Others3DominanceBelowCallback, 29.99, true, false},
		{"others3 en el umbral", providers.Others3DominanceAboveCallback, providers.Others3DominanceBelowCallback, 30, false, false},
		{"others3 menor al umbral", providers.Others3DominanceAboveCallback, providers.Others3DominanceBelowCallback, 30.01, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			above, err := tt.above(body, thresholdParameters(tt.threshold))
			testutil.RequireEmptyError(t, err)
			below, err := tt.below(body, thresholdParameters(tt.threshold))
			testutil.RequireEmptyError(t, err)

			if above != tt.wantAbove || below != tt.wantBelow {
				t.Errorf("umbral %v: above, below = %v, %v, want %v, %v", tt.threshold, above, below, tt.wantAbove, tt.wantBelow)
			}
		})
	}
}

func TestUsdtDominanceCallbacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dominance string
		threshold float64
		wantAbove bool
		wantBelow bool
	}{
		{"mayor al umbral", "5.01", 5, true, false},
		{"en el umbral", "5", 5, false, false},
		{"menor al umbral", "4.99", 5, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := usdtQuotesBody(tt.dominance)
			above, err := providers.UsdtDominanceAboveCallback(body, thresholdParameters(tt.threshold))
			testutil.RequireEmptyError(t, err)
			below, err := providers.UsdtDominanceBelowCallback(body, thresholdParameters(tt.threshold))
			testutil.RequireEmptyError(t, err)

			if above != tt.wantAbove || below != tt.wantBelow {
				t.Errorf("dominancia %s umbral %v: above, below = %v, %v, want %v, %v", tt.dominance, tt.threshold, above, below, tt.wantAbove, tt.wantBelow)
			}
		})
	}
}

func TestThresholdCallbacks_Errors(t *testing.T) {
	t.Parallel()

	rateLimit := testutil.ReadFixture(t, "coinmarketcap_rate_limit_error.json")
	noData := []byte(`{"status":{"error_code":0,"error_message":null}}`)
	valid := thresholdParameters(50.0)

	tests := []struct {
		name       string
		callback   providers.CallbackFunc
		body       []byte
		parameters map[string]any
		code       string
	}{
		{"fear and greed sin parametros", providers.FearAndGreedAboveCallback, fearAndGreedBody("50"), nil, "PCCMCE012"},
		{"fear and greed umbral texto", providers.FearAndGreedBelowCallback, fearAndGreedBody("50"), thresholdParameters("50"), "PCCMCE012"},
		{"fear and greed umbral negativo", providers.FearAndGreedAboveCallback, fearAndGreedBody("50"), thresholdParameters(-1.0), "PCCMCE013"},
		{"fear and greed umbral mayor a 100", providers.FearAndGreedBelowCallback, fearAndGreedBody("50"), thresholdParameters(100.1), "PCCMCE013"},
		{"fear and greed valor invalido", providers.FearAndGreedAboveCallback, fearAndGreedBody("101"), valid, "PCCMCE003"},
		{"btc sin umbral", providers.BtcDominanceAboveCallback, dominanceGlobalMetricsBody(55, 15), map[string]any{}, "PCCMCE012"},
		{"btc json invalido", providers.BtcDominanceBelowCallback, []byte(`[`), valid, "PCCMCE004"},
		{"btc sin data", providers.BtcDominanceAboveCallback, noData, valid, "PCCMCE005"},
		{"btc dominancia mayor a 100", providers.BtcDominanceBelowCallback, dominanceGlobalMetricsBody(100.5, 0), valid, "PCCMCE006"},
		{"others2 limite de peticiones", providers.Others2DominanceAboveCallback, rateLimit, valid, "PCCMCE002"},
		{"others2 sin dominancia de btc", providers.Others2DominanceBelowCallback, []byte(`{"data":{"quote":{}},"status":{"error_code":0}}`), valid, "PCCMCE006"},
		{"others3 umbral mayor a 100", providers.Others3DominanceAboveCallback, dominanceGlobalMetricsBody(55, 15), thresholdParameters(int64(101)), "PCCMCE013"},
		{"others3 sin dominancia de eth", providers.Others3DominanceAboveCallback, globalMetricsBody(55, 1000, 100), valid, "PCCMCE014"},
		{"others3 dominancias suman mas de 100", providers.Others3DominanceBelowCallback, dominanceGlobalMetricsBody(90, 10.5), valid, "PCCMCE014"},
		{"others3 dominancia de eth negativa", providers.Others3DominanceBelowCallback, dominanceGlobalMetricsBody(55, -1), valid, "PCCMCE014"},
		{"others3 sin data", providers.Others3DominanceBelowCallback, noData, valid, "PCCMCE005"},
		{"usdt sin umbral", providers.UsdtDominanceAboveCallback, usdtQuotesBody("5"), nil, "PCCMCE012"},
		{"usdt no es JSON", providers.UsdtDominanceAboveCallback, []byte(`<html></html>`), valid, "PCCMCE009"},
		{"usdt error de la API", providers.UsdtDominanceBelowCallback, []byte(`{"status":{"error_code":1002,"error_message":"API key missing."}}`), valid, "PCCMCE002"},
		{"usdt pedido con otro simbolo", providers.UsdtDominanceAboveCallback, []byte(coinMarketCapQuotesBody), valid, "PCCMCE010"},
		{"usdt sin market_cap_dominance", providers.UsdtDominanceBelowCallback, usdtQuotesBody("null"), valid, "PCCMCE015"},
		{"usdt market_cap_dominance mayor a 100", providers.UsdtDominanceAboveCallback, usdtQuotesBody("100.01"), valid, "PCCMCE015"},
		{"usdt sin cotizacion USD", providers.UsdtDominanceAboveCallback, []byte(`{"data":{"USDT":[{"cmc_rank":3,"quote":{"EUR":{"market_cap_dominance":5}}}]},"status":{"error_code":0}}`), valid, "PCCMCE015"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accepted, err := tt.callback(tt.body, tt.parameters)

			testutil.RequireTrackingCode(t, err, tt.code)
			if accepted {
				t.Error("un error no debe aceptar la condicion")
			}
		})
	}
}
