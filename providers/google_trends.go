package providers

import (
	cmm "c_trd/common"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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
			Date        string `json:"date"`
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
func GoogleTrendsHighCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	series, err := parseGoogleTrendsSeries(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	// Los valores solo son comparables dentro de la misma serie porque Google los reescala en cada consulta.
	// Una serie plana no tiene zona alta
	low, high := seriesRange(series)
	highZone := rangeLevel(low, high, googleTrendsHighThreshold)
	lastValue := series[len(series)-1]
	isFlat := low == high

	isHigh := !isFlat && lastValue >= highZone

	return isHigh, cmm.NewEmptyErrorHandler()
}

// GoogleTrendsLowCallback retorna true si el ultimo valor de interes esta en la zona baja del rango de la serie
func GoogleTrendsLowCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	series, err := parseGoogleTrendsSeries(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	// Los valores solo son comparables dentro de la misma serie porque Google los reescala en cada consulta.
	// Una serie plana no tiene zona baja
	low, high := seriesRange(series)
	lowZone := rangeLevel(low, high, googleTrendsLowThreshold)
	lastValue := series[len(series)-1]
	isFlat := low == high

	isLow := !isFlat && lastValue <= lowZone

	return isLow, cmm.NewEmptyErrorHandler()
}

// parseGoogleTrendsSeries extrae la serie del primer query con los puntos completos.
// Los puntos con partial_data se ignoran porque el periodo aun no termina y subestiman el interes
func parseGoogleTrendsSeries(responseData []byte) ([]float64, *cmm.ErrorHandler) {
	var data googleTrendsResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return nil, cmm.NewErrorHandler(err, "Error parsing Google Trends response data", cmm.LevelFatal, "PCGTRE001")
	}

	if data.Error != "" {
		return nil, cmm.NewErrorHandler(errors.New(data.Error), "Google Trends provider returned an error message", cmm.LevelError, "PCGTRE002")
	}

	series := []float64{}
	for _, point := range data.InterestOverTime.TimelineData {
		if point.PartialData || len(point.Values) == 0 {
			continue
		}

		value, err := parseTrendValue(point.Values[0].ExtractedValue, point.Date)
		if !err.IsEmtpy() {
			return nil, err
		}
		series = append(series, value)
	}

	if len(series) < googleTrendsMinPoints {
		return nil, cmm.NewErrorHandler(fmt.Errorf("got %d points", len(series)), "Google Trends series has not enough complete points", cmm.LevelError, "PCGTRE004")
	}

	return series, cmm.NewEmptyErrorHandler()
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
