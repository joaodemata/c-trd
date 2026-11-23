package providers

import (
	cmm "c_trd/common"
	"time"
)

// Reconstruccion de condiciones para el backtesting. Las reglas sobre series ya evaluan a una fecha (asOf), asi que
// con la historia completa del proveedor se puede saber si la condicion se cumplia en cualquier fecha pasada: se trae
// la serie una vez y se decide en cada fecha. La historia es la actual del proveedor: FRED revisa valores pasados y
// Google Trends reescala la serie a la ventana pedida, por eso el backtest marca estos estados como CURRENT_DATA.
// Los callbacks que no estan aqui (puntuales o sin historia) solo se reconstruyen con las evaluaciones guardadas

// PointInTimeDecision decide la condicion con la serie vista a la fecha asOf y los parametros de la condicion
type PointInTimeDecision func(series []Observation, parameters map[string]any, asOf time.Time) (CallbackResult, *cmm.ErrorHandler)

// PointInTimeDefinition como reconstruir un callback: el extractor de su proveedor, la decision de su regla y los
// parametros de query que se quitan de la peticion de la condicion para traer toda la historia (ej: limit de FRED)
type PointInTimeDefinition struct {
	Extract          func(responseBody []byte) ([]Observation, *cmm.ErrorHandler)
	Decide           PointInTimeDecision
	HistoryDropQuery []string
}

// Parametros de query de FRED que recortan la historia: sin ellos /fred/series/observations retorna toda la serie
var fredHistoryDropQuery = []string{"limit", "observation_start", "offset"}

// PointInTimeRegistry callbacks que se pueden reconstruir con la historia del proveedor, por el ID de FunctionRegistry
var PointInTimeRegistry = map[string]PointInTimeDefinition{
	// FRED
	"FredSeriesAboveWithHysteresis": {Extract: ExtractFredObservations, Decide: aboveWithHysteresisDecision, HistoryDropQuery: fredHistoryDropQuery},
	"FredSeriesBelow":               {Extract: ExtractFredObservations, Decide: belowDecision, HistoryDropQuery: fredHistoryDropQuery},
	"FredSeriesCrossAbove":          {Extract: ExtractFredObservations, Decide: crossUpDecision, HistoryDropQuery: fredHistoryDropQuery},
	"FredSeriesCrossBelow":          {Extract: ExtractFredObservations, Decide: crossDownDecision, HistoryDropQuery: fredHistoryDropQuery},
	"FredSeriesSlopeAbove":          {Extract: ExtractFredObservations, Decide: slopeAboveDecision, HistoryDropQuery: fredHistoryDropQuery},
	"FredSeriesPercentileAbove":     {Extract: ExtractFredObservations, Decide: percentileAboveDecision, HistoryDropQuery: fredHistoryDropQuery},
	"FredSeriesPercentileBelow":     {Extract: ExtractFredObservations, Decide: percentileBelowDecision, HistoryDropQuery: fredHistoryDropQuery},
	"FredSeriesZScoreAbove":         {Extract: ExtractFredObservations, Decide: zScoreAboveDecision, HistoryDropQuery: fredHistoryDropQuery},
	"FredSeriesZScoreBelow":         {Extract: ExtractFredObservations, Decide: zScoreBelowDecision, HistoryDropQuery: fredHistoryDropQuery},
	// Google Trends (SerpApi): la historia es la ventana de fechas que pide la condicion
	"GoogleTrendsAboveWithHysteresis": {Extract: ExtractGoogleTrendsObservations, Decide: aboveWithHysteresisDecision},
	"GoogleTrendsBelow":               {Extract: ExtractGoogleTrendsObservations, Decide: belowDecision},
	"GoogleTrendsCrossAbove":          {Extract: ExtractGoogleTrendsObservations, Decide: crossUpDecision},
	"GoogleTrendsCrossBelow":          {Extract: ExtractGoogleTrendsObservations, Decide: crossDownDecision},
	"GoogleTrendsSlopeAbove":          {Extract: ExtractGoogleTrendsObservations, Decide: slopeAboveDecision},
	"GoogleTrendsPercentileAbove":     {Extract: ExtractGoogleTrendsObservations, Decide: percentileAboveDecision},
	"GoogleTrendsPercentileBelow":     {Extract: ExtractGoogleTrendsObservations, Decide: percentileBelowDecision},
	"GoogleTrendsZScoreAbove":         {Extract: ExtractGoogleTrendsObservations, Decide: zScoreAboveDecision},
	"GoogleTrendsZScoreBelow":         {Extract: ExtractGoogleTrendsObservations, Decide: zScoreBelowDecision},
	// CSV de Google Trends subido por asset: la historia es el archivo actual, sin validar que este al dia
	"GoogleTrendsFileAboveWithHysteresis": {Extract: ExtractGoogleTrendsFileHistory, Decide: aboveWithHysteresisDecision},
	"GoogleTrendsFileBelow":               {Extract: ExtractGoogleTrendsFileHistory, Decide: belowDecision},
	"GoogleTrendsFileCrossAbove":          {Extract: ExtractGoogleTrendsFileHistory, Decide: crossUpDecision},
	"GoogleTrendsFileCrossBelow":          {Extract: ExtractGoogleTrendsFileHistory, Decide: crossDownDecision},
	"GoogleTrendsFileSlopeAbove":          {Extract: ExtractGoogleTrendsFileHistory, Decide: slopeAboveDecision},
	"GoogleTrendsFilePercentileAbove":     {Extract: ExtractGoogleTrendsFileHistory, Decide: percentileAboveDecision},
	"GoogleTrendsFilePercentileBelow":     {Extract: ExtractGoogleTrendsFileHistory, Decide: percentileBelowDecision},
	"GoogleTrendsFileZScoreAbove":         {Extract: ExtractGoogleTrendsFileHistory, Decide: zScoreAboveDecision},
	"GoogleTrendsFileZScoreBelow":         {Extract: ExtractGoogleTrendsFileHistory, Decide: zScoreBelowDecision},
}

// crossUpDecision Cross hacia arriba con level
func crossUpDecision(series []Observation, parameters map[string]any, asOf time.Time) (CallbackResult, *cmm.ErrorHandler) {
	result, err := crossDecision(series, parameters, asOf, CrossUp)
	return result, err
}

// crossDownDecision Cross hacia abajo con level
func crossDownDecision(series []Observation, parameters map[string]any, asOf time.Time) (CallbackResult, *cmm.ErrorHandler) {
	result, err := crossDecision(series, parameters, asOf, CrossDown)
	return result, err
}
