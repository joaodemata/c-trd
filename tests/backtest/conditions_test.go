package backtest_test

import (
	"errors"
	"testing"
	"time"

	"c_trd/backtest"
	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/providers"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// evaluation evaluacion guardada por el cronjob hours horas despues de simulationStart
func evaluation(hours int, evaluated bool, accepted bool, value float64) models.ConditionEvaluationsModelType {
	return models.ConditionEvaluationsModelType{
		ExecutedAt: simulationStart.Add(time.Duration(hours) * time.Hour),
		Evaluated:  evaluated,
		Accepted:   accepted,
		Value:      &value,
	}
}

// currentDataAccepted historia actual del proveedor que siempre responde accepted con value 7
func currentDataAccepted(accepted bool) backtest.CurrentDataFunc {
	return func(asOf time.Time) (providers.CallbackResult, *cmm.ErrorHandler) {
		return providers.CallbackResult{Accepted: accepted, Value: 7, AsOf: &asOf}, cmm.NewEmptyErrorHandler()
	}
}

func TestStateAt(t *testing.T) {
	t.Parallel()

	id := primitive.NewObjectID()
	// Llegan desordenadas; la de la hora 3 fallo (evaluated=false) y se descarta
	evaluations := []models.ConditionEvaluationsModelType{
		evaluation(5, true, false, 2),
		evaluation(1, true, true, 1),
		evaluation(3, false, false, 0),
	}
	maxAge := 3 * time.Hour
	at := func(hours int) time.Time { return simulationStart.Add(time.Duration(hours) * time.Hour) }

	onlyEvaluations := backtest.NewConditionTimeline(id, evaluations, maxAge, nil)
	withCurrentData := backtest.NewConditionTimeline(id, evaluations, maxAge, currentDataAccepted(true))
	failingCurrentData := backtest.NewConditionTimeline(id, evaluations, maxAge, func(time.Time) (providers.CallbackResult, *cmm.ErrorHandler) {
		return providers.CallbackResult{}, cmm.NewErrorHandler(errors.New("sin puntos"), "sin puntos", cmm.LevelError, "TESTE001")
	})

	tests := []struct {
		name         string
		timeline     backtest.ConditionTimeline
		at           time.Time
		wantSource   string
		wantAccepted bool
		wantValue    float64
	}{
		{"antes de la primera evaluacion", onlyEvaluations, at(0), cmm.BacktestStateSourceUnknown, false, 0},
		{"justo en la evaluacion", onlyEvaluations, at(1), cmm.BacktestStateSourceEvaluation, true, 1},
		{"la evaluacion fallida no reemplaza a la anterior", onlyEvaluations, at(4), cmm.BacktestStateSourceEvaluation, true, 1},
		{"la ultima evaluacion antes de la fecha", onlyEvaluations, at(6), cmm.BacktestStateSourceEvaluation, false, 2},
		{"evaluacion vieja sin historia", onlyEvaluations, at(9), cmm.BacktestStateSourceUnknown, false, 0},
		{"antes de la primera evaluacion con historia", withCurrentData, at(0), cmm.BacktestStateSourceCurrentData, true, 7},
		{"evaluacion vieja con historia", withCurrentData, at(9), cmm.BacktestStateSourceCurrentData, true, 7},
		{"la evaluacion tiene prioridad sobre la historia", withCurrentData, at(6), cmm.BacktestStateSourceEvaluation, false, 2},
		{"la historia falla", failingCurrentData, at(0), cmm.BacktestStateSourceUnknown, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := backtest.StateAt(tt.timeline, tt.at)
			if state.Source != tt.wantSource || state.Accepted != tt.wantAccepted || state.IDCondition != id {
				t.Fatalf("estado = %+v, se esperaba %s accepted=%v", state, tt.wantSource, tt.wantAccepted)
			}
			if tt.wantSource == cmm.BacktestStateSourceUnknown {
				if state.Value != nil {
					t.Errorf("valor = %v, se esperaba nil en un estado desconocido", *state.Value)
				}
				return
			}
			requireFloat(t, "valor", state.Value, tt.wantValue)
		})
	}
}

func TestNewConditionCheck(t *testing.T) {
	t.Parallel()

	met := backtest.NewConditionTimeline(primitive.NewObjectID(), nil, time.Hour, currentDataAccepted(true))
	notMet := backtest.NewConditionTimeline(primitive.NewObjectID(), nil, time.Hour, currentDataAccepted(false))
	unknown := backtest.NewConditionTimeline(primitive.NewObjectID(), nil, time.Hour, nil)

	coverage := map[primitive.ObjectID]*backtest.ConditionCoverage{}
	check := backtest.NewConditionCheck([]backtest.ConditionTimeline{met, unknown}, coverage)
	states, allMet := check(simulationStart)
	if allMet || len(states) != 2 {
		t.Fatalf("allMet = %v, estados = %d; una condicion desconocida cuenta como no cumplida", allMet, len(states))
	}
	check(simulationStart.Add(time.Hour))

	if got := coverage[met.IDCondition]; got == nil || got.Checks != 2 || got.CurrentData != 2 || got.Unknown != 0 {
		t.Errorf("cobertura de la cumplida = %+v", got)
	}
	if got := coverage[unknown.IDCondition]; got == nil || got.Checks != 2 || got.Unknown != 2 {
		t.Errorf("cobertura de la desconocida = %+v", got)
	}

	_, allMet = backtest.NewConditionCheck([]backtest.ConditionTimeline{met}, map[primitive.ObjectID]*backtest.ConditionCoverage{})(simulationStart)
	if !allMet {
		t.Error("una sola condicion cumplida debe dar allMet")
	}
	_, allMet = backtest.NewConditionCheck([]backtest.ConditionTimeline{met, notMet}, map[primitive.ObjectID]*backtest.ConditionCoverage{})(simulationStart)
	if allMet {
		t.Error("una condicion no cumplida no debe dar allMet")
	}
}
