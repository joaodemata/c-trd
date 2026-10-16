package format_validates_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// validTriggerBody body valido para crear un trigger, cada caso modifica un campo
func validTriggerBody() map[string]any {
	return map[string]any{
		"tag":                    "TRIGGER-RSI",
		"name":                   "RSI",
		"description":            "RSI en sobreventa",
		"idAsset":                primitive.NewObjectID().Hex(),
		"idAction":               primitive.NewObjectID().Hex(),
		"idCandlestickTimeframe": primitive.NewObjectID().Hex(),
		"maxCandlestickQty":      6,
	}
}

func TestCreateTriggerFormatValidate(t *testing.T) {
	t.Parallel()

	repeatedID := primitive.NewObjectID().Hex()

	tests := []struct {
		name           string
		edit           func(body map[string]any)
		wantNext       bool
		wantTracking   string
		wantConditions int
	}{
		{name: "sin condiciones", edit: func(map[string]any) {}, wantNext: true},
		{name: "condiciones vacias", edit: func(b map[string]any) { b["conditions"] = []string{} }, wantNext: true},
		{name: "con condiciones", wantNext: true, wantConditions: 2, edit: func(b map[string]any) {
			b["conditions"] = []string{primitive.NewObjectID().Hex(), primitive.NewObjectID().Hex()}
		}},
		{name: "sin tag", edit: func(b map[string]any) { delete(b, "tag") }, wantTracking: "CC003"},
		{name: "sin nombre", edit: func(b map[string]any) { delete(b, "name") }, wantTracking: "CC003"},
		{name: "sin descripcion", edit: func(b map[string]any) { delete(b, "description") }, wantTracking: "CC003"},
		{name: "asset que no es ObjectID", edit: func(b map[string]any) { b["idAsset"] = "abc" }, wantTracking: "CC003"},
		{name: "sin accion", edit: func(b map[string]any) { delete(b, "idAction") }, wantTracking: "CC003"},
		{name: "temporalidad que no es ObjectID", edit: func(b map[string]any) { b["idCandlestickTimeframe"] = "1H" }, wantTracking: "CC003"},
		{name: "cero velas", edit: func(b map[string]any) { b["maxCandlestickQty"] = 0 }, wantTracking: "CC003"},
		{name: "velas negativas", edit: func(b map[string]any) { b["maxCandlestickQty"] = -3 }, wantTracking: "CC003"},
		{name: "velas como texto", edit: func(b map[string]any) { b["maxCandlestickQty"] = "6" }, wantTracking: "CC001"},
		{name: "condicion que no es ObjectID", edit: func(b map[string]any) { b["conditions"] = []string{"abc"} }, wantTracking: "CC003"},
		{name: "condicion repetida", edit: func(b map[string]any) { b["conditions"] = []string{repeatedID, repeatedID} }, wantTracking: "CC003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := validTriggerBody()
			tt.edit(body)
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(encoded)))
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			fv.CreateTriggerFormatValidate(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				if res := testutil.DecodeResponse(t, rec); res.Tracking != tt.wantTracking {
					t.Errorf("tracking = %s, se esperaba %s", res.Tracking, tt.wantTracking)
				}
				return
			}

			payload, errCtx := cmm.GetFromRequestContext[*fv.CreateTriggerFormat](spy.Request, "payload")
			testutil.RequireEmptyError(t, errCtx)
			if payload.Tag != "TRIGGER-RSI" || payload.MaxCandlestickQty != 6 {
				t.Errorf("payload = %+v, se esperaba tag TRIGGER-RSI y 6 velas", payload)
			}
			if len(payload.Conditions) != tt.wantConditions {
				t.Errorf("conditions = %d, se esperaban %d", len(payload.Conditions), tt.wantConditions)
			}
		})
	}
}

// validConditionBody body valido para crear una condicion de cronjob, cada caso modifica un campo
func validConditionBody() map[string]any {
	return map[string]any{
		"idApi":                primitive.NewObjectID().Hex(),
		"nameCondition":        "Put/Call AAPL",
		"descriptionCondition": "Ratio alto o bajo",
		"parameters":           map[string]string{"symbol": "AAPL"},
		"callbackFunction":     "ProcessAlphaVantageData",
		"intervalSeconds":      3600,
	}
}

func TestCreateConditionFormatValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		edit         func(body map[string]any)
		wantNext     bool
		wantTracking string
	}{
		{name: "valido", edit: func(map[string]any) {}, wantNext: true},
		{name: "sin parametros", edit: func(b map[string]any) { delete(b, "parameters") }, wantNext: true},
		{name: "intervalo minimo", edit: func(b map[string]any) { b["intervalSeconds"] = 60 }, wantNext: true},
		{name: "con metadata", edit: func(b map[string]any) { b["metadata"] = map[string]any{"source": "manual"} }, wantNext: true},
		{name: "isWebhookOnly false", edit: func(b map[string]any) { b["isWebhookOnly"] = false }, wantNext: true},
		{name: "isWebhookOnly true", edit: func(b map[string]any) { b["isWebhookOnly"] = true }, wantTracking: "CC003"},
		{name: "isWebhookOnly como texto", edit: func(b map[string]any) { b["isWebhookOnly"] = "false" }, wantTracking: "CC001"},
		{name: "sin api", edit: func(b map[string]any) { delete(b, "idApi") }, wantTracking: "CC003"},
		{name: "api que no es ObjectID", edit: func(b map[string]any) { b["idApi"] = "abc" }, wantTracking: "CC003"},
		{name: "sin nombre", edit: func(b map[string]any) { delete(b, "nameCondition") }, wantTracking: "CC003"},
		{name: "sin descripcion", edit: func(b map[string]any) { delete(b, "descriptionCondition") }, wantTracking: "CC003"},
		{name: "sin callback", edit: func(b map[string]any) { delete(b, "callbackFunction") }, wantTracking: "CC003"},
		{name: "sin intervalo", edit: func(b map[string]any) { delete(b, "intervalSeconds") }, wantTracking: "CC003"},
		{name: "intervalo menor al ticker", edit: func(b map[string]any) { b["intervalSeconds"] = 59 }, wantTracking: "CC003"},
		{name: "intervalo negativo", edit: func(b map[string]any) { b["intervalSeconds"] = -60 }, wantTracking: "CC003"},
		{name: "intervalo como texto", edit: func(b map[string]any) { b["intervalSeconds"] = "3600" }, wantTracking: "CC001"},
		{name: "parametro numerico", edit: func(b map[string]any) { b["parameters"] = map[string]any{"limit": 5} }, wantTracking: "CC001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := validConditionBody()
			tt.edit(body)
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(encoded)))
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			fv.CreateConditionFormatValidate(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				if res := testutil.DecodeResponse(t, rec); res.Tracking != tt.wantTracking {
					t.Errorf("tracking = %s, se esperaba %s", res.Tracking, tt.wantTracking)
				}
				return
			}

			payload, errCtx := cmm.GetFromRequestContext[*fv.CreateConditionFormat](spy.Request, "payload")
			testutil.RequireEmptyError(t, errCtx)
			if payload.NameCondition != "Put/Call AAPL" {
				t.Errorf("payload = %+v, se esperaba el nombre enviado", payload)
			}
		})
	}
}
