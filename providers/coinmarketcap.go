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
)

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
		Quote        map[string]struct {
			TotalMarketCap      *float64 `json:"total_market_cap"`
			StablecoinMarketCap *float64 `json:"stablecoin_market_cap"`
		} `json:"quote"`
	} `json:"data"`
	Status coinMarketCapStatus `json:"status"`
}

// FearAndGreedExtremeFearCallback retorna true si el indice Fear and Greed esta en Extreme Fear (menor a 20)
func FearAndGreedExtremeFearCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	value, err := parseFearAndGreedLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isExtremeFear := value < fearAndGreedFearMin

	return isExtremeFear, cmm.NewEmptyErrorHandler()
}

// FearAndGreedFearCallback retorna true si el indice Fear and Greed esta en Fear (20 a menos de 40)
func FearAndGreedFearCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	value, err := parseFearAndGreedLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isFear := value >= fearAndGreedFearMin && value < fearAndGreedNeutralMin

	return isFear, cmm.NewEmptyErrorHandler()
}

// FearAndGreedNeutralCallback retorna true si el indice Fear and Greed esta en Neutral (40 a menos de 60)
func FearAndGreedNeutralCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	value, err := parseFearAndGreedLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isNeutral := value >= fearAndGreedNeutralMin && value < fearAndGreedGreedMin

	return isNeutral, cmm.NewEmptyErrorHandler()
}

// FearAndGreedGreedCallback retorna true si el indice Fear and Greed esta en Greed (60 a menos de 80)
func FearAndGreedGreedCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	value, err := parseFearAndGreedLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isGreed := value >= fearAndGreedGreedMin && value < fearAndGreedExtremeGreed

	return isGreed, cmm.NewEmptyErrorHandler()
}

// FearAndGreedExtremeGreedCallback retorna true si el indice Fear and Greed esta en Extreme Greed (80 o mas)
func FearAndGreedExtremeGreedCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	value, err := parseFearAndGreedLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	isExtremeGreed := value >= fearAndGreedExtremeGreed

	return isExtremeGreed, cmm.NewEmptyErrorHandler()
}

// BtcDominanceAbove60Callback retorna true si la dominancia de Bitcoin es mayor a btcDominanceThreshold
func BtcDominanceAbove60Callback(responseData []byte) (bool, *cmm.ErrorHandler) {
	data, err := parseGlobalMetricsLatest(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	if data.Data.BtcDominance == nil || *data.Data.BtcDominance < 0 || *data.Data.BtcDominance > 100 {
		return false, cmm.NewErrorHandler(fmt.Errorf("btc_dominance %v", data.Data.BtcDominance), "CoinMarketCap btc_dominance is missing or out of range [0, 100]", cmm.LevelError, "PCCMCE006")
	}

	isAbove := *data.Data.BtcDominance > btcDominanceThreshold

	return isAbove, cmm.NewEmptyErrorHandler()
}

// StablecoinDominanceAbove12Callback retorna true si el market cap de las stablecoins es mayor a
// stablecoinDominanceThreshold por ciento del market cap total. Una dominancia alta indica liquidez fuera del
// mercado esperando entrar (miedo o capitulacion), historicamente cerca de los minimos
func StablecoinDominanceAbove12Callback(responseData []byte) (bool, *cmm.ErrorHandler) {
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
