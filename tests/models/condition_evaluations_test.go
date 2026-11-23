package models_test

import (
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/providers"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNewConditionEvaluationModel(t *testing.T) {
	t.Parallel()

	executedAt := time.Date(2026, 11, 21, 10, 30, 0, 0, time.UTC)
	asOf := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	condition := models.ConditionsModelType{
		ID:                 primitive.NewObjectID(),
		CallbackFunction:   "BtcDominanceAbove",
		ParametersCallback: map[string]any{"threshold": 60.0},
	}

	t.Run("evaluada guarda resultado, valor y fecha del dato", func(t *testing.T) {
		t.Parallel()

		evaluation, err := models.NewConditionEvaluationModel(condition, models.ConditionEvaluationData{
			Evaluated:  true,
			Result:     providers.CallbackResult{Accepted: true, Value: 61.2, AsOf: &asOf},
			ExecutedAt: executedAt,
		})
		testutil.RequireEmptyError(t, err)

		if evaluation.ID.IsZero() || evaluation.IDCondition != condition.ID {
			t.Errorf("evaluacion = %s de la condicion %s, se esperaba la condicion %s", evaluation.ID.Hex(), evaluation.IDCondition.Hex(), condition.ID.Hex())
		}
		if evaluation.CallbackFunction != condition.CallbackFunction || evaluation.ParametersCallback["threshold"] != 60.0 {
			t.Errorf("la evaluacion debe copiar el callback y sus parametros: %s %v", evaluation.CallbackFunction, evaluation.ParametersCallback)
		}
		if !evaluation.Evaluated || !evaluation.Accepted || evaluation.Value == nil || *evaluation.Value != 61.2 {
			t.Errorf("evaluated, accepted, value = %v, %v, %v; se esperaba true, true, 61.2", evaluation.Evaluated, evaluation.Accepted, evaluation.Value)
		}
		if evaluation.AsOf == nil || !evaluation.AsOf.Equal(asOf) || !evaluation.ExecutedAt.Equal(executedAt) {
			t.Errorf("asOf = %v, executedAt = %s; se esperaba %s, %s", evaluation.AsOf, evaluation.ExecutedAt, asOf, executedAt)
		}
		if evaluation.AsOfSource == nil || *evaluation.AsOfSource != cmm.AsOfSourceProvider {
			t.Errorf("asOfSource = %v, se esperaba %s", evaluation.AsOfSource, cmm.AsOfSourceProvider)
		}
		if evaluation.ErrorTrackingCode != nil {
			t.Errorf("errorTrackingCode = %s, una evaluacion exitosa no tiene error", *evaluation.ErrorTrackingCode)
		}
		if evaluation.CreatedDate.IsZero() || evaluation.LogicalDelete {
			t.Errorf("createdDate = %s, logicalDelete = %v; se esperaba fecha de creacion y sin borrar", evaluation.CreatedDate, evaluation.LogicalDelete)
		}
	})

	t.Run("evaluada sin fecha del proveedor usa executed_at con origen SERVER", func(t *testing.T) {
		t.Parallel()

		evaluation, err := models.NewConditionEvaluationModel(condition, models.ConditionEvaluationData{
			Evaluated:  true,
			Result:     providers.CallbackResult{Accepted: false, Value: 0.865},
			ExecutedAt: executedAt,
		})
		testutil.RequireEmptyError(t, err)

		if evaluation.AsOf == nil || !evaluation.AsOf.Equal(executedAt) {
			t.Errorf("asOf = %v, se esperaba executed_at %s", evaluation.AsOf, executedAt)
		}
		if evaluation.AsOfSource == nil || *evaluation.AsOfSource != cmm.AsOfSourceServer {
			t.Errorf("asOfSource = %v, se esperaba %s", evaluation.AsOfSource, cmm.AsOfSourceServer)
		}
	})

	t.Run("fallida guarda el tracking code y deja vacios los datos", func(t *testing.T) {
		t.Parallel()

		evaluation, err := models.NewConditionEvaluationModel(condition, models.ConditionEvaluationData{
			Evaluated:         false,
			Result:            providers.CallbackResult{Accepted: true, Value: 61.2, AsOf: &asOf},
			ExecutedAt:        executedAt,
			ErrorTrackingCode: "PCCMCE002",
		})
		testutil.RequireEmptyError(t, err)

		if evaluation.Evaluated || evaluation.Accepted || evaluation.Value != nil || evaluation.AsOf != nil || evaluation.AsOfSource != nil {
			t.Errorf("evaluacion = %+v; una evaluacion fallida no guarda aceptacion, valor, fecha del dato ni su origen", evaluation)
		}
		if evaluation.ErrorTrackingCode == nil || *evaluation.ErrorTrackingCode != "PCCMCE002" {
			t.Errorf("errorTrackingCode = %v, se esperaba PCCMCE002", evaluation.ErrorTrackingCode)
		}
	})

	t.Run("sin parametros guarda un objeto vacio", func(t *testing.T) {
		t.Parallel()

		withoutParameters := condition
		withoutParameters.ParametersCallback = nil

		evaluation, err := models.NewConditionEvaluationModel(withoutParameters, models.ConditionEvaluationData{Evaluated: true, ExecutedAt: executedAt})
		testutil.RequireEmptyError(t, err)

		if evaluation.ParametersCallback == nil {
			t.Error("parameters_callback debe guardarse como {} y no null")
		}
	})
}

func TestConditionEvaluationsModel_JSONKeys(t *testing.T) {
	t.Parallel()

	// La respuesta al frontend va en camelCase y sin los campos de auditoria
	keys := jsonKeys(t, models.ConditionEvaluationsModelType{})
	requireHiddenKeys(t, keys,
		[]string{"logicalDelete", "coreDb", "coreVersion", "userDb", "schemaDb", "core_db", "logical_delete"},
		[]string{"idCondition", "callbackFunction", "parametersCallback", "evaluated", "accepted", "value", "asOf", "asOfSource", "executedAt", "errorTrackingCode"},
	)
}
