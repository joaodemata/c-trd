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
func GoogleTrendsFileHighCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	series, err := parseGoogleTrendsFile(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	return isTrendInHighZone(series), cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFileLowCallback el ultimo periodo del CSV esta en la zona baja del rango
func GoogleTrendsFileLowCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	series, err := parseGoogleTrendsFile(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	return isTrendInLowZone(series), cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFileNewMaxCallback el ultimo periodo del CSV es un nuevo maximo de la serie
func GoogleTrendsFileNewMaxCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	series, err := parseGoogleTrendsFile(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	return isTrendNewMaximum(series), cmm.NewEmptyErrorHandler()
}

// GoogleTrendsFileNewMinCallback el ultimo periodo del CSV es un nuevo minimo de la serie
func GoogleTrendsFileNewMinCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	series, err := parseGoogleTrendsFile(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	return isTrendNewMinimum(series), cmm.NewEmptyErrorHandler()
}

// parseGoogleTrendsFile extrae los valores de la serie y valida que el archivo no este desactualizado.
// Un archivo desactualizado retorna error para que la condicion conserve su ultimo status
func parseGoogleTrendsFile(responseData []byte) ([]float64, *cmm.ErrorHandler) {
	var data googleTrendsFileResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return nil, cmm.NewErrorHandler(err, "Error parsing Google Trends file response", cmm.LevelFatal, "PCGTFE001")
	}

	series := data.Data
	if len(series.Points) < googleTrendsMinPoints {
		return nil, cmm.NewErrorHandler(fmt.Errorf("got %d points", len(series.Points)), "Google Trends file has not enough complete points", cmm.LevelError, "PCGTFE002")
	}

	// El archivo cubre hasta el siguiente periodo, despues de eso hay que subir uno nuevo
	staleAt := series.UploadedAt.Add(time.Duration(series.PeriodSeconds)*time.Second + googleTrendsFileStaleMargin)
	if time.Now().UTC().After(staleAt) {
		return nil, cmm.NewErrorHandler(fmt.Errorf("asset %s uploaded at %s", series.IDAsset, series.UploadedAt.Format(time.RFC3339)), "Google Trends file is outdated, upload a new one", cmm.LevelWarning, "PCGTFE003")
	}

	values := make([]float64, len(series.Points))
	for i, point := range series.Points {
		values[i] = point.Value
	}

	return values, cmm.NewEmptyErrorHandler()
}
