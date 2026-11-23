package backtest

import (
	"context"
	"fmt"
	"log"
	"maps"
	"strings"
	"time"

	cmm "c_trd/common"
	"c_trd/cronjobs"
	"c_trd/models"
	"c_trd/providers"
	"c_trd/services"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Ejecucion de las corridas en cola. Solo la llama el binario cmd/backtester: nunca corre dentro del servidor HTTP ni
// del cronjob, asi un backtest largo no compite con la evaluacion de condiciones

// ProcessQueuedBacktests marca fallidas las corridas abandonadas y ejecuta hasta maxRuns corridas en cola, las mas
// antiguas primero. Retorna cuantas ejecuto
func ProcessQueuedBacktests(maxRuns int) int {
	stale, err := services.FailStaleBacktestRunsService(time.Now().UTC())
	if !err.IsEmtpy() {
		log.Printf("[BACKTEST] Error marcando las corridas abandonadas: %v", err)
	}
	if stale > 0 {
		log.Printf("[BACKTEST] Corridas abandonadas marcadas como fallidas: %d", stale)
	}

	processed := 0
	for processed < maxRuns {
		run, err := services.ClaimBacktestRunService(time.Now().UTC())
		if !err.IsEmtpy() {
			log.Printf("[BACKTEST] Error tomando una corrida de la cola: %v", err)
			break
		}
		if run.ID.IsZero() {
			break
		}

		log.Printf("[BACKTEST] Corrida %s: trigger %s sobre el dataset %s", run.ID.Hex(), run.TagTrigger, run.IDDataset.Hex())
		finish := ExecuteBacktestRun(run)
		if err := services.FinishBacktestRunService(finish, time.Now().UTC()); !err.IsEmtpy() {
			log.Printf("[BACKTEST] Error cerrando la corrida %s: %v", run.ID.Hex(), err)
		}
		log.Printf("[BACKTEST] Corrida %s: %s", run.ID.Hex(), finish.Status.Tag)
		processed++
	}

	return processed
}

// ExecuteBacktestRun ejecuta una corrida y retorna con que cerrarla. Cualquier error la cierra como fallida con el
// mensaje y el tracking code
func ExecuteBacktestRun(run models.BacktestRunsModelType) services.BacktestRunFinish {
	// 1. Velas y señales del trigger en el archivo copiado al encolar
	content, err := services.ReadBacktestDatasetFileService(run.DatasetSha256)
	if !err.IsEmtpy() {
		return failedFinish(run.ID, err)
	}

	chart, err := ParseChartCSV(content)
	if !err.IsEmtpy() {
		return failedFinish(run.ID, err)
	}

	signals, hasSignals := chart.Signals[strings.ToUpper(run.TagTrigger)]
	if !hasSignals {
		errSignals := cmm.NewErrorHandler(fmt.Errorf("tag %s", run.TagTrigger), fmt.Sprintf("El dataset no tiene la columna %s%s", cmm.BacktestSignalColumnPrefix, run.TagTrigger), cmm.LevelError, "BTRUNE001")
		return failedFinish(run.ID, errSignals)
	}

	isLong, err := IsLongAction(run.TagAction)
	if !err.IsEmtpy() {
		return failedFinish(run.ID, err)
	}

	if !cmm.IsKnownTimeframeTag(run.TagCandlestickTimeframe) {
		errTimeframe := cmm.NewErrorHandler(fmt.Errorf("timeframe %q", run.TagCandlestickTimeframe), "La temporalidad del trigger no tiene una duracion conocida", cmm.LevelError, "BTRUNE002")
		return failedFinish(run.ID, errTimeframe)
	}
	timeframe := cmm.ParseTimeframeTag(run.TagCandlestickTimeframe)

	if err := ValidateSignalLevels(chart.Bars, signals, isLong); !err.IsEmtpy() {
		return failedFinish(run.ID, err)
	}

	// 2. Condiciones actuales del trigger
	trigger, err := services.GetTriggerService(run.IDTrigger)
	if !err.IsEmtpy() {
		return failedFinish(run.ID, err)
	}
	if trigger.ID.IsZero() {
		errTrigger := cmm.NewErrorHandler(fmt.Errorf("trigger %s", run.IDTrigger.Hex()), "El trigger se borro o se reemplazo despues de encolar la corrida", cmm.LevelError, "BTRUNE003")
		return failedFinish(run.ID, errTrigger)
	}

	idConditions := make([]primitive.ObjectID, len(trigger.Conditions))
	for i, condition := range trigger.Conditions {
		idConditions[i] = condition.IDCondition
	}

	conditions, err := services.GetConditionsByIDsService(idConditions)
	if !err.IsEmtpy() {
		return failedFinish(run.ID, err)
	}
	if len(conditions) != len(idConditions) {
		errConditions := cmm.NewErrorHandler(fmt.Errorf("found %d of %d", len(conditions), len(idConditions)), "Alguna condicion del trigger ya no existe", cmm.LevelError, "BTRUNE004")
		return failedFinish(run.ID, errConditions)
	}

	providerIDs := make([]primitive.ObjectID, len(conditions))
	for i, condition := range conditions {
		providerIDs[i] = condition.IDProvider
	}

	providersByID, err := services.GetProvidersByIDsService(providerIDs)
	if !err.IsEmtpy() {
		return failedFinish(run.ID, err)
	}

	// 3. Fuentes del estado de cada condicion, solo con datos hasta el cierre de la ultima vela
	lastClose := chart.Bars[len(chart.Bars)-1].Time.Add(timeframe)
	timelines := make([]ConditionTimeline, 0, len(conditions))
	runConditions := make([]models.BacktestRunCondition, 0, len(conditions))
	for _, condition := range conditions {
		timeline, runCondition, errTimeline := buildConditionTimeline(run.Settings.ConditionSource, condition, providersByID[condition.IDProvider], lastClose)
		if !errTimeline.IsEmtpy() {
			return failedFinish(run.ID, errTimeline)
		}
		timelines = append(timelines, timeline)
		runConditions = append(runConditions, runCondition)
	}

	coverage := map[primitive.ObjectID]*ConditionCoverage{}
	var check ConditionCheck
	if len(timelines) > 0 {
		check = NewConditionCheck(timelines, coverage)
	}

	// 4. Las dos variantes con las mismas señales
	input := SimulationInput{
		Bars: chart.Bars, Signals: signals, Timeframe: timeframe, IsLong: isLong, MaxBars: run.MaxCandlestickQty,
		FeeBps: run.Settings.FeeBps, SlippageBps: run.Settings.SlippageBps, CheckConditions: check,
	}

	input.Variant = cmm.BacktestVariantTriggerOnly
	triggerOnly, err := Simulate(input)
	if !err.IsEmtpy() {
		return failedFinish(run.ID, err)
	}

	input.Variant = cmm.BacktestVariantWithConditions
	withConditions, err := Simulate(input)
	if !err.IsEmtpy() {
		return failedFinish(run.ID, err)
	}

	trades := append(triggerOnly.Trades, withConditions.Trades...)
	if len(trades) > cmm.BacktestMaxTrades {
		errTrades := cmm.NewErrorHandler(fmt.Errorf("trades %d", len(trades)), fmt.Sprintf("La corrida tiene %d operaciones, el maximo es %d: usa un CSV con menos velas", len(trades), cmm.BacktestMaxTrades), cmm.LevelError, "BTRUNE005")
		return failedFinish(run.ID, errTrades)
	}

	// 5. Cobertura de cada condicion y metricas
	for i := range runConditions {
		counter, has := coverage[runConditions[i].IDCondition]
		if !has || counter.Checks == 0 {
			continue
		}
		checks := float64(counter.Checks)
		runConditions[i].Checks = counter.Checks
		runConditions[i].CoveragePctEvaluations = float64(counter.Evaluations) / checks * 100
		runConditions[i].CoveragePctCurrentData = float64(counter.CurrentData) / checks * 100
		runConditions[i].CoveragePctUnknown = float64(counter.Unknown) / checks * 100
	}

	results := &models.BacktestResults{TriggerOnly: ComputeMetrics(triggerOnly), WithConditions: ComputeMetrics(withConditions)}
	finish := services.BacktestRunFinish{
		IDRun: run.ID, Status: cmm.BacktestRunStatusSuccess, Conditions: runConditions, Results: results, Trades: trades,
	}
	return finish
}

// IsLongAction true si la accion del trigger es compra y false si es venta
func IsLongAction(tagAction string) (bool, *cmm.ErrorHandler) {
	switch tagAction {
	case cmm.ActionTagBuy:
		return true, cmm.NewEmptyErrorHandler()
	case cmm.ActionTagSell:
		return false, cmm.NewEmptyErrorHandler()
	}
	return false, cmm.NewErrorHandler(fmt.Errorf("action %q", tagAction), fmt.Sprintf("La accion %q no es %s ni %s", tagAction, cmm.ActionTagBuy, cmm.ActionTagSell), cmm.LevelError, "BTRUNE006")
}

// buildConditionTimeline fuentes del estado de la condicion segun condition_source: las evaluaciones guardadas hasta
// until y, si el callback se puede reconstruir, la historia actual del proveedor. Si la historia falla la condicion
// sigue con las evaluaciones y el error queda en la corrida
func buildConditionTimeline(source string, condition models.ConditionsModelType, provider models.ProvidersModelType, until time.Time) (ConditionTimeline, models.BacktestRunCondition, *cmm.ErrorHandler) {
	runCondition := models.BacktestRunCondition{
		IDCondition: condition.ID, TagCondition: condition.Tag, NameCondition: condition.Name,
		CallbackFunction: condition.CallbackFunction, ParametersCallback: condition.ParametersCallback,
	}

	evaluations := []models.ConditionEvaluationsModelType{}
	if source != cmm.BacktestConditionSourceCurrentDataOnly {
		stored, err := services.GetConditionEvaluationsUntilService(condition.ID, until)
		if !err.IsEmtpy() {
			return ConditionTimeline{}, runCondition, err
		}
		evaluations = stored
	}

	var currentData CurrentDataFunc
	definition, canRebuild := providers.PointInTimeRegistry[condition.CallbackFunction]
	if source != cmm.BacktestConditionSourceEvaluationsOnly && canRebuild {
		series, err := fetchCurrentDataSeries(condition, provider, definition)
		if !err.IsEmtpy() {
			message := err.Message
			runCondition.CurrentDataError = &message
		}
		if err.IsEmtpy() {
			currentData = NewCurrentDataFunc(definition.Decide, series, condition.ParametersCallback)
		}
	}

	maxEvaluationAge := cronjobs.ConditionInterval(condition) * cmm.BacktestEvaluationMaxAgeIntervals
	timeline := NewConditionTimeline(condition.ID, evaluations, maxEvaluationAge, currentData)
	return timeline, runCondition, cmm.NewEmptyErrorHandler()
}

// fetchCurrentDataSeries trae una vez la historia del proveedor con la peticion de la condicion sin los parametros que
// la recortan y la convierte en serie
func fetchCurrentDataSeries(condition models.ConditionsModelType, provider models.ProvidersModelType, definition providers.PointInTimeDefinition) ([]providers.Observation, *cmm.ErrorHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.BacktestHistoryTimeout)
	defer cancel()

	request := HistoryRequest(condition.HttpRequest, definition.HistoryDropQuery)
	credentials := providers.RequestCredentials{SecretRef: provider.SecretRef, AllowedHosts: provider.AllowedHosts}

	body, err := providers.ExecuteRequest(ctx, &request, credentials)
	if !err.IsEmtpy() {
		return nil, err
	}

	series, err := definition.Extract(body)
	if !err.IsEmtpy() {
		return nil, err
	}

	return series, cmm.NewEmptyErrorHandler()
}

// HistoryRequest copia la peticion de la condicion sin los parametros de query de dropQuery. No modifica la original
func HistoryRequest(config providers.HTTPRequestConfig, dropQuery []string) providers.HTTPRequestConfig {
	request := config
	if config.QueryData == nil {
		return request
	}

	request.QueryData = maps.Clone(config.QueryData)
	for _, key := range dropQuery {
		delete(request.QueryData, key)
	}
	return request
}

// failedFinish cierre de una corrida fallida con el mensaje y el tracking code del error
func failedFinish(idRun primitive.ObjectID, err *cmm.ErrorHandler) services.BacktestRunFinish {
	message := err.Message
	trackingCode := err.TrackingCode
	return services.BacktestRunFinish{IDRun: idRun, Status: cmm.BacktestRunStatusFailed, Error: &message, ErrorTrackingCode: &trackingCode}
}
