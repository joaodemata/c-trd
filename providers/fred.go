package providers

import (
	cmm "c_trd/common"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"
)

// FRED marca con "." las observaciones sin valor (ej: feriados o periodos sin dato)
const (
	fredMissingValue = "."
	fredMinPoints    = 2
)

// fredObservationsResponse estructura minima de la respuesta de /fred/series/observations (file_type=json)
type fredObservationsResponse struct {
	Observations []struct {
		Date  string `json:"date"`
		Value string `json:"value"`
	} `json:"observations"`

	// FRED responde con estos campos cuando hay un error
	ErrorCode    int    `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

// FredSeriesRisingCallback retorna true si la pendiente de la regresion lineal de la ventana es positiva.
// La ventana es la cantidad de observaciones que devuelve FRED (parametro limit de la condicion). Value es la pendiente
// en unidades de la serie por dia y AsOf la fecha de la ultima observacion con valor
func FredSeriesRisingCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	points, asOf, err := parseFredSeries(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	slope := trendSlope(points)
	isRising := slope > 0

	result := CallbackResult{Accepted: isRising, Value: slope, AsOf: asOf}
	return result, cmm.NewEmptyErrorHandler()
}

// FredSeriesFallingCallback retorna true si la pendiente de la regresion lineal de la ventana es negativa
func FredSeriesFallingCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	points, asOf, err := parseFredSeries(responseData)
	if !err.IsEmtpy() {
		return CallbackResult{}, err
	}

	slope := trendSlope(points)
	isFalling := slope < 0

	result := CallbackResult{Accepted: isFalling, Value: slope, AsOf: asOf}
	return result, cmm.NewEmptyErrorHandler()
}

// ExtractFredObservations extractor de /fred/series/observations: retorna las observaciones con valor ordenadas por
// fecha, sin depender del sort_order de la peticion. Las observaciones sin valor (".") se ignoran. No exige un minimo
// de puntos, eso lo decide cada regla
func ExtractFredObservations(responseData []byte) ([]Observation, *cmm.ErrorHandler) {
	var data fredObservationsResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return nil, cmm.NewErrorHandler(err, "Error parsing FRED response data", cmm.LevelFatal, "PCFREDE001")
	}

	if data.ErrorMessage != "" {
		return nil, cmm.NewErrorHandler(fmt.Errorf("error_code %d: %w", data.ErrorCode, errors.New(data.ErrorMessage)), "FRED returned an error message", cmm.LevelError, "PCFREDE002")
	}

	observations := make([]Observation, 0, len(data.Observations))
	for _, observation := range data.Observations {
		if observation.Value == fredMissingValue {
			continue
		}

		value, err := strconv.ParseFloat(observation.Value, 64)
		if err != nil {
			return nil, cmm.NewErrorHandler(fmt.Errorf("date %q: %w", observation.Date, err), "Error parsing FRED observation value", cmm.LevelFatal, "PCFREDE003")
		}

		date, err := time.Parse(time.DateOnly, observation.Date)
		if err != nil {
			return nil, cmm.NewErrorHandler(err, "Error parsing FRED observation date", cmm.LevelFatal, "PCFREDE005")
		}

		observations = append(observations, Observation{At: date, Value: value})
	}

	sort.Slice(observations, func(i, j int) bool { return observations[i].At.Before(observations[j].At) })
	return observations, cmm.NewEmptyErrorHandler()
}

// parseFredSeries retorna las observaciones con valor como puntos de una serie. El eje x son los dias desde la primera
// observacion, asi los meses de distinta duracion o los valores faltantes no deforman la tendencia. Tambien retorna la
// fecha de la ultima observacion con valor
func parseFredSeries(responseData []byte) ([]seriesPoint, *time.Time, *cmm.ErrorHandler) {
	observations, err := ExtractFredObservations(responseData)
	if !err.IsEmtpy() {
		return nil, nil, err
	}

	if len(observations) < fredMinPoints {
		return nil, nil, cmm.NewErrorHandler(fmt.Errorf("got %d points", len(observations)), "FRED series has not enough observations with value", cmm.LevelError, "PCFREDE004")
	}

	series := make([]seriesPoint, len(observations))
	for i, observation := range observations {
		series[i] = seriesPoint{X: observation.At.Sub(observations[0].At).Hours() / 24, Y: observation.Value}
	}

	lastDate := observations[len(observations)-1].At
	return series, &lastDate, cmm.NewEmptyErrorHandler()
}

// Callbacks de reglas sobre series (extractor + regla generica de series_rules.go)

// FredSeriesAboveWithHysteresisCallback ExtractFredObservations + regla AboveWithHysteresis. Se cumple cuando la serie de FRED supera onLevel y deja de cumplirse solo cuando baja de offLevel; opcionalmente durante points observaciones seguidas. Value es el ultimo valor
func FredSeriesAboveWithHysteresisCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractFredObservations(responseData)
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

// FredSeriesBelowCallback ExtractFredObservations + regla Below. El ultimo valor de la serie de FRED es menor a threshold; opcionalmente durante points observaciones seguidas. Value es el ultimo valor
func FredSeriesBelowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractFredObservations(responseData)
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

// FredSeriesCrossAboveCallback ExtractFredObservations + regla CrossAbove. La ultima observacion de la serie de FRED cruzo level hacia arriba (la penultima estaba en el nivel o debajo). Value es el ultimo valor
func FredSeriesCrossAboveCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractFredObservations(responseData)
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

// FredSeriesCrossBelowCallback ExtractFredObservations + regla CrossBelow. La ultima observacion de la serie de FRED cruzo level hacia abajo (la penultima estaba en el nivel o arriba). Value es el ultimo valor
func FredSeriesCrossBelowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractFredObservations(responseData)
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

// FredSeriesSlopeAboveCallback ExtractFredObservations + regla SlopeAbove. La pendiente de la regresion lineal de la serie de FRED (por dia) es mayor a minSlope con un R² de al menos minR2; opcionalmente durante points observaciones seguidas. Value es la pendiente
func FredSeriesSlopeAboveCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractFredObservations(responseData)
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

// FredSeriesPercentileAboveCallback ExtractFredObservations + regla PercentileAbove. El percentil del ultimo valor de la serie de FRED respecto a los anteriores es mayor a percentile; opcionalmente durante points observaciones seguidas. Value es el percentil
func FredSeriesPercentileAboveCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractFredObservations(responseData)
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

// FredSeriesPercentileBelowCallback ExtractFredObservations + regla PercentileBelow. El percentil del ultimo valor de la serie de FRED respecto a los anteriores es menor a percentile; opcionalmente durante points observaciones seguidas. Value es el percentil
func FredSeriesPercentileBelowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractFredObservations(responseData)
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

// FredSeriesZScoreAboveCallback ExtractFredObservations + regla ZScoreAbove. El z-score del ultimo valor de la serie de FRED respecto a la ventana de window observaciones anteriores es mayor a zScore; opcionalmente durante points observaciones seguidas. Value es el z-score
func FredSeriesZScoreAboveCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractFredObservations(responseData)
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

// FredSeriesZScoreBelowCallback ExtractFredObservations + regla ZScoreBelow. El z-score del ultimo valor de la serie de FRED respecto a la ventana de window observaciones anteriores es menor a zScore; opcionalmente durante points observaciones seguidas. Value es el z-score
func FredSeriesZScoreBelowCallback(responseData []byte, parameters map[string]any) (CallbackResult, *cmm.ErrorHandler) {
	series, err := ExtractFredObservations(responseData)
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
