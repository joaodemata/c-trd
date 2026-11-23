package providers

import (
	cmm "c_trd/common"
	"encoding/json"
	"fmt"
	"time"
)

// Margen extra sobre el periodo de la serie antes de considerar que el archivo esta desactualizado
const googleTrendsFileStaleMargin = 24 * time.Hour

// GoogleTrendsFilePoint punto de la serie de un CSV de Google Trends
type GoogleTrendsFilePoint struct {
	Date  time.Time `json:"date"`
	Value float64   `json:"value"`
}

// GoogleTrendsFileSeries serie de un CSV de Google Trends subido para un asset. Solo contiene periodos completos
type GoogleTrendsFileSeries struct {
	IDAsset       string                  `json:"idAsset"`
	Keyword       string                  `json:"keyword"`
	UploadedAt    time.Time               `json:"uploadedAt"`
	PeriodSeconds int64                   `json:"periodSeconds"`
	Points        []GoogleTrendsFilePoint `json:"points"`
}

// googleTrendsFileResponse respuesta de la ruta local POST /operation/google_trends/series
type googleTrendsFileResponse struct {
	Status string                 `json:"status"`
	Data   GoogleTrendsFileSeries `json:"data"`
}

// GoogleTrendsFileHighCallback el ultimo periodo del CSV esta en la zona alta del rango
func GoogleTrendsFileHighCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, asOf, err := parseGoogleTrendsFile(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	// Los valores solo son comparables dentro de la misma serie porque Google los reescala en cada consulta.
	// Una serie plana no tiene zona alta
	low, high := seriesRange(series)
	highZone := rangeLevel(low, high, googleTrendsHighThreshold)
	lastValue := series[len(series)-1]
	isFlat := low == high

	isHigh := !isFlat && lastValue >= highZone

	result := CallbackResult{Accepted: isHigh, Value: lastValue, AsOf: asOf}
	return result, cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFileLowCallback el ultimo periodo del CSV esta en la zona baja del rango
func GoogleTrendsFileLowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, asOf, err := parseGoogleTrendsFile(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	// Los valores solo son comparables dentro de la misma serie porque Google los reescala en cada consulta.
	// Una serie plana no tiene zona baja
	low, high := seriesRange(series)
	lowZone := rangeLevel(low, high, googleTrendsLowThreshold)
	lastValue := series[len(series)-1]
	isFlat := low == high

	isLow := !isFlat && lastValue <= lowZone

	result := CallbackResult{Accepted: isLow, Value: lastValue, AsOf: asOf}
	return result, cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFileNewMaxCallback el ultimo periodo del CSV es un nuevo maximo de la serie
func GoogleTrendsFileNewMaxCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, asOf, err := parseGoogleTrendsFile(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	previousValues := series[:len(series)-1]
	lastValue := series[len(series)-1]
	_, previousHigh := seriesRange(previousValues)

	isNewMax := lastValue > previousHigh

	result := CallbackResult{Accepted: isNewMax, Value: lastValue, AsOf: asOf}
	return result, cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFileNewMinCallback el ultimo periodo del CSV es un nuevo minimo de la serie
func GoogleTrendsFileNewMinCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, asOf, err := parseGoogleTrendsFile(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	previousValues := series[:len(series)-1]
	lastValue := series[len(series)-1]
	previousLow, _ := seriesRange(previousValues)

	isNewMin := lastValue < previousLow

	result := CallbackResult{Accepted: isNewMin, Value: lastValue, AsOf: asOf}
	return result, cmm.NewEmptyErrorHandler()
}

// ExtractGoogleTrendsFileObservations extractor de la ruta local /operation/google_trends/series: retorna los periodos
// completos del CSV y valida que el archivo no este desactualizado. Un archivo desactualizado retorna error para que la
// condicion conserve su ultimo status. No exige un minimo de puntos, eso lo decide cada regla
func ExtractGoogleTrendsFileObservations(responseData []byte) ([]Observation, *cmm.ErrorHandler) {
	var data googleTrendsFileResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return nil, cmm.NewErrorHandler(err, "Error parsing Google Trends file response", cmm.LevelFatal, "PCGTFE001")
	}

	// El archivo cubre hasta el siguiente periodo, despues de eso hay que subir uno nuevo
	series := data.Data
	staleAt := series.UploadedAt.Add(time.Duration(series.PeriodSeconds)*time.Second + googleTrendsFileStaleMargin)
	if time.Now().UTC().After(staleAt) {
		return nil, cmm.NewErrorHandler(fmt.Errorf("asset %s uploaded at %s", series.IDAsset, series.UploadedAt.Format(time.RFC3339)), "Google Trends file is outdated, upload a new one", cmm.LevelWarning, "PCGTFE003")
	}

	observations := make([]Observation, len(series.Points))
	for i, point := range series.Points {
		observations[i] = Observation{At: point.Date.UTC(), Value: point.Value}
	}
	return observations, cmm.NewEmptyErrorHandler()
}

// ExtractGoogleTrendsFileHistory extractor de la ruta local /operation/google_trends/series para el backtesting: igual
// que ExtractGoogleTrendsFileObservations pero sin validar que el archivo este al dia, porque el backtest evalua fechas
// pasadas. No exige un minimo de puntos, eso lo decide cada regla
func ExtractGoogleTrendsFileHistory(responseData []byte) ([]Observation, *cmm.ErrorHandler) {
	var data googleTrendsFileResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return nil, cmm.NewErrorHandler(err, "Error parsing Google Trends file response", cmm.LevelFatal, "PCGTFE004")
	}

	observations := make([]Observation, len(data.Data.Points))
	for i, point := range data.Data.Points {
		observations[i] = Observation{At: point.Date.UTC(), Value: point.Value}
	}
	return observations, cmm.NewEmptyErrorHandler()
}

// parseGoogleTrendsFile extrae los valores de la serie y la fecha del ultimo periodo
func parseGoogleTrendsFile(responseData []byte) ([]float64, *time.Time, *cmm.ErrorHandler) {
	observations, err := ExtractGoogleTrendsFileObservations(responseData)
	if !err.IsEmtpy() {
		return nil, nil, err
	}

	if len(observations) < googleTrendsMinPoints {
		return nil, nil, cmm.NewErrorHandler(fmt.Errorf("got %d points", len(observations)), "Google Trends file has not enough complete points", cmm.LevelError, "PCGTFE002")
	}

	values := observationValues(observations)
	lastDate := observations[len(observations)-1].At
	return values, &lastDate, cmm.NewEmptyErrorHandler()
}

// Callbacks de reglas sobre series (extractor + regla generica de series_rules.go)

// GoogleTrendsFileAboveWithHysteresisCallback ExtractGoogleTrendsFileObservations + regla AboveWithHysteresis. Se cumple cuando el CSV de Google Trends supera onLevel y deja de cumplirse solo cuando baja de offLevel; opcionalmente durante points observaciones seguidas. Value es el ultimo valor
func GoogleTrendsFileAboveWithHysteresisCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsFileObservations(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	asOf := time.Now().UTC()
	result, err := aboveWithHysteresisDecision(series, parameters, asOf)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFileBelowCallback ExtractGoogleTrendsFileObservations + regla Below. El ultimo valor de el CSV de Google Trends es menor a threshold; opcionalmente durante points observaciones seguidas. Value es el ultimo valor
func GoogleTrendsFileBelowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsFileObservations(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	asOf := time.Now().UTC()
	result, err := belowDecision(series, parameters, asOf)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFileCrossAboveCallback ExtractGoogleTrendsFileObservations + regla CrossAbove. La ultima observacion de el CSV de Google Trends cruzo level hacia arriba (la penultima estaba en el nivel o debajo). Value es el ultimo valor
func GoogleTrendsFileCrossAboveCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsFileObservations(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	asOf := time.Now().UTC()
	result, err := crossDecision(series, parameters, asOf, CrossUp)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFileCrossBelowCallback ExtractGoogleTrendsFileObservations + regla CrossBelow. La ultima observacion de el CSV de Google Trends cruzo level hacia abajo (la penultima estaba en el nivel o arriba). Value es el ultimo valor
func GoogleTrendsFileCrossBelowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsFileObservations(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	asOf := time.Now().UTC()
	result, err := crossDecision(series, parameters, asOf, CrossDown)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFileSlopeAboveCallback ExtractGoogleTrendsFileObservations + regla SlopeAbove. La pendiente de la regresion lineal de el CSV de Google Trends (por dia) es mayor a minSlope con un R² de al menos minR2; opcionalmente durante points observaciones seguidas. Value es la pendiente
func GoogleTrendsFileSlopeAboveCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsFileObservations(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	asOf := time.Now().UTC()
	result, err := slopeAboveDecision(series, parameters, asOf)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFilePercentileAboveCallback ExtractGoogleTrendsFileObservations + regla PercentileAbove. El percentil del ultimo valor de el CSV de Google Trends respecto a los anteriores es mayor a percentile; opcionalmente durante points observaciones seguidas. Value es el percentil
func GoogleTrendsFilePercentileAboveCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsFileObservations(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	asOf := time.Now().UTC()
	result, err := percentileAboveDecision(series, parameters, asOf)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFilePercentileBelowCallback ExtractGoogleTrendsFileObservations + regla PercentileBelow. El percentil del ultimo valor de el CSV de Google Trends respecto a los anteriores es menor a percentile; opcionalmente durante points observaciones seguidas. Value es el percentil
func GoogleTrendsFilePercentileBelowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsFileObservations(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	asOf := time.Now().UTC()
	result, err := percentileBelowDecision(series, parameters, asOf)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFileZScoreAboveCallback ExtractGoogleTrendsFileObservations + regla ZScoreAbove. El z-score del ultimo valor de el CSV de Google Trends respecto a la ventana de window observaciones anteriores es mayor a zScore; opcionalmente durante points observaciones seguidas. Value es el z-score
func GoogleTrendsFileZScoreAboveCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsFileObservations(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	asOf := time.Now().UTC()
	result, err := zScoreAboveDecision(series, parameters, asOf)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFileZScoreBelowCallback ExtractGoogleTrendsFileObservations + regla ZScoreBelow. El z-score del ultimo valor de el CSV de Google Trends respecto a la ventana de window observaciones anteriores es menor a zScore; opcionalmente durante points observaciones seguidas. Value es el z-score
func GoogleTrendsFileZScoreBelowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsFileObservations(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	asOf := time.Now().UTC()
	result, err := zScoreBelowDecision(series, parameters, asOf)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	return result, cmm.NewEmptyErrorHandler()
}
