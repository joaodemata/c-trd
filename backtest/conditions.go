package backtest

import (
	"sort"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/providers"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Reconstruccion del estado de las condiciones en una fecha pasada usando solo lo que se sabia en esa fecha:
//  1. La ultima evaluacion guardada por el cronjob con executed_at hasta esa fecha (se usa executed_at y no as_of
//     porque es cuando el sistema conocio el dato). Si es mas vieja que maxEvaluationAge el estado es desconocido
//  2. Si no hay evaluacion util, la historia actual del proveedor vista hasta esa fecha (CURRENT_DATA)
//  3. Si no, desconocido (UNKNOWN), que cuenta como no cumplida

// CurrentDataFunc decide la condicion en una fecha con la historia actual del proveedor
type CurrentDataFunc func(asOf time.Time) (providers.CallbackResult, *cmm.ErrorHandler)

// ConditionTimeline fuentes del estado de una condicion. evaluations solo tiene evaluaciones correctas ordenadas por
// executed_at; currentData es nil si el callback no se puede reconstruir o la corrida no lo permite
type ConditionTimeline struct {
	IDCondition      primitive.ObjectID
	evaluations      []models.ConditionEvaluationsModelType
	maxEvaluationAge time.Duration
	currentData      CurrentDataFunc
}

// ConditionCoverage cuantas veces se consulto la condicion y con que origen se respondio
type ConditionCoverage struct {
	Checks      int
	Evaluations int
	CurrentData int
	Unknown     int
}

// NewConditionTimeline arma las fuentes de una condicion. Descarta las evaluaciones fallidas (evaluated=false: en vivo
// la condicion conserva su status) y ordena el resto por executed_at
func NewConditionTimeline(idCondition primitive.ObjectID, evaluations []models.ConditionEvaluationsModelType, maxEvaluationAge time.Duration, currentData CurrentDataFunc) ConditionTimeline {
	evaluated := make([]models.ConditionEvaluationsModelType, 0, len(evaluations))
	for _, evaluation := range evaluations {
		if evaluation.Evaluated {
			evaluated = append(evaluated, evaluation)
		}
	}
	sort.SliceStable(evaluated, func(i, j int) bool { return evaluated[i].ExecutedAt.Before(evaluated[j].ExecutedAt) })

	return ConditionTimeline{IDCondition: idCondition, evaluations: evaluated, maxEvaluationAge: maxEvaluationAge, currentData: currentData}
}

// NewCurrentDataFunc decide la condicion con la serie ya traida del proveedor y los parametros de la condicion
func NewCurrentDataFunc(decide providers.PointInTimeDecision, series []providers.Observation, parameters map[string]any) CurrentDataFunc {
	return func(asOf time.Time) (providers.CallbackResult, *cmm.ErrorHandler) {
		result, err := decide(series, parameters, asOf)
		return result, err
	}
}

// StateAt estado de la condicion en la fecha at
func StateAt(timeline ConditionTimeline, at time.Time) models.BacktestConditionState {
	state := models.BacktestConditionState{IDCondition: timeline.IDCondition, Accepted: false, Source: cmm.BacktestStateSourceUnknown}

	// Ultima evaluacion con executed_at <= at
	position := sort.Search(len(timeline.evaluations), func(i int) bool { return timeline.evaluations[i].ExecutedAt.After(at) })
	if position > 0 {
		evaluation := timeline.evaluations[position-1]
		isFresh := at.Sub(evaluation.ExecutedAt) <= timeline.maxEvaluationAge
		if isFresh {
			state.Accepted = evaluation.Accepted
			state.Source = cmm.BacktestStateSourceEvaluation
			state.Value = evaluation.Value
			state.AsOf = evaluation.AsOf
			return state
		}
	}

	if timeline.currentData == nil {
		return state
	}

	result, err := timeline.currentData(at)
	if !err.IsEmtpy() {
		return state
	}

	value := result.Value
	state.Accepted = result.Accepted
	state.Source = cmm.BacktestStateSourceCurrentData
	state.Value = &value
	state.AsOf = result.AsOf
	return state
}

// NewConditionCheck consulta todas las condiciones en una fecha y suma el origen de cada estado en coverage (por id de
// condicion). Sin condiciones siempre se cumplen
func NewConditionCheck(timelines []ConditionTimeline, coverage map[primitive.ObjectID]*ConditionCoverage) ConditionCheck {
	return func(at time.Time) ([]models.BacktestConditionState, bool) {
		states := make([]models.BacktestConditionState, 0, len(timelines))
		allMet := true
		for _, timeline := range timelines {
			state := StateAt(timeline, at)
			states = append(states, state)
			if !state.Accepted {
				allMet = false
			}

			counter, has := coverage[timeline.IDCondition]
			if !has {
				counter = &ConditionCoverage{}
				coverage[timeline.IDCondition] = counter
			}
			counter.Checks++
			switch state.Source {
			case cmm.BacktestStateSourceEvaluation:
				counter.Evaluations++
			case cmm.BacktestStateSourceCurrentData:
				counter.CurrentData++
			default:
				counter.Unknown++
			}
		}
		return states, allMet
	}
}
