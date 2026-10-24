package models_test

import (
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/providers"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// newTestCondition crea una condicion con datos fijos, solo varia el intervalo
func newTestCondition(intervalSeconds int) (models.ConditionsModelType, *cmm.ErrorHandler) {
	request := providers.HTTPRequestConfig{URL: "https://www.alphavantage.co/query", Method: "GET", QueryData: map[string]string{"symbol": "AAPL"}}

	return models.NewConditionModel(
		primitive.NewObjectID(), "Alpha Vantage",
		"Put/Call AAPL", "Ratio alto o bajo",
		primitive.NewObjectID(),
		models.ConditionCronjobData{HttpRequest: request, CallbackFunction: "ProcessAlphaVantageData", IntervalSeconds: intervalSeconds},
	)
}

func TestNewConditionModel(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC()
	condition, err := newTestCondition(3600)
	testutil.RequireEmptyError(t, err)

	if condition.ID.IsZero() {
		t.Error("el ID debe generarse en el constructor")
	}
	// Una condicion nace inactiva, el cronjob la activa cuando se cumple
	if condition.IDStatus != cmm.GenericStatusInactive.ID || condition.TagStatus != cmm.GenericStatusInactive.Tag {
		t.Errorf("status = %s, toda condicion nace en %s", condition.TagStatus, cmm.GenericStatusInactive.Tag)
	}
	// Solo se crean condiciones que evalua el cronjob
	if condition.IsWebhookOnly || condition.IsProcessing || condition.LogicalDelete {
		t.Errorf("condicion = %+v, se esperaba de cronjob, sin lock y sin borrar", condition)
	}
	if condition.HttpRequest.QueryData["symbol"] != "AAPL" || condition.CallbackFunction != "ProcessAlphaVantageData" {
		t.Errorf("http_request = %+v, callback = %s, se esperaban los datos de cronjob", condition.HttpRequest, condition.CallbackFunction)
	}
	if condition.IntervalSeconds == nil || *condition.IntervalSeconds != 3600 {
		t.Errorf("interval_seconds = %v, se esperaba 3600", condition.IntervalSeconds)
	}
	// Se evalua en la siguiente ejecucion del cronjob
	if condition.NextExecutionAt == nil || condition.NextExecutionAt.Before(before) || condition.NextExecutionAt.Location() != time.UTC {
		t.Errorf("next_execution_at = %v, se esperaba ahora en UTC", condition.NextExecutionAt)
	}
	if condition.LastExecutedAt != nil {
		t.Errorf("last_executed_at = %v, una condicion nueva nunca se ha ejecutado", condition.LastExecutedAt)
	}
}

func TestNewConditionModel_EmptyMetadata(t *testing.T) {
	t.Parallel()

	condition, err := newTestCondition(60)
	testutil.RequireEmptyError(t, err)

	// Integridad de la data: metadata siempre como objeto y last_executed_at en null
	doc, errMarshal := bson.Marshal(condition)
	if errMarshal != nil {
		t.Fatalf("bson.Marshal: %v", errMarshal)
	}
	raw := bson.Raw(doc)
	if got := raw.Lookup("metadata").Type; got != bsontype.EmbeddedDocument {
		t.Errorf("metadata es %v, se esperaba un objeto", got)
	}
	if got := raw.Lookup("last_executed_at").Type; got != bsontype.Null {
		t.Errorf("last_executed_at es %v, se esperaba null", got)
	}
}

func TestNewConditionModel_MinInterval(t *testing.T) {
	t.Parallel()

	_, err := newTestCondition(cmm.ConditionMinIntervalSeconds - 1)
	testutil.RequireTrackingCode(t, err, "MCONE001")
}
