package providers

import (
	cmm "c_trd/common"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Rangos del indice Fear and Greed de CoinMarketCap (0 a 100) segun su documentacion. El limite inferior de cada
// rango es inclusivo y el superior exclusivo, salvo Extreme Greed que incluye 100
const (
	fearAndGreedMinValue     = 0
	fearAndGreedFearMin      = 20
	fearAndGreedNeutralMin   = 40
	fearAndGreedGreedMin     = 60
	fearAndGreedExtremeGreed = 80
	fearAndGreedMaxValue     = 100
)

// Umbrales fijos de dominancia en porcentaje del market cap total.
// TODO: cuando existan callback_parameters en las condiciones, el umbral debe salir de la condicion y no de aqui
const (
	btcDominanceThreshold        = 60.0
	stablecoinDominanceThreshold = 12.0

	// Moneda de las cotizaciones de global metrics (parametro convert de la API)
	coinMarketCapQuoteCurrency = "USD"

	// Simbolo de Tether en /v2/cryptocurrency/quotes/latest
	coinMarketCapUsdtSymbol = "USDT"

	// Las dominancias son porcentajes del market cap total
	dominanceMinValue = 0.0
	dominanceMaxValue = 100.0
)

// thresholdParameterName nombre del parametro del callback con el umbral (porcentaje o valor del indice, ambos de 0 a 100)
const thresholdParameterName = "threshold"

// Rango del parametro threshold al crear la condicion. Son variables porque CallbackParameter guarda punteros
var (
	thresholdMinValue = dominanceMinValue
	thresholdMaxValue = dominanceMaxValue
)

// fearAndGreedThresholdParameters parametros de los callbacks de umbral del indice Fear and Greed
var fearAndGreedThresholdParameters = []CallbackParameter{
	{
		Name:        thresholdParameterName,
		Label:       "Umbral",
		Description: "Valor del indice Fear and Greed (0 a 100) contra el que se compara el valor actual",
		DataType:    cmm.ApiParameterTypeFloat,
		IsRequired:  true,
		MinValue:    &thresholdMinValue,
		MaxValue:    &thresholdMaxValue,
	},
}

// dominanceThresholdParameters parametros de los callbacks de umbral de dominancia
var dominanceThresholdParameters = []CallbackParameter{
	{
		Name:        thresholdParameterName,
		Label:       "Umbral (%)",
		Description: "Porcentaje del market cap total (0 a 100) contra el que se compara la dominancia actual",
		DataType:    cmm.ApiParameterTypeFloat,
		IsRequired:  true,
		MinValue:    &thresholdMinValue,
		MaxValue:    &thresholdMaxValue,
	},
}

// coinMarketCapStatus objeto status comun de todas las respuestas. error_code llega como numero en unos endpoints
// (ej: 0 o 1022) y como string en otros (ej: "0" o "1022"), por eso se guarda crudo
type coinMarketCapStatus struct {
	ErrorCode    json.RawMessage `json:"error_code"`
	ErrorMessage *string         `json:"error_message"`
}

// coinMarketCapFearAndGreedResponse estructura minima de la respuesta de /v3/fear-and-greed/latest
type coinMarketCapFearAndGreedResponse struct {
	Data *struct {
		Value               *float64 `json:"value"`
		ValueClassification string   `json:"value_classification"`
		UpdateTime          string   `json:"update_time"`
	} `json:"data"`
	Status coinMarketCapStatus `json:"status"`
}

// coinMarketCapGlobalMetricsResponse estructura minima de la respuesta de /v1/global-metrics/quotes/latest
type coinMarketCapGlobalMetricsResponse struct {
	Data *struct {
		BtcDominance *float64 `json:"btc_dominance"`
		EthDominance *float64 `json:"eth_dominance"`
		Quote        map[string]struct {
			TotalMarketCap      *float64 `json:"total_market_cap"`
			StablecoinMarketCap *float64 `json:"stablecoin_market_cap"`
		} `json:"quote"`
	} `json:"data"`
	Status coinMarketCapStatus `json:"status"`
}

// coinMarketCapQuotesResponse estructura minima de la respuesta de /v2/cryptocurrency/quotes/latest pedida por symbol.
// Cada simbolo trae un arreglo porque varias criptomonedas pueden compartir simbolo
type coinMarketCapQuotesResponse struct {
	Data   map[string][]coinMarketCapQuote `json:"data"`
	Status coinMarketCapStatus             `json:"status"`
}

// coinMarketCapQuote criptomoneda de /v2/cryptocurrency/quotes/latest con sus cotizaciones por moneda.
// market_cap_dominance es el porcentaje del market cap total que representa la criptomoneda
type coinMarketCapQuote struct {
	Symbol  string `json:"symbol"`
	CmcRank *int   `json:"cmc_rank"`
	Quote   map[string]struct {
		Price              *float64 `json:"price"`
		MarketCapDominance *float64 `json:"market_cap_dominance"`
	} `json:"quote"`
}

// FearAndGreedExtremeFearCallback retorna true si el indice Fear and Greed esta en Extreme Fear (menor a 20)
func FearAndGreedExtremeFearCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	value, err := parseFearAndGreedLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isExtremeFear := value < fearAndGreedFearMin

	return isExtremeFear, cmm.NewEmptyErrorHandler()
}

// FearAndGreedFearCallback retorna true si el indice Fear and Greed esta en Fear (20 a menos de 40)
func FearAndGreedFearCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	value, err := parseFearAndGreedLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isFear := value >= fearAndGreedFearMin && value < fearAndGreedNeutralMin

	return isFear, cmm.NewEmptyErrorHandler()
}

// FearAndGreedNeutralCallback retorna true si el indice Fear and Greed esta en Neutral (40 a menos de 60)
func FearAndGreedNeutralCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	value, err := parseFearAndGreedLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isNeutral := value >= fearAndGreedNeutralMin && value < fearAndGreedGreedMin

	return isNeutral, cmm.NewEmptyErrorHandler()
}

// FearAndGreedGreedCallback retorna true si el indice Fear and Greed esta en Greed (60 a menos de 80)
func FearAndGreedGreedCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	value, err := parseFearAndGreedLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isGreed := value >= fearAndGreedGreedMin && value < fearAndGreedExtremeGreed

	return isGreed, cmm.NewEmptyErrorHandler()
}

// FearAndGreedExtremeGreedCallback retorna true si el indice Fear and Greed esta en Extreme Greed (80 o mas)
func FearAndGreedExtremeGreedCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	value, err := parseFearAndGreedLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isExtremeGreed := value >= fearAndGreedExtremeGreed

	return isExtremeGreed, cmm.NewEmptyErrorHandler()
}

// BtcDominanceAbove60Callback retorna true si la dominancia de Bitcoin es mayor a btcDominanceThreshold
func BtcDominanceAbove60Callback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	btcDominance, err := parseBtcDominance(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isAbove := btcDominance > btcDominanceThreshold

	return isAbove, cmm.NewEmptyErrorHandler()
}

// StablecoinDominanceAbove12Callback retorna true si el market cap de las stablecoins es mayor a
// stablecoinDominanceThreshold por ciento del market cap total. Una dominancia alta indica liquidez fuera del
// mercado esperando entrar (miedo o capitulacion), historicamente cerca de los minimos
func StablecoinDominanceAbove12Callback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	data, err := parseGlobalMetricsLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	quote, exists := data.Data.Quote[coinMarketCapQuoteCurrency]
	if !exists {
		return false, cmm.NewErrorHandler(fmt.Errorf("quote currencies %d", len(data.Data.Quote)), "CoinMarketCap global metrics has no USD quote", cmm.LevelError, "PCCMCE007")
	}

	if quote.TotalMarketCap == nil || quote.StablecoinMarketCap == nil || *quote.TotalMarketCap <= 0 || *quote.StablecoinMarketCap < 0 {
		return false, cmm.NewErrorHandler(fmt.Errorf("total_market_cap %v stablecoin_market_cap %v", quote.TotalMarketCap, quote.StablecoinMarketCap), "CoinMarketCap market caps are missing or invalid", cmm.LevelError, "PCCMCE008")
	}

	dominance := sharePercentage(*quote.StablecoinMarketCap, *quote.TotalMarketCap)
	isAbove := dominance > stablecoinDominanceThreshold

	return isAbove, cmm.NewEmptyErrorHandler()
}

// FearAndGreedAboveCallback retorna true si el indice Fear and Greed es mayor al parametro threshold
func FearAndGreedAboveCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	threshold, err := parseThresholdParameter(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	value, err := parseFearAndGreedLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isAbove := value > threshold

	return isAbove, cmm.NewEmptyErrorHandler()
}

// FearAndGreedBelowCallback retorna true si el indice Fear and Greed es menor al parametro threshold
func FearAndGreedBelowCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	threshold, err := parseThresholdParameter(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	value, err := parseFearAndGreedLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isBelow := value < threshold

	return isBelow, cmm.NewEmptyErrorHandler()
}

// BtcDominanceAboveCallback retorna true si la dominancia de Bitcoin (BTC.D) es mayor al parametro threshold
func BtcDominanceAboveCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	threshold, err := parseThresholdParameter(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	btcDominance, err := parseBtcDominance(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isAbove := btcDominance > threshold

	return isAbove, cmm.NewEmptyErrorHandler()
}

// BtcDominanceBelowCallback retorna true si la dominancia de Bitcoin (BTC.D) es menor al parametro threshold
func BtcDominanceBelowCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	threshold, err := parseThresholdParameter(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	btcDominance, err := parseBtcDominance(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isBelow := btcDominance < threshold

	return isBelow, cmm.NewEmptyErrorHandler()
}

// Others2DominanceAboveCallback retorna true si la dominancia del mercado sin Bitcoin (OTHERS2 = 100 - BTC.D)
// es mayor al parametro threshold
func Others2DominanceAboveCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	threshold, err := parseThresholdParameter(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	btcDominance, err := parseBtcDominance(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	others2Dominance := dominanceMaxValue - btcDominance
	isAbove := others2Dominance > threshold

	return isAbove, cmm.NewEmptyErrorHandler()
}

// Others2DominanceBelowCallback retorna true si la dominancia del mercado sin Bitcoin (OTHERS2 = 100 - BTC.D)
// es menor al parametro threshold
func Others2DominanceBelowCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	threshold, err := parseThresholdParameter(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	btcDominance, err := parseBtcDominance(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	others2Dominance := dominanceMaxValue - btcDominance
	isBelow := others2Dominance < threshold

	return isBelow, cmm.NewEmptyErrorHandler()
}

// Others3DominanceAboveCallback retorna true si la dominancia del mercado sin Bitcoin ni Ethereum
// (OTHERS3 = 100 - BTC.D - ETH.D) es mayor al parametro threshold
func Others3DominanceAboveCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	threshold, err := parseThresholdParameter(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	others3Dominance, err := parseOthers3Dominance(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isAbove := others3Dominance > threshold

	return isAbove, cmm.NewEmptyErrorHandler()
}

// Others3DominanceBelowCallback retorna true si la dominancia del mercado sin Bitcoin ni Ethereum
// (OTHERS3 = 100 - BTC.D - ETH.D) es menor al parametro threshold
func Others3DominanceBelowCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	threshold, err := parseThresholdParameter(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	others3Dominance, err := parseOthers3Dominance(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isBelow := others3Dominance < threshold

	return isBelow, cmm.NewEmptyErrorHandler()
}

// UsdtDominanceAboveCallback retorna true si la dominancia de Tether (USDT.D) es mayor al parametro threshold.
// Lee la respuesta de /v2/cryptocurrency/quotes/latest pedida con symbol=USDT
func UsdtDominanceAboveCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	threshold, err := parseThresholdParameter(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	usdtDominance, err := parseUsdtDominance(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isAbove := usdtDominance > threshold

	return isAbove, cmm.NewEmptyErrorHandler()
}

// UsdtDominanceBelowCallback retorna true si la dominancia de Tether (USDT.D) es menor al parametro threshold.
// Lee la respuesta de /v2/cryptocurrency/quotes/latest pedida con symbol=USDT
func UsdtDominanceBelowCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	threshold, err := parseThresholdParameter(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	usdtDominance, err := parseUsdtDominance(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isBelow := usdtDominance < threshold

	return isBelow, cmm.NewEmptyErrorHandler()
}

// parseThresholdParameter retorna el parametro threshold del callback en el rango [0, 100]. Se guarda como FLOAT,
// pero se aceptan enteros por si la condicion se guardo a mano en Mongo
func parseThresholdParameter(parameters map[string]any) (float64, *cmm.ErrorHandler) {
	var threshold float64
	switch value := parameters[thresholdParameterName].(type) {
	case float64:
		threshold = value
	case int64:
		threshold = float64(value)
	case int32:
		threshold = float64(value)
	case int:
		threshold = float64(value)
	default:
		return 0, cmm.NewErrorHandler(fmt.Errorf("threshold %T %v", value, value), "The callback threshold parameter is missing or is not a number", cmm.LevelError, "PCCMCE012")
	}

	if threshold < dominanceMinValue || threshold > dominanceMaxValue {
		return 0, cmm.NewErrorHandler(fmt.Errorf("threshold %v", threshold), "The callback threshold parameter is out of range [0, 100]", cmm.LevelError, "PCCMCE013")
	}

	return threshold, cmm.NewEmptyErrorHandler()
}

// parseBtcDominance retorna la dominancia de Bitcoin de la respuesta de global metrics
func parseBtcDominance(responseData []byte) (float64, *cmm.ErrorHandler) {
	data, err := parseGlobalMetricsLatest(responseData)
	if !err.IsEmtpy() {
		return 0, err
	}

	if !isValidDominance(data.Data.BtcDominance) {
		return 0, cmm.NewErrorHandler(fmt.Errorf("btc_dominance %v", data.Data.BtcDominance), "CoinMarketCap btc_dominance is missing or out of range [0, 100]", cmm.LevelError, "PCCMCE006")
	}

	return *data.Data.BtcDominance, cmm.NewEmptyErrorHandler()
}

// parseOthers3Dominance retorna la dominancia del mercado sin Bitcoin ni Ethereum (100 - BTC.D - ETH.D)
func parseOthers3Dominance(responseData []byte) (float64, *cmm.ErrorHandler) {
	data, err := parseGlobalMetricsLatest(responseData)
	if !err.IsEmtpy() {
		return 0, err
	}

	btcDominance := data.Data.BtcDominance
	ethDominance := data.Data.EthDominance
	if !isValidDominance(btcDominance) || !isValidDominance(ethDominance) || *btcDominance+*ethDominance > dominanceMaxValue {
		return 0, cmm.NewErrorHandler(fmt.Errorf("btc_dominance %v eth_dominance %v", btcDominance, ethDominance), "CoinMarketCap btc_dominance or eth_dominance is missing or out of range [0, 100]", cmm.LevelError, "PCCMCE014")
	}

	return dominanceMaxValue - *btcDominance - *ethDominance, cmm.NewEmptyErrorHandler()
}

// parseUsdtDominance retorna el market_cap_dominance en USD de Tether de la respuesta de /v2/cryptocurrency/quotes/latest
func parseUsdtDominance(responseData []byte) (float64, *cmm.ErrorHandler) {
	data, err := parseQuotesLatest(responseData)
	if !err.IsEmtpy() {
		return 0, err
	}

	coin, err := bestRankedCoin(data, coinMarketCapUsdtSymbol)
	if !err.IsEmtpy() {
		return 0, err
	}

	quote, exists := coin.Quote[coinMarketCapQuoteCurrency]
	if !exists || !isValidDominance(quote.MarketCapDominance) {
		return 0, cmm.NewErrorHandler(fmt.Errorf("market_cap_dominance %v", quote.MarketCapDominance), "CoinMarketCap USDT market_cap_dominance is missing or out of range [0, 100]", cmm.LevelError, "PCCMCE015")
	}

	return *quote.MarketCapDominance, cmm.NewEmptyErrorHandler()
}

// isValidDominance retorna true si la dominancia existe y esta en el rango [0, 100]
func isValidDominance(dominance *float64) bool {
	return dominance != nil && *dominance >= dominanceMinValue && *dominance <= dominanceMaxValue
}

// parseFearAndGreedLatest retorna el valor actual del indice Fear and Greed. Se usa el valor y no
// value_classification para no depender del texto de la clasificacion
func parseFearAndGreedLatest(responseData []byte) (float64, *cmm.ErrorHandler) {
	var data coinMarketCapFearAndGreedResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return 0, cmm.NewErrorHandler(err, "Error parsing CoinMarketCap Fear and Greed response", cmm.LevelFatal, "PCCMCE001")
	}

	err := coinMarketCapStatusError(data.Status)
	if !err.IsEmtpy() {
		return 0, err
	}

	if data.Data == nil || data.Data.Value == nil || *data.Data.Value < fearAndGreedMinValue || *data.Data.Value > fearAndGreedMaxValue {
		return 0, cmm.NewErrorHandler(errors.New("value missing or out of range [0, 100]"), "CoinMarketCap Fear and Greed value is invalid", cmm.LevelError, "PCCMCE003")
	}

	return *data.Data.Value, cmm.NewEmptyErrorHandler()
}

// parseGlobalMetricsLatest retorna la respuesta de global metrics validando el status y que exista data
func parseGlobalMetricsLatest(responseData []byte) (coinMarketCapGlobalMetricsResponse, *cmm.ErrorHandler) {
	var data coinMarketCapGlobalMetricsResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return data, cmm.NewErrorHandler(err, "Error parsing CoinMarketCap global metrics response", cmm.LevelFatal, "PCCMCE004")
	}

	err := coinMarketCapStatusError(data.Status)
	if !err.IsEmtpy() {
		return data, err
	}

	if data.Data == nil {
		return data, cmm.NewErrorHandler(errors.New("data is null"), "CoinMarketCap global metrics response has no data", cmm.LevelError, "PCCMCE005")
	}

	return data, cmm.NewEmptyErrorHandler()
}

// coinMarketCapStatusError retorna error si el status trae un error_code distinto de 0 (numero o string).
// Sin error_code o con null se considera exitoso
func coinMarketCapStatusError(status coinMarketCapStatus) *cmm.ErrorHandler {
	code := strings.Trim(string(status.ErrorCode), `"`)
	if code == "" || code == "null" || code == "0" {
		return cmm.NewEmptyErrorHandler()
	}

	message := ""
	if status.ErrorMessage != nil {
		message = *status.ErrorMessage
	}

	return cmm.NewErrorHandler(fmt.Errorf("error_code %s: %w", code, errors.New(message)), "CoinMarketCap returned an error message", cmm.LevelError, "PCCMCE002")
}

// CoinMarketCapQuotePrice retorna el precio en USD del simbolo en la respuesta de /v2/cryptocurrency/quotes/latest.
// Si varias criptomonedas comparten el simbolo se usa la de mejor cmc_rank; las que no tienen rank estan inactivas
func CoinMarketCapQuotePrice(responseData []byte, symbol string) (float64, *cmm.ErrorHandler) {
	data, err := parseQuotesLatest(responseData)
	if !err.IsEmtpy() {
		return 0, err
	}

	coin, err := bestRankedCoin(data, symbol)
	if !err.IsEmtpy() {
		return 0, err
	}

	quote, exists := coin.Quote[coinMarketCapQuoteCurrency]
	if !exists || quote.Price == nil || *quote.Price <= 0 {
		return 0, cmm.NewErrorHandler(fmt.Errorf("symbol %q price %v", symbol, quote.Price), "CoinMarketCap USD price is missing or invalid", cmm.LevelError, "PCCMCE011")
	}

	return *quote.Price, cmm.NewEmptyErrorHandler()
}

// parseQuotesLatest retorna la respuesta de /v2/cryptocurrency/quotes/latest validando el status
func parseQuotesLatest(responseData []byte) (coinMarketCapQuotesResponse, *cmm.ErrorHandler) {
	var data coinMarketCapQuotesResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return data, cmm.NewErrorHandler(err, "Error parsing CoinMarketCap quotes response", cmm.LevelFatal, "PCCMCE009")
	}

	err := coinMarketCapStatusError(data.Status)
	if !err.IsEmtpy() {
		return data, err
	}

	return data, cmm.NewEmptyErrorHandler()
}

// bestRankedCoin retorna la criptomoneda del simbolo con mejor cmc_rank. Las que no tienen rank estan inactivas
func bestRankedCoin(data coinMarketCapQuotesResponse, symbol string) (coinMarketCapQuote, *cmm.ErrorHandler) {
	bestIndex := -1
	coins := data.Data[strings.ToUpper(symbol)]
	for i, coin := range coins {
		if coin.CmcRank == nil {
			continue
		}
		if bestIndex < 0 || *coin.CmcRank < *coins[bestIndex].CmcRank {
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return coinMarketCapQuote{}, cmm.NewErrorHandler(fmt.Errorf("symbol %q coins %d", symbol, len(coins)), "CoinMarketCap has no active cryptocurrency for the symbol", cmm.LevelError, "PCCMCE010")
	}

	return coins[bestIndex], cmm.NewEmptyErrorHandler()
}
