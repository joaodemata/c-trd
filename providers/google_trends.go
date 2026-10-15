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
func GoogleTrendsHighCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	series, err := parseGoogleTrendsSeries(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	return isTrendInHighZone(series), cmm.NewEmptyErrorHandler()
}

// GoogleTrendsLowCallback retorna true si el ultimo valor de interes esta en la zona baja del rango de la serie
func GoogleTrendsLowCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	series, err := parseGoogleTrendsSeries(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	return isTrendInLowZone(series), cmm.NewEmptyErrorHandler()
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

// Las siguientes funciones reciben una serie con al menos googleTrendsMinPoints puntos,
// los valores solo son comparables dentro de la misma serie porque Google los reescala en cada consulta

// trendRange retorna el minimo y el maximo de la serie
func trendRange(series []float64) (float64, float64) {
	low, high := series[0], series[0]
	for _, value := range series {
		low = min(low, value)
		high = max(high, value)
	}
	return low, high
}

// isTrendInHighZone el ultimo valor esta en el 10% superior del rango. Una serie plana no tiene zona alta
func isTrendInHighZone(series []float64) bool {
	low, high := trendRange(series)
	if high == low {
		return false
	}
	return series[len(series)-1] >= low+(high-low)*googleTrendsHighThreshold
}

// isTrendInLowZone el ultimo valor esta en el 10% inferior del rango. Una serie plana no tiene zona baja
func isTrendInLowZone(series []float64) bool {
	low, high := trendRange(series)
	if high == low {
		return false
	}
	return series[len(series)-1] <= low+(high-low)*googleTrendsLowThreshold
}

// isTrendNewMaximum el ultimo valor supera a todos los anteriores
func isTrendNewMaximum(series []float64) bool {
	_, previousHigh := trendRange(series[:len(series)-1])
	return series[len(series)-1] > previousHigh
}

// isTrendNewMinimum el ultimo valor es menor que todos los anteriores
func isTrendNewMinimum(series []float64) bool {
	previousLow, _ := trendRange(series[:len(series)-1])
	return series[len(series)-1] < previousLow
}
