package providers

import (
	cmm "c_trd/common"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Umbrales relativos al rango (min-max) de la serie para considerar que la tendencia esta en maximos o minimos
const (
	googleTrendsHighThreshold = 0.9
	googleTrendsLowThreshold  = 0.1
	googleTrendsMinPoints     = 2
)

// googleTrendsResponse estructura minima de la respuesta del engine google_trends de SerpApi (data_type=TIMESERIES)
type googleTrendsResponse struct {
	InterestOverTime struct {
		TimelineData []struct {
			Date string `json:"date"`
			// Inicio del periodo en unix (segundos) como string, ej: "1694908800"
			Timestamp   string `json:"timestamp"`
			PartialData bool   `json:"partial_data"`
			Values      []struct {
				Query string `json:"query"`
				// Algunos proveedores lo devuelven como numero y otros como string
				ExtractedValue json.RawMessage `json:"extracted_value"`
			} `json:"values"`
		} `json:"timeline_data"`
	} `json:"interest_over_time"`

	// SerpApi responde con este campo cuando hay un error o limite de peticiones
	Error string `json:"error"`
}

// GoogleTrendsHighCallback retorna true si el ultimo valor de interes esta en la zona alta del rango de la serie
func GoogleTrendsHighCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, asOf, err := parseGoogleTrendsSeries(responseData)
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

// GoogleTrendsLowCallback retorna true si el ultimo valor de interes esta en la zona baja del rango de la serie
func GoogleTrendsLowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, asOf, err := parseGoogleTrendsSeries(responseData)
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

// parseGoogleTrendsSeries extrae la serie del primer query con los puntos completos y la fecha del ultimo punto completo
// (nil si no trae timestamp). Los puntos con partial_data se ignoran porque el periodo aun no termina y subestiman el interes
func parseGoogleTrendsSeries(responseData []byte) ([]float64, *time.Time, *cmm.ErrorHandler) {
	var data googleTrendsResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return nil, nil, cmm.NewErrorHandler(err, "Error parsing Google Trends response data", cmm.LevelFatal, "PCGTRE001")
	}

	if data.Error != "" {
		return nil, nil, cmm.NewErrorHandler(errors.New(data.Error), "Google Trends provider returned an error message", cmm.LevelError, "PCGTRE002")
	}

	series := []float64{}
	lastTimestamp := ""
	for _, point := range data.InterestOverTime.TimelineData {
		if point.PartialData || len(point.Values) == 0 {
			continue
		}

		value, err := parseTrendValue(point.Values[0].ExtractedValue, point.Date)
		if !err.IsEmtpy() {
			return nil, nil, err
		}
		series = append(series, value)
		lastTimestamp = point.Timestamp
	}

	if len(series) < googleTrendsMinPoints {
		return nil, nil, cmm.NewErrorHandler(fmt.Errorf("got %d points", len(series)), "Google Trends series has not enough complete points", cmm.LevelError, "PCGTRE004")
	}

	asOf := unixTimestampTime(lastTimestamp)
	return series, asOf, cmm.NewEmptyErrorHandler()
}

// ExtractGoogleTrendsObservations extractor del engine google_trends de SerpApi: retorna los puntos completos del
// primer query con la fecha de inicio de su periodo. Los puntos con partial_data se ignoran porque el periodo aun no
// termina y subestiman el interes. No exige un minimo de puntos, eso lo decide cada regla
func ExtractGoogleTrendsObservations(responseData []byte) ([]Observation, *cmm.ErrorHandler) {
	var data googleTrendsResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return nil, cmm.NewErrorHandler(err, "Error parsing Google Trends response data", cmm.LevelFatal, "PCGTRE006")
	}

	if data.Error != "" {
		return nil, cmm.NewErrorHandler(errors.New(data.Error), "Google Trends provider returned an error message", cmm.LevelError, "PCGTRE007")
	}

	observations := []Observation{}
	for _, point := range data.InterestOverTime.TimelineData {
		if point.PartialData || len(point.Values) == 0 {
			continue
		}

		value, err := parseTrendValue(point.Values[0].ExtractedValue, point.Date)
		if !err.IsEmtpy() {
			return nil, err
		}

		at := unixTimestampTime(point.Timestamp)
		if at == nil {
			return nil, cmm.NewErrorHandler(fmt.Errorf("date %q timestamp %q", point.Date, point.Timestamp), "Google Trends point has no valid timestamp", cmm.LevelError, "PCGTRE008")
		}

		observations = append(observations, Observation{At: *at, Value: value})
	}

	return observations, cmm.NewEmptyErrorHandler()
}

// unixTimestampTime convierte un timestamp unix en segundos (string) a fecha UTC. nil si viene vacio o no es un numero
func unixTimestampTime(text string) *time.Time {
	seconds, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return nil
	}

	asOf := time.Unix(seconds, 0).UTC()
	return &asOf
}

// parseTrendValue acepta extracted_value como numero o como string numerico
func parseTrendValue(raw json.RawMessage, date string) (float64, *cmm.ErrorHandler) {
	var number float64
	if err := json.Unmarshal(raw, &number); err == nil {
		return number, cmm.NewEmptyErrorHandler()
	}

	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0, cmm.NewErrorHandler(fmt.Errorf("date %q: %w", date, err), "Google Trends extracted_value is not a number", cmm.LevelFatal, "PCGTRE003")
	}

	number, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, cmm.NewErrorHandler(fmt.Errorf("date %q: %w", date, err), "Error parsing Google Trends value", cmm.LevelFatal, "PCGTRE005")
	}

	return number, cmm.NewEmptyErrorHandler()
}

// Callbacks de reglas sobre series (extractor + regla generica de series_rules.go)

// GoogleTrendsAboveWithHysteresisCallback ExtractGoogleTrendsObservations + regla AboveWithHysteresis. Se cumple cuando la serie de Google Trends supera onLevel y deja de cumplirse solo cuando baja de offLevel; opcionalmente durante points observaciones seguidas. Value es el ultimo valor
func GoogleTrendsAboveWithHysteresisCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsObservations(responseData)
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

// GoogleTrendsBelowCallback ExtractGoogleTrendsObservations + regla Below. El ultimo valor de la serie de Google Trends es menor a threshold; opcionalmente durante points observaciones seguidas. Value es el ultimo valor
func GoogleTrendsBelowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsObservations(responseData)
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

// GoogleTrendsCrossAboveCallback ExtractGoogleTrendsObservations + regla CrossAbove. La ultima observacion de la serie de Google Trends cruzo level hacia arriba (la penultima estaba en el nivel o debajo). Value es el ultimo valor
func GoogleTrendsCrossAboveCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsObservations(responseData)
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

// GoogleTrendsCrossBelowCallback ExtractGoogleTrendsObservations + regla CrossBelow. La ultima observacion de la serie de Google Trends cruzo level hacia abajo (la penultima estaba en el nivel o arriba). Value es el ultimo valor
func GoogleTrendsCrossBelowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsObservations(responseData)
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

// GoogleTrendsSlopeAboveCallback ExtractGoogleTrendsObservations + regla SlopeAbove. La pendiente de la regresion lineal de la serie de Google Trends (por dia) es mayor a minSlope con un R² de al menos minR2; opcionalmente durante points observaciones seguidas. Value es la pendiente
func GoogleTrendsSlopeAboveCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsObservations(responseData)
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

// GoogleTrendsPercentileAboveCallback ExtractGoogleTrendsObservations + regla PercentileAbove. El percentil del ultimo valor de la serie de Google Trends respecto a los anteriores es mayor a percentile; opcionalmente durante points observaciones seguidas. Value es el percentil
func GoogleTrendsPercentileAboveCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsObservations(responseData)
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

// GoogleTrendsPercentileBelowCallback ExtractGoogleTrendsObservations + regla PercentileBelow. El percentil del ultimo valor de la serie de Google Trends respecto a los anteriores es menor a percentile; opcionalmente durante points observaciones seguidas. Value es el percentil
func GoogleTrendsPercentileBelowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsObservations(responseData)
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

// GoogleTrendsZScoreAboveCallback ExtractGoogleTrendsObservations + regla ZScoreAbove. El z-score del ultimo valor de la serie de Google Trends respecto a la ventana de window observaciones anteriores es mayor a zScore; opcionalmente durante points observaciones seguidas. Value es el z-score
func GoogleTrendsZScoreAboveCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsObservations(responseData)
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

// GoogleTrendsZScoreBelowCallback ExtractGoogleTrendsObservations + regla ZScoreBelow. El z-score del ultimo valor de la serie de Google Trends respecto a la ventana de window observaciones anteriores es menor a zScore; opcionalmente durante points observaciones seguidas. Value es el z-score
func GoogleTrendsZScoreBelowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractGoogleTrendsObservations(responseData)
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
