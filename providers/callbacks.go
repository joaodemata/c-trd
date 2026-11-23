package providers

import (
	cmm "c_trd/common"
	"sort"
	"time"
)

// CallbackResult resultado de un callback: si la condicion se cumple, el valor medido con el que se decidio y la
// fecha propia del dato segun el proveedor. AsOf es nil si la respuesta no trae fecha; el callback nunca la reemplaza
// con la hora del servidor, eso lo decide la evaluacion (as_of_source)
type CallbackResult struct {
	Accepted bool
	Value    float64
	AsOf     *time.Time
}

// CallbackFunc defines the signature for dynamic callback functions.
// Recibe el body de la respuesta HTTP y el parameters_callback de la condicion (ya tipados segun su data_type)
// Todos los callbacks retornan un CallbackResult; si hay error retornan CallbackResult{}
type CallbackFunc func(responseBody []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler)

// CallbackParameter parametro que acepta un callback (ej: umbral). El callback solo define el parametro,
// el valor lo guarda cada condicion en parameters_callback
type CallbackParameter struct {
	Name         string  `json:"name"`
	Label        string  `json:"label"`
	Description  string  `json:"description"`
	DataType     string  `json:"dataType"`
	IsRequired   bool    `json:"isRequired"`
	DefaultValue *string `json:"defaultValue"`
	// Rango inclusivo del valor, solo para INTEGER y FLOAT. nil si no tiene limite
	MinValue *float64 `json:"minValue"`
	MaxValue *float64 `json:"maxValue"`
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
	// Antiguedad maxima del dato (executed_at - as_of). Si el AsOf del resultado es mas viejo la evaluacion se
	// considera fallida. 0 no limita, y si el callback no trae AsOf (nil) no se puede validar
	MaxDataAge time.Duration
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
		MaxDataAge:     4 * 24 * time.Hour,
	},
	"GoogleTrendsHigh": {
		ID:             "GoogleTrendsHigh",
		Tag:            "GOOGLE-TRENDS-HIGH",
		Name:           "Google Trends en zona alta",
		Fn:             GoogleTrendsHighCallback,
		Description:    "El ultimo valor de interes esta en la zona alta del rango de la serie",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
		MaxDataAge:     14 * 24 * time.Hour,
	},
	"GoogleTrendsLow": {
		ID:             "GoogleTrendsLow",
		Tag:            "GOOGLE-TRENDS-LOW",
		Name:           "Google Trends en zona baja",
		Fn:             GoogleTrendsLowCallback,
		Description:    "El ultimo valor de interes esta en la zona baja del rango de la serie",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
		MaxDataAge:     14 * 24 * time.Hour,
	},
	"GoogleTrendsFileHigh": {
		ID:             "GoogleTrendsFileHigh",
		Tag:            "GOOGLE-TRENDS-FILE-HIGH",
		Name:           "CSV de Google Trends en zona alta",
		Fn:             GoogleTrendsFileHighCallback,
		Description:    "El ultimo periodo del CSV esta en la zona alta del rango",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
	},
	"GoogleTrendsFileLow": {
		ID:             "GoogleTrendsFileLow",
		Tag:            "GOOGLE-TRENDS-FILE-LOW",
		Name:           "CSV de Google Trends en zona baja",
		Fn:             GoogleTrendsFileLowCallback,
		Description:    "El ultimo periodo del CSV esta en la zona baja del rango",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
	},
	"GoogleTrendsFileNewMax": {
		ID:             "GoogleTrendsFileNewMax",
		Tag:            "GOOGLE-TRENDS-FILE-NEW-MAX",
		Name:           "CSV de Google Trends en nuevo maximo",
		Fn:             GoogleTrendsFileNewMaxCallback,
		Description:    "El ultimo periodo del CSV es un nuevo maximo de la serie",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
	},
	"GoogleTrendsFileNewMin": {
		ID:             "GoogleTrendsFileNewMin",
		Tag:            "GOOGLE-TRENDS-FILE-NEW-MIN",
		Name:           "CSV de Google Trends en nuevo minimo",
		Fn:             GoogleTrendsFileNewMinCallback,
		Description:    "El ultimo periodo del CSV es un nuevo minimo de la serie",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
	},
	"FredSeriesRising": {
		ID:             "FredSeriesRising",
		Tag:            "FRED-SERIES-RISING",
		Name:           "Serie de FRED en subida",
		Fn:             FredSeriesRisingCallback,
		Description:    "La regresion lineal de la ventana de la serie de FRED tiene pendiente positiva",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
		MaxDataAge:     200 * 24 * time.Hour,
	},
	"FredSeriesFalling": {
		ID:             "FredSeriesFalling",
		Tag:            "FRED-SERIES-FALLING",
		Name:           "Serie de FRED en bajada",
		Fn:             FredSeriesFallingCallback,
		Description:    "La regresion lineal de la ventana de la serie de FRED tiene pendiente negativa",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
		MaxDataAge:     200 * 24 * time.Hour,
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
		MaxDataAge:     2 * 24 * time.Hour,
	},
	"FearAndGreedFear": {
		ID:             "FearAndGreedFear",
		Tag:            "FEAR-AND-GREED-FEAR",
		Name:           "Fear and Greed: Fear",
		Fn:             FearAndGreedFearCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap esta en Fear (20 a menos de 40)",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
		MaxDataAge:     2 * 24 * time.Hour,
	},
	"FearAndGreedNeutral": {
		ID:             "FearAndGreedNeutral",
		Tag:            "FEAR-AND-GREED-NEUTRAL",
		Name:           "Fear and Greed: Neutral",
		Fn:             FearAndGreedNeutralCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap esta en Neutral (40 a menos de 60)",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
		MaxDataAge:     2 * 24 * time.Hour,
	},
	"FearAndGreedGreed": {
		ID:             "FearAndGreedGreed",
		Tag:            "FEAR-AND-GREED-GREED",
		Name:           "Fear and Greed: Greed",
		Fn:             FearAndGreedGreedCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap esta en Greed (60 a menos de 80)",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
		MaxDataAge:     2 * 24 * time.Hour,
	},
	"FearAndGreedExtremeGreed": {
		ID:             "FearAndGreedExtremeGreed",
		Tag:            "FEAR-AND-GREED-EXTREME-GREED",
		Name:           "Fear and Greed: Extreme Greed",
		Fn:             FearAndGreedExtremeGreedCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap esta en Extreme Greed (80 o mas)",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
		MaxDataAge:     2 * 24 * time.Hour,
	},
	"BtcDominanceAbove60": {
		ID:             "BtcDominanceAbove60",
		Tag:            "BTC-DOMINANCE-ABOVE-60",
		Name:           "Dominancia de BTC mayor a 60%",
		Fn:             BtcDominanceAbove60Callback,
		Description:    "La dominancia de Bitcoin en CoinMarketCap es mayor a 60%",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		MaxDataAge:     24 * time.Hour,
	},
	"StablecoinDominanceAbove12": {
		ID:             "StablecoinDominanceAbove12",
		Tag:            "STABLECOIN-DOMINANCE-ABOVE-12",
		Name:           "Dominancia de stablecoins mayor a 12%",
		Fn:             StablecoinDominanceAbove12Callback,
		Description:    "El market cap de las stablecoins en CoinMarketCap es mayor al 12% del market cap total",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		MaxDataAge:     24 * time.Hour,
	},
	"FearAndGreedAbove": {
		ID:             "FearAndGreedAbove",
		Tag:            "FEAR-AND-GREED-ABOVE",
		Name:           "Fear and Greed mayor al umbral",
		Fn:             FearAndGreedAboveCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap es mayor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
		MaxDataAge:     2 * 24 * time.Hour,
		Parameters:     fearAndGreedThresholdParameters,
	},
	"FearAndGreedBelow": {
		ID:             "FearAndGreedBelow",
		Tag:            "FEAR-AND-GREED-BELOW",
		Name:           "Fear and Greed menor al umbral",
		Fn:             FearAndGreedBelowCallback,
		Description:    "El indice Fear and Greed de CoinMarketCap es menor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapFearGreed,
		MaxDataAge:     2 * 24 * time.Hour,
		Parameters:     fearAndGreedThresholdParameters,
	},
	"BtcDominanceAbove": {
		ID:             "BtcDominanceAbove",
		Tag:            "BTC-DOMINANCE-ABOVE",
		Name:           "Dominancia de BTC mayor al umbral",
		Fn:             BtcDominanceAboveCallback,
		Description:    "La dominancia de Bitcoin (BTC.D) en CoinMarketCap es mayor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		MaxDataAge:     24 * time.Hour,
		Parameters:     dominanceThresholdParameters,
	},
	"BtcDominanceBelow": {
		ID:             "BtcDominanceBelow",
		Tag:            "BTC-DOMINANCE-BELOW",
		Name:           "Dominancia de BTC menor al umbral",
		Fn:             BtcDominanceBelowCallback,
		Description:    "La dominancia de Bitcoin (BTC.D) en CoinMarketCap es menor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		MaxDataAge:     24 * time.Hour,
		Parameters:     dominanceThresholdParameters,
	},
	"Others2DominanceAbove": {
		ID:             "Others2DominanceAbove",
		Tag:            "OTHERS2-DOMINANCE-ABOVE",
		Name:           "Dominancia de OTHERS2 mayor al umbral",
		Fn:             Others2DominanceAboveCallback,
		Description:    "La dominancia del mercado sin BTC (100 - BTC.D) en CoinMarketCap es mayor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		MaxDataAge:     24 * time.Hour,
		Parameters:     dominanceThresholdParameters,
	},
	"Others2DominanceBelow": {
		ID:             "Others2DominanceBelow",
		Tag:            "OTHERS2-DOMINANCE-BELOW",
		Name:           "Dominancia de OTHERS2 menor al umbral",
		Fn:             Others2DominanceBelowCallback,
		Description:    "La dominancia del mercado sin BTC (100 - BTC.D) en CoinMarketCap es menor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		MaxDataAge:     24 * time.Hour,
		Parameters:     dominanceThresholdParameters,
	},
	"Others3DominanceAbove": {
		ID:             "Others3DominanceAbove",
		Tag:            "OTHERS3-DOMINANCE-ABOVE",
		Name:           "Dominancia de OTHERS3 mayor al umbral",
		Fn:             Others3DominanceAboveCallback,
		Description:    "La dominancia del mercado sin BTC ni ETH (100 - BTC.D - ETH.D) en CoinMarketCap es mayor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		MaxDataAge:     24 * time.Hour,
		Parameters:     dominanceThresholdParameters,
	},
	"Others3DominanceBelow": {
		ID:             "Others3DominanceBelow",
		Tag:            "OTHERS3-DOMINANCE-BELOW",
		Name:           "Dominancia de OTHERS3 menor al umbral",
		Fn:             Others3DominanceBelowCallback,
		Description:    "La dominancia del mercado sin BTC ni ETH (100 - BTC.D - ETH.D) en CoinMarketCap es menor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapGlobal,
		MaxDataAge:     24 * time.Hour,
		Parameters:     dominanceThresholdParameters,
	},
	"UsdtDominanceAbove": {
		ID:             "UsdtDominanceAbove",
		Tag:            "USDT-DOMINANCE-ABOVE",
		Name:           "Dominancia de USDT mayor al umbral",
		Fn:             UsdtDominanceAboveCallback,
		Description:    "La dominancia de Tether (USDT.D) en CoinMarketCap es mayor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapUsdtQuote,
		MaxDataAge:     24 * time.Hour,
		Parameters:     dominanceThresholdParameters,
	},
	"UsdtDominanceBelow": {
		ID:             "UsdtDominanceBelow",
		Tag:            "USDT-DOMINANCE-BELOW",
		Name:           "Dominancia de USDT menor al umbral",
		Fn:             UsdtDominanceBelowCallback,
		Description:    "La dominancia de Tether (USDT.D) en CoinMarketCap es menor al umbral de la condicion",
		ResponseFormat: cmm.ResponseFormatCoinMarketCapUsdtQuote,
		MaxDataAge:     24 * time.Hour,
		Parameters:     dominanceThresholdParameters,
	},
	"EconomicCalendarHighImpactUpcoming": {
		ID:             "EconomicCalendarHighImpactUpcoming",
		Tag:            "ECONOMIC-CALENDAR-HIGH-IMPACT-UPCOMING",
		Name:           "Publicacion macro de alto impacto proxima",
		Fn:             EconomicCalendarHighImpactUpcomingCallback,
		Description:    "Hay al menos una publicacion macroeconomica de importancia alta sin dato real en las proximas horas (volatilidad alta esperada)",
		ResponseFormat: cmm.ResponseFormatBiquoteEconomicCalendar,
		Parameters:     economicCalendarUpcomingParameters,
	},
	"EconomicCalendarHighImpactCluster": {
		ID:             "EconomicCalendarHighImpactCluster",
		Tag:            "ECONOMIC-CALENDAR-HIGH-IMPACT-CLUSTER",
		Name:           "Acumulacion de publicaciones macro de alto impacto",
		Fn:             EconomicCalendarHighImpactClusterCallback,
		Description:    "Hay al menos minReleases publicaciones macroeconomicas de importancia alta (distintas por pais y hora) en las proximas horas",
		ResponseFormat: cmm.ResponseFormatBiquoteEconomicCalendar,
		Parameters:     economicCalendarClusterParameters,
	},
	"EconomicCalendarHighImpactSurprise": {
		ID:             "EconomicCalendarHighImpactSurprise",
		Tag:            "ECONOMIC-CALENDAR-HIGH-IMPACT-SURPRISE",
		Name:           "Sorpresa en publicacion macro de alto impacto",
		Fn:             EconomicCalendarHighImpactSurpriseCallback,
		Description:    "En las ultimas horas salio una publicacion de importancia alta cuyo dato real se aleja del pronostico al menos el porcentaje de la condicion",
		ResponseFormat: cmm.ResponseFormatBiquoteEconomicCalendar,
		Parameters:     economicCalendarSurpriseParameters,
	},

	// Reglas genericas sobre series (series_rules.go) combinadas con el extractor de cada proveedor
	"FredSeriesAboveWithHysteresis": {
		ID:             "FredSeriesAboveWithHysteresis",
		Tag:            "FRED-SERIES-ABOVE-HYSTERESIS",
		Name:           "Serie de FRED: mayor al nivel con histeresis",
		Fn:             FredSeriesAboveWithHysteresisCallback,
		Description:    "Se cumple cuando la serie de FRED supera onLevel y deja de cumplirse solo cuando baja de offLevel; opcionalmente durante points observaciones seguidas. Value es el ultimo valor",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
		MaxDataAge:     200 * 24 * time.Hour,
		Parameters:     seriesHysteresisParameters,
	},
	"FredSeriesBelow": {
		ID:             "FredSeriesBelow",
		Tag:            "FRED-SERIES-BELOW",
		Name:           "Serie de FRED: menor al umbral",
		Fn:             FredSeriesBelowCallback,
		Description:    "El ultimo valor de la serie de FRED es menor a threshold; opcionalmente durante points observaciones seguidas. Value es el ultimo valor",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
		MaxDataAge:     200 * 24 * time.Hour,
		Parameters:     seriesBelowParameters,
	},
	"FredSeriesCrossAbove": {
		ID:             "FredSeriesCrossAbove",
		Tag:            "FRED-SERIES-CROSS-ABOVE",
		Name:           "Serie de FRED: cruce hacia arriba",
		Fn:             FredSeriesCrossAboveCallback,
		Description:    "La ultima observacion de la serie de FRED cruzo level hacia arriba (la penultima estaba en el nivel o debajo). Value es el ultimo valor",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
		MaxDataAge:     200 * 24 * time.Hour,
		Parameters:     seriesCrossParameters,
	},
	"FredSeriesCrossBelow": {
		ID:             "FredSeriesCrossBelow",
		Tag:            "FRED-SERIES-CROSS-BELOW",
		Name:           "Serie de FRED: cruce hacia abajo",
		Fn:             FredSeriesCrossBelowCallback,
		Description:    "La ultima observacion de la serie de FRED cruzo level hacia abajo (la penultima estaba en el nivel o arriba). Value es el ultimo valor",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
		MaxDataAge:     200 * 24 * time.Hour,
		Parameters:     seriesCrossParameters,
	},
	"FredSeriesSlopeAbove": {
		ID:             "FredSeriesSlopeAbove",
		Tag:            "FRED-SERIES-SLOPE-ABOVE",
		Name:           "Serie de FRED: pendiente mayor al minimo",
		Fn:             FredSeriesSlopeAboveCallback,
		Description:    "La pendiente de la regresion lineal de la serie de FRED (por dia) es mayor a minSlope con un R² de al menos minR2; opcionalmente durante points observaciones seguidas. Value es la pendiente",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
		MaxDataAge:     200 * 24 * time.Hour,
		Parameters:     seriesSlopeParameters,
	},
	"FredSeriesPercentileAbove": {
		ID:             "FredSeriesPercentileAbove",
		Tag:            "FRED-SERIES-PERCENTILE-ABOVE",
		Name:           "Serie de FRED: percentil mayor al umbral",
		Fn:             FredSeriesPercentileAboveCallback,
		Description:    "El percentil del ultimo valor de la serie de FRED respecto a los anteriores es mayor a percentile; opcionalmente durante points observaciones seguidas. Value es el percentil",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
		MaxDataAge:     200 * 24 * time.Hour,
		Parameters:     seriesPercentileParameters,
	},
	"FredSeriesPercentileBelow": {
		ID:             "FredSeriesPercentileBelow",
		Tag:            "FRED-SERIES-PERCENTILE-BELOW",
		Name:           "Serie de FRED: percentil menor al umbral",
		Fn:             FredSeriesPercentileBelowCallback,
		Description:    "El percentil del ultimo valor de la serie de FRED respecto a los anteriores es menor a percentile; opcionalmente durante points observaciones seguidas. Value es el percentil",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
		MaxDataAge:     200 * 24 * time.Hour,
		Parameters:     seriesPercentileParameters,
	},
	"FredSeriesZScoreAbove": {
		ID:             "FredSeriesZScoreAbove",
		Tag:            "FRED-SERIES-ZSCORE-ABOVE",
		Name:           "Serie de FRED: z-score mayor al umbral",
		Fn:             FredSeriesZScoreAboveCallback,
		Description:    "El z-score del ultimo valor de la serie de FRED respecto a la ventana de window observaciones anteriores es mayor a zScore; opcionalmente durante points observaciones seguidas. Value es el z-score",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
		MaxDataAge:     200 * 24 * time.Hour,
		Parameters:     seriesZScoreParameters,
	},
	"FredSeriesZScoreBelow": {
		ID:             "FredSeriesZScoreBelow",
		Tag:            "FRED-SERIES-ZSCORE-BELOW",
		Name:           "Serie de FRED: z-score menor al umbral",
		Fn:             FredSeriesZScoreBelowCallback,
		Description:    "El z-score del ultimo valor de la serie de FRED respecto a la ventana de window observaciones anteriores es menor a zScore; opcionalmente durante points observaciones seguidas. Value es el z-score",
		ResponseFormat: cmm.ResponseFormatFredSeriesObservations,
		MaxDataAge:     200 * 24 * time.Hour,
		Parameters:     seriesZScoreParameters,
	},
	"GoogleTrendsAboveWithHysteresis": {
		ID:             "GoogleTrendsAboveWithHysteresis",
		Tag:            "GOOGLE-TRENDS-ABOVE-HYSTERESIS",
		Name:           "Google Trends: mayor al nivel con histeresis",
		Fn:             GoogleTrendsAboveWithHysteresisCallback,
		Description:    "Se cumple cuando la serie de Google Trends supera onLevel y deja de cumplirse solo cuando baja de offLevel; opcionalmente durante points observaciones seguidas. Value es el ultimo valor",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
		MaxDataAge:     14 * 24 * time.Hour,
		Parameters:     seriesHysteresisParameters,
	},
	"GoogleTrendsBelow": {
		ID:             "GoogleTrendsBelow",
		Tag:            "GOOGLE-TRENDS-BELOW",
		Name:           "Google Trends: menor al umbral",
		Fn:             GoogleTrendsBelowCallback,
		Description:    "El ultimo valor de la serie de Google Trends es menor a threshold; opcionalmente durante points observaciones seguidas. Value es el ultimo valor",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
		MaxDataAge:     14 * 24 * time.Hour,
		Parameters:     seriesBelowParameters,
	},
	"GoogleTrendsCrossAbove": {
		ID:             "GoogleTrendsCrossAbove",
		Tag:            "GOOGLE-TRENDS-CROSS-ABOVE",
		Name:           "Google Trends: cruce hacia arriba",
		Fn:             GoogleTrendsCrossAboveCallback,
		Description:    "La ultima observacion de la serie de Google Trends cruzo level hacia arriba (la penultima estaba en el nivel o debajo). Value es el ultimo valor",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
		MaxDataAge:     14 * 24 * time.Hour,
		Parameters:     seriesCrossParameters,
	},
	"GoogleTrendsCrossBelow": {
		ID:             "GoogleTrendsCrossBelow",
		Tag:            "GOOGLE-TRENDS-CROSS-BELOW",
		Name:           "Google Trends: cruce hacia abajo",
		Fn:             GoogleTrendsCrossBelowCallback,
		Description:    "La ultima observacion de la serie de Google Trends cruzo level hacia abajo (la penultima estaba en el nivel o arriba). Value es el ultimo valor",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
		MaxDataAge:     14 * 24 * time.Hour,
		Parameters:     seriesCrossParameters,
	},
	"GoogleTrendsSlopeAbove": {
		ID:             "GoogleTrendsSlopeAbove",
		Tag:            "GOOGLE-TRENDS-SLOPE-ABOVE",
		Name:           "Google Trends: pendiente mayor al minimo",
		Fn:             GoogleTrendsSlopeAboveCallback,
		Description:    "La pendiente de la regresion lineal de la serie de Google Trends (por dia) es mayor a minSlope con un R² de al menos minR2; opcionalmente durante points observaciones seguidas. Value es la pendiente",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
		MaxDataAge:     14 * 24 * time.Hour,
		Parameters:     seriesSlopeParameters,
	},
	"GoogleTrendsPercentileAbove": {
		ID:             "GoogleTrendsPercentileAbove",
		Tag:            "GOOGLE-TRENDS-PERCENTILE-ABOVE",
		Name:           "Google Trends: percentil mayor al umbral",
		Fn:             GoogleTrendsPercentileAboveCallback,
		Description:    "El percentil del ultimo valor de la serie de Google Trends respecto a los anteriores es mayor a percentile; opcionalmente durante points observaciones seguidas. Value es el percentil",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
		MaxDataAge:     14 * 24 * time.Hour,
		Parameters:     seriesPercentileParameters,
	},
	"GoogleTrendsPercentileBelow": {
		ID:             "GoogleTrendsPercentileBelow",
		Tag:            "GOOGLE-TRENDS-PERCENTILE-BELOW",
		Name:           "Google Trends: percentil menor al umbral",
		Fn:             GoogleTrendsPercentileBelowCallback,
		Description:    "El percentil del ultimo valor de la serie de Google Trends respecto a los anteriores es menor a percentile; opcionalmente durante points observaciones seguidas. Value es el percentil",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
		MaxDataAge:     14 * 24 * time.Hour,
		Parameters:     seriesPercentileParameters,
	},
	"GoogleTrendsZScoreAbove": {
		ID:             "GoogleTrendsZScoreAbove",
		Tag:            "GOOGLE-TRENDS-ZSCORE-ABOVE",
		Name:           "Google Trends: z-score mayor al umbral",
		Fn:             GoogleTrendsZScoreAboveCallback,
		Description:    "El z-score del ultimo valor de la serie de Google Trends respecto a la ventana de window observaciones anteriores es mayor a zScore; opcionalmente durante points observaciones seguidas. Value es el z-score",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
		MaxDataAge:     14 * 24 * time.Hour,
		Parameters:     seriesZScoreParameters,
	},
	"GoogleTrendsZScoreBelow": {
		ID:             "GoogleTrendsZScoreBelow",
		Tag:            "GOOGLE-TRENDS-ZSCORE-BELOW",
		Name:           "Google Trends: z-score menor al umbral",
		Fn:             GoogleTrendsZScoreBelowCallback,
		Description:    "El z-score del ultimo valor de la serie de Google Trends respecto a la ventana de window observaciones anteriores es menor a zScore; opcionalmente durante points observaciones seguidas. Value es el z-score",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
		MaxDataAge:     14 * 24 * time.Hour,
		Parameters:     seriesZScoreParameters,
	},
	"GoogleTrendsFileAboveWithHysteresis": {
		ID:             "GoogleTrendsFileAboveWithHysteresis",
		Tag:            "GOOGLE-TRENDS-FILE-ABOVE-HYSTERESIS",
		Name:           "CSV de Google Trends: mayor al nivel con histeresis",
		Fn:             GoogleTrendsFileAboveWithHysteresisCallback,
		Description:    "Se cumple cuando el CSV de Google Trends supera onLevel y deja de cumplirse solo cuando baja de offLevel; opcionalmente durante points observaciones seguidas. Value es el ultimo valor",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
		Parameters:     seriesHysteresisParameters,
	},
	"GoogleTrendsFileBelow": {
		ID:             "GoogleTrendsFileBelow",
		Tag:            "GOOGLE-TRENDS-FILE-BELOW",
		Name:           "CSV de Google Trends: menor al umbral",
		Fn:             GoogleTrendsFileBelowCallback,
		Description:    "El ultimo valor de el CSV de Google Trends es menor a threshold; opcionalmente durante points observaciones seguidas. Value es el ultimo valor",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
		Parameters:     seriesBelowParameters,
	},
	"GoogleTrendsFileCrossAbove": {
		ID:             "GoogleTrendsFileCrossAbove",
		Tag:            "GOOGLE-TRENDS-FILE-CROSS-ABOVE",
		Name:           "CSV de Google Trends: cruce hacia arriba",
		Fn:             GoogleTrendsFileCrossAboveCallback,
		Description:    "La ultima observacion de el CSV de Google Trends cruzo level hacia arriba (la penultima estaba en el nivel o debajo). Value es el ultimo valor",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
		Parameters:     seriesCrossParameters,
	},
	"GoogleTrendsFileCrossBelow": {
		ID:             "GoogleTrendsFileCrossBelow",
		Tag:            "GOOGLE-TRENDS-FILE-CROSS-BELOW",
		Name:           "CSV de Google Trends: cruce hacia abajo",
		Fn:             GoogleTrendsFileCrossBelowCallback,
		Description:    "La ultima observacion de el CSV de Google Trends cruzo level hacia abajo (la penultima estaba en el nivel o arriba). Value es el ultimo valor",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
		Parameters:     seriesCrossParameters,
	},
	"GoogleTrendsFileSlopeAbove": {
		ID:             "GoogleTrendsFileSlopeAbove",
		Tag:            "GOOGLE-TRENDS-FILE-SLOPE-ABOVE",
		Name:           "CSV de Google Trends: pendiente mayor al minimo",
		Fn:             GoogleTrendsFileSlopeAboveCallback,
		Description:    "La pendiente de la regresion lineal de el CSV de Google Trends (por dia) es mayor a minSlope con un R² de al menos minR2; opcionalmente durante points observaciones seguidas. Value es la pendiente",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
		Parameters:     seriesSlopeParameters,
	},
	"GoogleTrendsFilePercentileAbove": {
		ID:             "GoogleTrendsFilePercentileAbove",
		Tag:            "GOOGLE-TRENDS-FILE-PERCENTILE-ABOVE",
		Name:           "CSV de Google Trends: percentil mayor al umbral",
		Fn:             GoogleTrendsFilePercentileAboveCallback,
		Description:    "El percentil del ultimo valor de el CSV de Google Trends respecto a los anteriores es mayor a percentile; opcionalmente durante points observaciones seguidas. Value es el percentil",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
		Parameters:     seriesPercentileParameters,
	},
	"GoogleTrendsFilePercentileBelow": {
		ID:             "GoogleTrendsFilePercentileBelow",
		Tag:            "GOOGLE-TRENDS-FILE-PERCENTILE-BELOW",
		Name:           "CSV de Google Trends: percentil menor al umbral",
		Fn:             GoogleTrendsFilePercentileBelowCallback,
		Description:    "El percentil del ultimo valor de el CSV de Google Trends respecto a los anteriores es menor a percentile; opcionalmente durante points observaciones seguidas. Value es el percentil",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
		Parameters:     seriesPercentileParameters,
	},
	"GoogleTrendsFileZScoreAbove": {
		ID:             "GoogleTrendsFileZScoreAbove",
		Tag:            "GOOGLE-TRENDS-FILE-ZSCORE-ABOVE",
		Name:           "CSV de Google Trends: z-score mayor al umbral",
		Fn:             GoogleTrendsFileZScoreAboveCallback,
		Description:    "El z-score del ultimo valor de el CSV de Google Trends respecto a la ventana de window observaciones anteriores es mayor a zScore; opcionalmente durante points observaciones seguidas. Value es el z-score",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
		Parameters:     seriesZScoreParameters,
	},
	"GoogleTrendsFileZScoreBelow": {
		ID:             "GoogleTrendsFileZScoreBelow",
		Tag:            "GOOGLE-TRENDS-FILE-ZSCORE-BELOW",
		Name:           "CSV de Google Trends: z-score menor al umbral",
		Fn:             GoogleTrendsFileZScoreBelowCallback,
		Description:    "El z-score del ultimo valor de el CSV de Google Trends respecto a la ventana de window observaciones anteriores es menor a zScore; opcionalmente durante points observaciones seguidas. Value es el z-score",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
		MaxDataAge:     35 * 24 * time.Hour,
		Parameters:     seriesZScoreParameters,
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
