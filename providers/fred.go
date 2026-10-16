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

// fredPoint observacion con valor de la serie
type fredPoint struct {
	Date  time.Time
	Value float64
}

// FredSeriesRisingCallback retorna true si la pendiente de la regresion lineal de la ventana es positiva.
// La ventana es la cantidad de observaciones que devuelve FRED (parametro limit de la condicion)
func FredSeriesRisingCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	points, err := parseFredSeries(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	return fredTrendSlope(points) > 0, cmm.NewEmptyErrorHandler()
}

// FredSeriesFallingCallback retorna true si la pendiente de la regresion lineal de la ventana es negativa
func FredSeriesFallingCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	points, err := parseFredSeries(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	return fredTrendSlope(points) < 0, cmm.NewEmptyErrorHandler()
}

// parseFredSeries retorna las observaciones con valor ordenadas por fecha, sin depender del sort_order de la peticion.
// Las observaciones sin valor se ignoran
func parseFredSeries(responseData []byte) ([]fredPoint, *cmm.ErrorHandler) {
	var data fredObservationsResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return nil, cmm.NewErrorHandler(err, "Error parsing FRED response data", cmm.LevelFatal, "PCFREDE001")
	}

	if data.ErrorMessage != "" {
		return nil, cmm.NewErrorHandler(fmt.Errorf("error_code %d: %w", data.ErrorCode, errors.New(data.ErrorMessage)), "FRED returned an error message", cmm.LevelError, "PCFREDE002")
	}

	points := make([]fredPoint, 0, len(data.Observations))
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

		points = append(points, fredPoint{Date: date, Value: value})
	}

	if len(points) < fredMinPoints {
		return nil, cmm.NewErrorHandler(fmt.Errorf("got %d points", len(points)), "FRED series has not enough observations with value", cmm.LevelError, "PCFREDE004")
	}

	sort.Slice(points, func(i, j int) bool { return points[i].Date.Before(points[j].Date) })
	return points, cmm.NewEmptyErrorHandler()
}

// fredTrendSlope pendiente (por dia) de la regresion lineal por minimos cuadrados de la serie. El eje x son los dias
// desde la primera observacion, asi los meses de distinta duracion o los valores faltantes no deforman la tendencia.
// Recibe al menos fredMinPoints puntos; si todas las fechas son iguales retorna 0
func fredTrendSlope(points []fredPoint) float64 {
	n := float64(len(points))
	var meanX, meanY float64
	xs := make([]float64, len(points))
	for i, point := range points {
		xs[i] = point.Date.Sub(points[0].Date).Hours() / 24
		meanX += xs[i] / n
		meanY += point.Value / n
	}

	var covariance, variance float64
	for i, point := range points {
		dx := xs[i] - meanX
		covariance += dx * (point.Value - meanY)
		variance += dx * dx
	}

	if variance == 0 {
		return 0
	}
	return covariance / variance
}
