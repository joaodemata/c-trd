package providers

import (
	cmm "c_trd/common"
	"sort"
)

// CallbackFunc defines the signature for dynamic callback functions.
// Recibe el body de la respuesta HTTP y el parameters_callback de la condicion (ya tipados segun su data_type)
// Todos los callbacks solo deberian recibir un bool
type CallbackFunc func(responseBody []byte, parameters map[string]any) (bool, *cmm.ErrorHandler)

// CallbackParameter parametro que acepta un callback (ej: umbral). El callback solo define el parametro,
// el valor lo guarda cada condicion en parameters_callback
type CallbackParameter struct {
	Name         string  `json:"name"`
	Label        string  `json:"label"`
	Description  string  `json:"description"`
	DataType     string  `json:"dataType"`
	IsRequired   bool    `json:"isRequired"`
	DefaultValue *string `json:"defaultValue"`
}

// CallbackDefinition callback registrado y el formato de respuesta que sabe leer. Solo es compatible con
// las APIs que tengan el mismo response_format. ID es la llave del registro y lo que se guarda en callback_function,
// Tag y Name son para mostrarlo en el frontend
type CallbackDefinition struct {
	ID             string
	Tag            string
	Name           string
	Fn             CallbackFunc
	Description    string
	ResponseFormat string
	// Parametros que la condicion puede personalizar, nil si no tiene
	Parameters []CallbackParameter
}

// CallbackInfo datos publicos de un callback para listarlo
type CallbackInfo struct {
	ID             string              `json:"id"`
	Tag            string              `json:"tag"`
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	ResponseFormat string              `json:"responseFormat"`
	Parameters     []CallbackParameter `json:"parameters"`
}

// FunctionRegistry maps a string (from the database) to an actual Go function.
var FunctionRegistry = map[string]CallbackDefinition{
	"ProcessAlphaVantageData": {
		ID:             "ProcessAlphaVantageData",
		Tag:            "ALPHA-VANTAGE-PUT-CALL-RATIO",
		Name:           "Put/Call ratio extremo",
		Fn:             RealtimePutCallRatioCallback,
		Description:    "Put/Call ratio mayor a 1 o menor o igual a 0.6",
		ResponseFormat: cmm.ResponseFormatAlphaVantagePutCallRatio,
	},
	"GoogleTrendsHigh": {
		ID:             "GoogleTrendsHigh",
		Tag:            "GOOGLE-TRENDS-HIGH",
		Name:           "Google Trends en zona alta",
		Fn:             GoogleTrendsHighCallback,
		Description:    "El ultimo valor de interes esta en la zona alta del rango de la serie",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
	},
	"GoogleTrendsLow": {
		ID:             "GoogleTrendsLow",
		Tag:            "GOOGLE-TRENDS-LOW",
		Name:           "Google Trends en zona baja",
		Fn:             GoogleTrendsLowCallback,
		Description:    "El ultimo valor de interes esta en la zona baja del rango de la serie",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
	},
	"GoogleTrendsFileHigh": {
		ID:             "GoogleTrendsFileHigh",
		Tag:            "GOOGLE-TRENDS-FILE-HIGH",
		Name:           "CSV de Google Trends en zona alta",
		Fn:             GoogleTrendsFileHighCallback,
		Description:    "El ultimo periodo del CSV esta en la zona alta del rango",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
	},
	"GoogleTrendsFileLow": {
		ID:             "GoogleTrendsFileLow",
		Tag:            "GOOGLE-TRENDS-FILE-LOW",
		Name:           "CSV de Google Trends en zona baja",
		Fn:             GoogleTrendsFileLowCallback,
		Description:    "El ultimo periodo del CSV esta en la zona baja del rango",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
	},
	"GoogleTrendsFileNewMax": {
		ID:             "GoogleTrendsFileNewMax",
		Tag:            "GOOGLE-TRENDS-FILE-NEW-MAX",
		Name:           "CSV de Google Trends en nuevo maximo",
		Fn:             GoogleTrendsFileNewMaxCallback,
		Description:    "El ultimo periodo del CSV es un nuevo maximo de la serie",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
	},
	"GoogleTrendsFileNewMin": {
		ID:             "GoogleTrendsFileNewMin",
		Tag:            "GOOGLE-TRENDS-FILE-NEW-MIN",
		Name:           "CSV de Google Trends en nuevo minimo",
		Fn:             GoogleTrendsFileNewMinCallback,
		Description:    "El ultimo periodo del CSV es un nuevo minimo de la serie",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
	},
	"FredSeriesRising": {
		ID:             "FredSeriesRising",
		Tag:            "FRED-SERIES-RISING",
		Name:           "Serie de FRED en subida",
		Fn:             FredSeriesRisingCallback,
		Description:    "La regresion lineal de la ventana de la serie de FRED tiene pendiente positiva",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
	},
	"FredSeriesFalling": {
		ID:             "FredSeriesFalling",
		Tag:            "FRED-SERIES-FALLING",
		Name:           "Serie de FRED en bajada",
		Fn:             FredSeriesFallingCallback,
		Description:    "La regresion lineal de la ventana de la serie de FRED tiene pendiente negativa",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
	},
	"FedNextMeetingCutLikely": {
		ID:             "FedNextMeetingCutLikely",
		Tag:            "FED-NEXT-MEETING-CUT-LIKELY",
		Name:           "FED: bajada de tasa probable",
		Fn:             FedNextMeetingCutLikely,
		Description:    "Polymarket da 60% o mas de probabilidad a una bajada de tasa en la proxima reunion de la FED",
		ResponseFormat: cmm.ResponseFormatPolymarketFedDecisions,
	},
	"FedNextMeetingHoldLikely": {
		ID:             "FedNextMeetingHoldLikely",
		Tag:            "FED-NEXT-MEETING-HOLD-LIKELY",
		Name:           "FED: mantener la tasa probable",
		Fn:             FedNextMeetingHoldLikely,
		Description:    "Polymarket da 60% o mas de probabilidad a mantener la tasa en la proxima reunion de la FED",
		ResponseFormat: cmm.ResponseFormatPolymarketFedDecisions,
	},
	"FedNextMeetingHikeLikely": {
		ID:             "FedNextMeetingHikeLikely",
		Tag:            "FED-NEXT-MEETING-HIKE-LIKELY",
		Name:           "FED: subida de tasa probable",
		Fn:             FedNextMeetingHikeLikely,
		Description:    "Polymarket da 60% o mas de probabilidad a una subida de tasa en la proxima reunion de la FED",
		ResponseFormat: cmm.ResponseFormatPolymarketFedDecisions,
	},
	"FearAndGreedExtremeFear": {
		ID:             "FearAndGreedExtremeFear",
		Tag:            "FEAR-AND-GREED-EXTREME-FEAR",
		Name:           "Fear and Greed: Extreme Fear",
		Fn:             FearAndGreedExtremeFearCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap esta en Extreme Fear (menor a 20)",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
	},
	"FearAndGreedFear": {
		ID:             "FearAndGreedFear",
		Tag:            "FEAR-AND-GREED-FEAR",
		Name:           "Fear and Greed: Fear",
		Fn:             FearAndGreedFearCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap esta en Fear (20 a menos de 40)",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
	},
	"FearAndGreedNeutral": {
		ID:             "FearAndGreedNeutral",
		Tag:            "FEAR-AND-GREED-NEUTRAL",
		Name:           "Fear and Greed: Neutral",
		Fn:             FearAndGreedNeutralCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap esta en Neutral (40 a menos de 60)",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
	},
	"FearAndGreedGreed": {
		ID:             "FearAndGreedGreed",
		Tag:            "FEAR-AND-GREED-GREED",
		Name:           "Fear and Greed: Greed",
		Fn:             FearAndGreedGreedCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap esta en Greed (60 a menos de 80)",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
	},
	"FearAndGreedExtremeGreed": {
		ID:             "FearAndGreedExtremeGreed",
		Tag:            "FEAR-AND-GREED-EXTREME-GREED",
		Name:           "Fear and Greed: Extreme Greed",
		Fn:             FearAndGreedExtremeGreedCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap esta en Extreme Greed (80 o mas)",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
	},
	"BtcDominanceAbove60": {
		ID:             "BtcDominanceAbove60",
		Tag:            "BTC-DOMINANCE-ABOVE-60",
		Name:           "Dominancia de BTC mayor a 60%",
		Fn:             BtcDominanceAbove60Callback,
		Description:    "La dominancia de Bitcoin en CoinMarketCap es mayor a 60%",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
	},
	"StablecoinDominanceAbove12": {
		ID:             "StablecoinDominanceAbove12",
		Tag:            "STABLECOIN-DOMINANCE-ABOVE-12",
		Name:           "Dominancia de stablecoins mayor a 12%",
		Fn:             StablecoinDominanceAbove12Callback,
		Description:    "El market cap de las stablecoins en CoinMarketCap es mayor al 12% del market cap total",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
	},
	"FearAndGreedAbove": {
		ID:             "FearAndGreedAbove",
		Tag:            "FEAR-AND-GREED-ABOVE",
		Name:           "Fear and Greed mayor al umbral",
		Fn:             FearAndGreedAboveCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap es mayor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
		Parameters:     fearAndGreedThresholdParameters,
	},
	"FearAndGreedBelow": {
		ID:             "FearAndGreedBelow",
		Tag:            "FEAR-AND-GREED-BELOW",
		Name:           "Fear and Greed menor al umbral",
		Fn:             FearAndGreedBelowCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap es menor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
		Parameters:     fearAndGreedThresholdParameters,
	},
	"BtcDominanceAbove": {
		ID:             "BtcDominanceAbove",
		Tag:            "BTC-DOMINANCE-ABOVE",
		Name:           "Dominancia de BTC mayor al umbral",
		Fn:             BtcDominanceAboveCallback,
		Description:    "La dominancia de Bitcoin (BTC.D) en CoinMarketCap es mayor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		Parameters:     dominanceThresholdParameters,
	},
	"BtcDominanceBelow": {
		ID:             "BtcDominanceBelow",
		Tag:            "BTC-DOMINANCE-BELOW",
		Name:           "Dominancia de BTC menor al umbral",
		Fn:             BtcDominanceBelowCallback,
		Description:    "La dominancia de Bitcoin (BTC.D) en CoinMarketCap es menor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		Parameters:     dominanceThresholdParameters,
	},
	"Others2DominanceAbove": {
		ID:             "Others2DominanceAbove",
		Tag:            "OTHERS2-DOMINANCE-ABOVE",
		Name:           "Dominancia de OTHERS2 mayor al umbral",
		Fn:             Others2DominanceAboveCallback,
		Description:    "La dominancia del mercado sin BTC (100 - BTC.D) en CoinMarketCap es mayor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		Parameters:     dominanceThresholdParameters,
	},
	"Others2DominanceBelow": {
		ID:             "Others2DominanceBelow",
		Tag:            "OTHERS2-DOMINANCE-BELOW",
		Name:           "Dominancia de OTHERS2 menor al umbral",
		Fn:             Others2DominanceBelowCallback,
		Description:    "La dominancia del mercado sin BTC (100 - BTC.D) en CoinMarketCap es menor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		Parameters:     dominanceThresholdParameters,
	},
	"Others3DominanceAbove": {
		ID:             "Others3DominanceAbove",
		Tag:            "OTHERS3-DOMINANCE-ABOVE",
		Name:           "Dominancia de OTHERS3 mayor al umbral",
		Fn:             Others3DominanceAboveCallback,
		Description:    "La dominancia del mercado sin BTC ni ETH (100 - BTC.D - ETH.D) en CoinMarketCap es mayor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		Parameters:     dominanceThresholdParameters,
	},
	"Others3DominanceBelow": {
		ID:             "Others3DominanceBelow",
		Tag:            "OTHERS3-DOMINANCE-BELOW",
		Name:           "Dominancia de OTHERS3 menor al umbral",
		Fn:             Others3DominanceBelowCallback,
		Description:    "La dominancia del mercado sin BTC ni ETH (100 - BTC.D - ETH.D) en CoinMarketCap es menor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		Parameters:     dominanceThresholdParameters,
	},
	"UsdtDominanceAbove": {
		ID:             "UsdtDominanceAbove",
		Tag:            "USDT-DOMINANCE-ABOVE",
		Name:           "Dominancia de USDT mayor al umbral",
		Fn:             UsdtDominanceAboveCallback,
		Description:    "La dominancia de Tether (USDT.D) en CoinMarketCap es mayor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapUsdtQuote,
		Parameters:     dominanceThresholdParameters,
	},
	"UsdtDominanceBelow": {
		ID:             "UsdtDominanceBelow",
		Tag:            "USDT-DOMINANCE-BELOW",
		Name:           "Dominancia de USDT menor al umbral",
		Fn:             UsdtDominanceBelowCallback,
		Description:    "La dominancia de Tether (USDT.D) en CoinMarketCap es menor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapUsdtQuote,
		Parameters:     dominanceThresholdParameters,
	},
}

// ListCallbacksByResponseFormat retorna los callbacks que leen el formato de respuesta indicado, ordenados por ID.
// Si ninguno coincide retorna [] y no nil
func ListCallbacksByResponseFormat(responseFormat string) []CallbackInfo {
	result := make([]CallbackInfo, 0)
	for _, definition := range FunctionRegistry {
		if definition.ResponseFormat == responseFormat {
			parameters := definition.Parameters
			if parameters == nil {
				parameters = []CallbackParameter{}
			}
			result = append(result, CallbackInfo{
				ID:             definition.ID,
				Tag:            definition.Tag,
				Name:           definition.Name,
				Description:    definition.Description,
				ResponseFormat: definition.ResponseFormat,
				Parameters:     parameters,
			})
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
