package providers

import (
	cmm "c_trd/common"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// Valores del calendario economico de biquote (GET https://biquote.io/api/calendar). La API devuelve un arreglo de
// eventos del mas antiguo al mas reciente; importance es el impacto esperado de la publicacion
const (
	biquoteImportanceHigh = "high"
	biquoteTypeHoliday    = "holiday"
	// Con timeMode distinto a exact la hora es un marcador: el evento puede salir en cualquier momento del dia
	biquoteTimeModeExact = "exact"
	biquoteInexactWindow = 24 * time.Hour
)

// Nombres y rangos de los parametros de los callbacks del calendario economico
const (
	biquoteHoursParameterName           = "hours"
	biquoteMinReleasesParameterName     = "minReleases"
	biquoteSurprisePercentParameterName = "surprisePercent"

	// La API devuelve por defecto hasta 7 dias hacia adelante
	biquoteUpcomingHoursMin = 1.0
	biquoteUpcomingHoursMax = 168.0
	// La API devuelve por defecto desde ayer, por eso la sorpresa se busca como maximo 24 horas atras
	biquoteRecentHoursMin  = 1.0
	biquoteRecentHoursMax  = 24.0
	biquoteMinReleasesMin  = 2.0
	biquoteMinReleasesMax  = 50.0
	biquoteSurpriseMin     = 1.0
	biquoteSurpriseMax     = 1000.0
	biquotePercentMultiple = 100.0
)

// Rangos de los parametros al crear la condicion. Son variables porque CallbackParameter guarda punteros
var (
	biquoteUpcomingHoursMinValue = biquoteUpcomingHoursMin
	biquoteUpcomingHoursMaxValue = biquoteUpcomingHoursMax
	biquoteRecentHoursMinValue   = biquoteRecentHoursMin
	biquoteRecentHoursMaxValue   = biquoteRecentHoursMax
	biquoteMinReleasesMinValue   = biquoteMinReleasesMin
	biquoteMinReleasesMaxValue   = biquoteMinReleasesMax
	biquoteSurpriseMinValue      = biquoteSurpriseMin
	biquoteSurpriseMaxValue      = biquoteSurpriseMax

	biquoteDefaultUpcomingHours   = "24"
	biquoteDefaultMinReleases     = "3"
	biquoteDefaultRecentHours     = "6"
	biquoteDefaultSurprisePercent = "20"
)

// biquoteUpcomingHoursParameter ventana hacia adelante en la que se busca una publicacion de alto impacto
var biquoteUpcomingHoursParameter = CallbackParameter{
	Name:         biquoteHoursParameterName,
	Label:        "Ventana (horas)",
	Description:  "Horas desde ahora (1 a 168) en las que se busca la publicacion",
	DataType:     cmm.ApiParameterTypeInteger,
	IsRequired:   true,
	DefaultValue: &biquoteDefaultUpcomingHours,
	MinValue:     &biquoteUpcomingHoursMinValue,
	MaxValue:     &biquoteUpcomingHoursMaxValue,
}

// economicCalendarUpcomingParameters parametros de EconomicCalendarHighImpactUpcoming
var economicCalendarUpcomingParameters = []CallbackParameter{biquoteUpcomingHoursParameter}

// economicCalendarClusterParameters parametros de EconomicCalendarHighImpactCluster
var economicCalendarClusterParameters = []CallbackParameter{
	biquoteUpcomingHoursParameter,
	{
		Name:         biquoteMinReleasesParameterName,
		Label:        "Minimo de publicaciones",
		Description:  "Cantidad minima (2 a 50) de publicaciones de alto impacto distintas (pais y hora) dentro de la ventana",
		DataType:     cmm.ApiParameterTypeInteger,
		IsRequired:   true,
		DefaultValue: &biquoteDefaultMinReleases,
		MinValue:     &biquoteMinReleasesMinValue,
		MaxValue:     &biquoteMinReleasesMaxValue,
	},
}

// economicCalendarSurpriseParameters parametros de EconomicCalendarHighImpactSurprise
var economicCalendarSurpriseParameters = []CallbackParameter{
	{
		Name:         biquoteHoursParameterName,
		Label:        "Ventana (horas)",
		Description:  "Horas hacia atras (1 a 24) en las que se busca la publicacion ya con dato real",
		DataType:     cmm.ApiParameterTypeInteger,
		IsRequired:   true,
		DefaultValue: &biquoteDefaultRecentHours,
		MinValue:     &biquoteRecentHoursMinValue,
		MaxValue:     &biquoteRecentHoursMaxValue,
	},
	{
		Name:         biquoteSurprisePercentParameterName,
		Label:        "Sorpresa minima (%)",
		Description:  "Diferencia minima entre el dato real y el pronostico, en porcentaje del pronostico (1 a 1000)",
		DataType:     cmm.ApiParameterTypeFloat,
		IsRequired:   true,
		DefaultValue: &biquoteDefaultSurprisePercent,
		MinValue:     &biquoteSurpriseMinValue,
		MaxValue:     &biquoteSurpriseMaxValue,
	},
}

// biquoteCalendarEvent estructura minima de cada evento del calendario. actual y forecast son null cuando aun no hay
// dato (null no es cero)
type biquoteCalendarEvent struct {
	Time        string   `json:"time"`
	CountryCode string   `json:"countryCode"`
	Name        string   `json:"name"`
	Importance  string   `json:"importance"`
	Type        string   `json:"type"`
	TimeMode    string   `json:"timeMode"`
	Actual      *float64 `json:"actual"`
	Forecast    *float64 `json:"forecast"`
}

// biquoteErrorResponse biquote responde un objeto con error (no un arreglo) cuando los parametros son invalidos
type biquoteErrorResponse struct {
	Error string `json:"error"`
}

// biquoteRelease evento de alto impacto con su ventana de publicacion. Start y End son iguales si la hora es exacta
type biquoteRelease struct {
	Key      string
	Start    time.Time
	End      time.Time
	Actual   *float64
	Forecast *float64
}

// EconomicCalendarHighImpactUpcomingCallback retorna true si hay al menos una publicacion de alto impacto sin dato
// real dentro de las proximas horas (parametro hours). Anticipa volatilidad alta por la publicacion
func EconomicCalendarHighImpactUpcomingCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	hours, err := parseBiquoteUpcomingHours(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	releases, err := parseBiquoteHighImpactReleases(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	now := time.Now().UTC()
	pending := pendingReleasesInWindow(releases, now, now.Add(time.Duration(hours)*time.Hour))
	hasUpcoming := len(pending) > 0

	return hasUpcoming, cmm.NewEmptyErrorHandler()
}

// EconomicCalendarHighImpactClusterCallback retorna true si dentro de las proximas horas (parametro hours) hay al menos
// minReleases publicaciones de alto impacto sin dato real. Varios indicadores del mismo pais a la misma hora (ej: CPI
// m/m y CPI y/y) cuentan como una sola publicacion
func EconomicCalendarHighImpactClusterCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	hours, err := parseBiquoteUpcomingHours(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	minReleases, err := parseBiquoteMinReleases(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	releases, err := parseBiquoteHighImpactReleases(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	now := time.Now().UTC()
	pending := pendingReleasesInWindow(releases, now, now.Add(time.Duration(hours)*time.Hour))
	distinct := countDistinctReleases(pending)
	isCluster := distinct >= minReleases

	return isCluster, cmm.NewEmptyErrorHandler()
}

// EconomicCalendarHighImpactSurpriseCallback retorna true si en las ultimas horas (parametro hours) salio una
// publicacion de alto impacto cuyo dato real se aleja del pronostico al menos surprisePercent (en % del pronostico).
// Las publicaciones sin pronostico o con pronostico 0 no se pueden medir y se ignoran
func EconomicCalendarHighImpactSurpriseCallback(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	hours, err := parseBiquoteRecentHours(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	surprisePercent, err := parseBiquoteSurprisePercent(parameters)
	if !err.IsEmtpy() {
		return false, err
	}

	releases, err := parseBiquoteHighImpactReleases(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	now := time.Now().UTC()
	published := publishedReleasesInWindow(releases, now.Add(-time.Duration(hours)*time.Hour), now)
	maxSurprise := maxSurprisePercent(published)
	isSurprise := maxSurprise >= surprisePercent

	return isSurprise, cmm.NewEmptyErrorHandler()
}

// parseBiquoteHighImpactReleases retorna los eventos de importancia alta que no son feriados, sin depender del filtro
// importance de la peticion
func parseBiquoteHighImpactReleases(responseData []byte) ([]biquoteRelease, *cmm.ErrorHandler) {
	var events []biquoteCalendarEvent
	if errJSON := json.Unmarshal(responseData, &events); errJSON != nil {
		var errorResponse biquoteErrorResponse
		if json.Unmarshal(responseData, &errorResponse) == nil && errorResponse.Error != "" {
			return nil, cmm.NewErrorHandler(errors.New(errorResponse.Error), "biquote returned an error message", cmm.LevelError, "PCBIQE002")
		}
		return nil, cmm.NewErrorHandler(errJSON, "Error parsing biquote economic calendar response data", cmm.LevelFatal, "PCBIQE001")
	}

	releases := make([]biquoteRelease, 0, len(events))
	for _, event := range events {
		if !strings.EqualFold(event.Importance, biquoteImportanceHigh) || strings.EqualFold(event.Type, biquoteTypeHoliday) {
			continue
		}

		start, errTime := time.Parse(time.RFC3339, event.Time)
		if errTime != nil {
			return nil, cmm.NewErrorHandler(fmt.Errorf("event %q: %w", event.Name, errTime), "Error parsing biquote event time", cmm.LevelFatal, "PCBIQE003")
		}

		end := start
		if event.TimeMode != biquoteTimeModeExact {
			end = start.Add(biquoteInexactWindow)
		}

		releases = append(releases, biquoteRelease{
			Key:      event.CountryCode + "|" + event.Time,
			Start:    start.UTC(),
			End:      end.UTC(),
			Actual:   event.Actual,
			Forecast: event.Forecast,
		})
	}

	return releases, cmm.NewEmptyErrorHandler()
}

// pendingReleasesInWindow publicaciones sin dato real cuya ventana se cruza con [from, to]
func pendingReleasesInWindow(releases []biquoteRelease, from, to time.Time) []biquoteRelease {
	result := make([]biquoteRelease, 0)
	for _, release := range releases {
		isPending := release.Actual == nil
		if isPending && releaseOverlaps(release, from, to) {
			result = append(result, release)
		}
	}
	return result
}

// publishedReleasesInWindow publicaciones con dato real cuya ventana se cruza con [from, to]
func publishedReleasesInWindow(releases []biquoteRelease, from, to time.Time) []biquoteRelease {
	result := make([]biquoteRelease, 0)
	for _, release := range releases {
		isPublished := release.Actual != nil
		if isPublished && releaseOverlaps(release, from, to) {
			result = append(result, release)
		}
	}
	return result
}

// releaseOverlaps la hora exacta debe estar en [from, to]; sin hora exacta basta con que el dia se cruce con la ventana
func releaseOverlaps(release biquoteRelease, from, to time.Time) bool {
	if release.Start.Equal(release.End) {
		return !release.Start.Before(from) && !release.Start.After(to)
	}
	return !release.Start.After(to) && release.End.After(from)
}

// countDistinctReleases cuenta las publicaciones por pais y hora
func countDistinctReleases(releases []biquoteRelease) int {
	keys := make(map[string]bool)
	for _, release := range releases {
		keys[release.Key] = true
	}
	return len(keys)
}

// maxSurprisePercent mayor diferencia entre dato real y pronostico, en % del pronostico. 0 si ninguna se puede medir
func maxSurprisePercent(releases []biquoteRelease) float64 {
	maxSurprise := 0.0
	for _, release := range releases {
		if release.Actual == nil || release.Forecast == nil || *release.Forecast == 0 {
			continue
		}
		surprise := math.Abs(*release.Actual-*release.Forecast) / math.Abs(*release.Forecast) * biquotePercentMultiple
		maxSurprise = math.Max(maxSurprise, surprise)
	}
	return maxSurprise
}

// parseBiquoteUpcomingHours parametro hours de los callbacks que miran hacia adelante
func parseBiquoteUpcomingHours(parameters map[string]any) (int, *cmm.ErrorHandler) {
	hours, ok := biquoteParameterNumber(parameters[biquoteHoursParameterName])
	if !ok || hours != math.Trunc(hours) {
		return 0, cmm.NewErrorHandler(fmt.Errorf("hours %T %v", parameters[biquoteHoursParameterName], parameters[biquoteHoursParameterName]), "The callback hours parameter is missing or is not an integer", cmm.LevelError, "PCBIQE004")
	}
	if hours < biquoteUpcomingHoursMin || hours > biquoteUpcomingHoursMax {
		return 0, cmm.NewErrorHandler(fmt.Errorf("hours %v", hours), "The callback hours parameter is out of range [1, 168]", cmm.LevelError, "PCBIQE005")
	}
	return int(hours), cmm.NewEmptyErrorHandler()
}

// parseBiquoteRecentHours parametro hours del callback que mira hacia atras
func parseBiquoteRecentHours(parameters map[string]any) (int, *cmm.ErrorHandler) {
	hours, ok := biquoteParameterNumber(parameters[biquoteHoursParameterName])
	if !ok || hours != math.Trunc(hours) {
		return 0, cmm.NewErrorHandler(fmt.Errorf("hours %T %v", parameters[biquoteHoursParameterName], parameters[biquoteHoursParameterName]), "The callback hours parameter is missing or is not an integer", cmm.LevelError, "PCBIQE006")
	}
	if hours < biquoteRecentHoursMin || hours > biquoteRecentHoursMax {
		return 0, cmm.NewErrorHandler(fmt.Errorf("hours %v", hours), "The callback hours parameter is out of range [1, 24]", cmm.LevelError, "PCBIQE007")
	}
	return int(hours), cmm.NewEmptyErrorHandler()
}

// parseBiquoteMinReleases parametro minReleases del callback de acumulacion de publicaciones
func parseBiquoteMinReleases(parameters map[string]any) (int, *cmm.ErrorHandler) {
	minReleases, ok := biquoteParameterNumber(parameters[biquoteMinReleasesParameterName])
	if !ok || minReleases != math.Trunc(minReleases) {
		return 0, cmm.NewErrorHandler(fmt.Errorf("minReleases %T %v", parameters[biquoteMinReleasesParameterName], parameters[biquoteMinReleasesParameterName]), "The callback minReleases parameter is missing or is not an integer", cmm.LevelError, "PCBIQE008")
	}
	if minReleases < biquoteMinReleasesMin || minReleases > biquoteMinReleasesMax {
		return 0, cmm.NewErrorHandler(fmt.Errorf("minReleases %v", minReleases), "The callback minReleases parameter is out of range [2, 50]", cmm.LevelError, "PCBIQE009")
	}
	return int(minReleases), cmm.NewEmptyErrorHandler()
}

// parseBiquoteSurprisePercent parametro surprisePercent del callback de sorpresa
func parseBiquoteSurprisePercent(parameters map[string]any) (float64, *cmm.ErrorHandler) {
	surprisePercent, ok := biquoteParameterNumber(parameters[biquoteSurprisePercentParameterName])
	if !ok {
		return 0, cmm.NewErrorHandler(fmt.Errorf("surprisePercent %T %v", parameters[biquoteSurprisePercentParameterName], parameters[biquoteSurprisePercentParameterName]), "The callback surprisePercent parameter is missing or is not a number", cmm.LevelError, "PCBIQE010")
	}
	if surprisePercent < biquoteSurpriseMin || surprisePercent > biquoteSurpriseMax {
		return 0, cmm.NewErrorHandler(fmt.Errorf("surprisePercent %v", surprisePercent), "The callback surprisePercent parameter is out of range [1, 1000]", cmm.LevelError, "PCBIQE011")
	}
	return surprisePercent, cmm.NewEmptyErrorHandler()
}

// biquoteParameterNumber convierte los numeros de parameters_callback (int64/float64 al crear, int32 al leer de Mongo)
func biquoteParameterNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case int64:
		return float64(number), true
	case int32:
		return float64(number), true
	case int:
		return float64(number), true
	default:
		return 0, false
	}
}
