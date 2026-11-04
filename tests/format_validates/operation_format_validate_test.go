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

// requireCode valida el codigo del trigger: nil si no se envio
func requireCode(t *testing.T, got *string, want *string) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Errorf("code = %q, se esperaba nil", *got)
		}
		return
	}
	if got == nil || *got != *want {
		t.Errorf("code = %v, se esperaba %q", got, *want)
	}
}

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
	pineCode := "//@version=6\nindicator(\"RSI\")"

	tests := []struct {
		name           string
		edit           func(body map[string]any)
		wantNext       bool
		wantTracking   string
		wantConditions int
		wantCode       *string
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
		{name: "velas como texto numerico", edit: func(b map[string]any) { b["maxCandlestickQty"] = "6" }, wantNext: true},
		{name: "velas como texto no numerico", edit: func(b map[string]any) { b["maxCandlestickQty"] = "seis" }, wantTracking: "CC001"},
		{name: "condicion que no es ObjectID", edit: func(b map[string]any) { b["conditions"] = []string{"abc"} }, wantTracking: "CC003"},
		{name: "condicion repetida", edit: func(b map[string]any) { b["conditions"] = []string{repeatedID, repeatedID} }, wantTracking: "CC003"},
		{name: "con codigo", edit: func(b map[string]any) { b["code"] = pineCode }, wantNext: true, wantCode: &pineCode},
		{name: "codigo muy largo", edit: func(b map[string]any) { b["code"] = strings.Repeat("a", 65537) }, wantTracking: "CC003"},
		{name: "codigo que no es texto", edit: func(b map[string]any) { b["code"] = 5 }, wantTracking: "CC001"},
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
			requireCode(t, payload.Code, tt.wantCode)
		})
	}
}

func TestUpdateTriggerFormatValidate(t *testing.T) {
	t.Parallel()

	idTrigger := primitive.NewObjectID().Hex()
	pineCode := "//@version=6"

	tests := []struct {
		name         string
		edit         func(body map[string]any)
		wantNext     bool
		wantTracking string
		wantCode     *string
	}{
		{name: "valido", edit: func(map[string]any) {}, wantNext: true},
		{name: "con condiciones", wantNext: true, edit: func(b map[string]any) {
			b["conditions"] = []string{primitive.NewObjectID().Hex()}
		}},
		{name: "sin trigger", edit: func(b map[string]any) { delete(b, "idTrigger") }, wantTracking: "CC003"},
		{name: "trigger que no es ObjectID", edit: func(b map[string]any) { b["idTrigger"] = "abc" }, wantTracking: "CC003"},
		{name: "sin tag", edit: func(b map[string]any) { delete(b, "tag") }, wantTracking: "CC003"},
		{name: "sin asset", edit: func(b map[string]any) { delete(b, "idAsset") }, wantTracking: "CC003"},
		{name: "cero velas", edit: func(b map[string]any) { b["maxCandlestickQty"] = 0 }, wantTracking: "CC003"},
		{name: "velas como texto numerico", edit: func(b map[string]any) { b["maxCandlestickQty"] = "6" }, wantNext: true},
		{name: "velas como texto no numerico", edit: func(b map[string]any) { b["maxCandlestickQty"] = "seis" }, wantTracking: "CC001"},
		{name: "con codigo", edit: func(b map[string]any) { b["code"] = pineCode }, wantNext: true, wantCode: &pineCode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := validTriggerBody()
			body["idTrigger"] = idTrigger
			tt.edit(body)
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}

			req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(string(encoded)))
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			fv.UpdateTriggerFormatValidate(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				res := testutil.DecodeResponse(t, rec)
				if res.Tracking != tt.wantTracking {
					t.Errorf("tracking = %s, se esperaba %s", res.Tracking, tt.wantTracking)
				}
				return
			}

			payload, errCtx := cmm.GetFromRequestContext[*fv.UpdateTriggerFormat](spy.Request, "payload")
			testutil.RequireEmptyError(t, errCtx)
			if payload.IdTrigger != idTrigger || payload.Tag != "TRIGGER-RSI" || payload.MaxCandlestickQty != 6 {
				t.Errorf("payload = %+v, se esperaba el trigger %s con tag TRIGGER-RSI y 6 velas", payload, idTrigger)
			}
			requireCode(t, payload.Code, tt.wantCode)
		})
	}
}

// validConditionBody body valido para crear una condicion de cronjob, cada caso modifica un campo
func validConditionBody() map[string]any {
	return map[string]any{
		"tag":                "PUT-CALL-AAPL",
		"name":               "Put/Call AAPL",
		"description":        "Ratio alto o bajo",
		"idTypeCondition":    primitive.NewObjectID().Hex(),
		"idApi":              primitive.NewObjectID().Hex(),
		"parameters":         map[string]string{"symbol": "AAPL"},
		"callbackFunction":   "ProcessAlphaVantageData",
		"parametersCallback": map[string]string{"threshold": "60"},
		"intervalSeconds":    3600,
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
		{name: "isWebhookOnly false", edit: func(b map[string]any) { b["isWebhookOnly"] = false }, wantNext: true},
		{name: "isWebhookOnly true", edit: func(b map[string]any) { b["isWebhookOnly"] = true }, wantTracking: "CC003"},
		{name: "isWebhookOnly como texto false", edit: func(b map[string]any) { b["isWebhookOnly"] = "false" }, wantNext: true},
		{name: "isWebhookOnly como texto true", edit: func(b map[string]any) { b["isWebhookOnly"] = "true" }, wantTracking: "CC003"},
		{name: "isWebhookOnly como texto no booleano", edit: func(b map[string]any) { b["isWebhookOnly"] = "no" }, wantTracking: "CC001"},
		{name: "sin api", edit: func(b map[string]any) { delete(b, "idApi") }, wantTracking: "CC003"},
		{name: "api que no es ObjectID", edit: func(b map[string]any) { b["idApi"] = "abc" }, wantTracking: "CC003"},
		{name: "sin tag", edit: func(b map[string]any) { delete(b, "tag") }, wantTracking: "CC003"},
		{name: "sin nombre", edit: func(b map[string]any) { delete(b, "name") }, wantTracking: "CC003"},
		{name: "sin descripcion", edit: func(b map[string]any) { delete(b, "description") }, wantTracking: "CC003"},
		{name: "sin tipo de condicion", edit: func(b map[string]any) { delete(b, "idTypeCondition") }, wantTracking: "CC003"},
		{name: "tipo de condicion que no es ObjectID", edit: func(b map[string]any) { b["idTypeCondition"] = "abc" }, wantTracking: "CC003"},
		{name: "sin parametros del callback", edit: func(b map[string]any) { delete(b, "parametersCallback") }, wantNext: true},
		{name: "parametro del callback numerico", edit: func(b map[string]any) { b["parametersCallback"] = map[string]any{"threshold": 60} }, wantTracking: "CC001"},
		{name: "sin callback", edit: func(b map[string]any) { delete(b, "callbackFunction") }, wantTracking: "CC003"},
		{name: "sin intervalo", edit: func(b map[string]any) { delete(b, "intervalSeconds") }, wantTracking: "CC003"},
		{name: "intervalo menor al ticker", edit: func(b map[string]any) { b["intervalSeconds"] = 59 }, wantTracking: "CC003"},
		{name: "intervalo negativo", edit: func(b map[string]any) { b["intervalSeconds"] = -60 }, wantTracking: "CC003"},
		{name: "intervalo como texto numerico", edit: func(b map[string]any) { b["intervalSeconds"] = "3600" }, wantNext: true},
		{name: "intervalo como texto menor al ticker", edit: func(b map[string]any) { b["intervalSeconds"] = "59" }, wantTracking: "CC003"},
		{name: "intervalo como texto no numerico", edit: func(b map[string]any) { b["intervalSeconds"] = "una hora" }, wantTracking: "CC001"},
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
			if payload.Name != "Put/Call AAPL" || payload.Tag != "PUT-CALL-AAPL" || payload.IdTypeCondition == "" {
				t.Errorf("payload = %+v, se esperaban el tag, el nombre y el tipo enviados", payload)
			}
		})
	}
}

// TestVisibilityFormatValidate la visibilidad viene en el mismo body del trigger o la condicion y se inyecta en visibilityPayload
func TestVisibilityFormatValidate(t *testing.T) {
	t.Parallel()

	repeatedID := primitive.NewObjectID().Hex()

	tests := []struct {
		name         string
		edit         func(body map[string]any)
		wantNext     bool
		wantTracking string
		wantUsers    int
	}{
		{name: "sin usuarios permitidos", edit: func(map[string]any) {}, wantNext: true},
		{name: "con usuarios permitidos", wantNext: true, wantUsers: 2, edit: func(b map[string]any) {
			b["allowedUsers"] = []string{primitive.NewObjectID().Hex(), primitive.NewObjectID().Hex()}
		}},
		{name: "sin visibilidad", edit: func(b map[string]any) { delete(b, "idTypeVisibility") }, wantTracking: "CC003"},
		{name: "visibilidad que no es ObjectID", edit: func(b map[string]any) { b["idTypeVisibility"] = "PUBLIC" }, wantTracking: "CC003"},
		{name: "usuario permitido que no es ObjectID", edit: func(b map[string]any) { b["allowedUsers"] = []string{"jmata"} }, wantTracking: "CC003"},
		{name: "usuario permitido repetido", edit: func(b map[string]any) { b["allowedUsers"] = []string{repeatedID, repeatedID} }, wantTracking: "CC003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Body completo de un trigger con su visibilidad, como lo envia el frontend
			body := validTriggerBody()
			body["idTypeVisibility"] = primitive.NewObjectID().Hex()
			tt.edit(body)
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(encoded)))
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			fv.CreateTriggerFormatValidate(fv.VisibilityFormatValidate(spy)).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				if res := testutil.DecodeResponse(t, rec); res.Tracking != tt.wantTracking {
					t.Errorf("tracking = %s, se esperaba %s", res.Tracking, tt.wantTracking)
				}
				return
			}

			visibility, errCtx := cmm.GetFromRequestContext[*fv.VisibilityFormat](spy.Request, "visibilityPayload")
			testutil.RequireEmptyError(t, errCtx)
			if visibility.IdTypeVisibility != body["idTypeVisibility"] || len(visibility.AllowedUsers) != tt.wantUsers {
				t.Errorf("visibilidad = %+v, se esperaba %v con %d usuarios", visibility, body["idTypeVisibility"], tt.wantUsers)
			}
			// El payload del trigger sigue disponible para su data validate
			_, errCtx = cmm.GetFromRequestContext[*fv.CreateTriggerFormat](spy.Request, "payload")
			testutil.RequireEmptyError(t, errCtx)
		})
	}
}

// runFormatValidate envia el body al middleware y retorna si llamo a next y el tracking de la respuesta
func runFormatValidate(t *testing.T, middleware func(http.Handler) http.Handler, body map[string]any) (*testutil.NextSpy, string) {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(encoded)))
	rec := httptest.NewRecorder()
	spy := &testutil.NextSpy{}
	middleware(spy).ServeHTTP(rec, req)
	if spy.Called {
		return spy, ""
	}
	return spy, testutil.DecodeResponse(t, rec).Tracking
}

func TestCreateAgentThesisFormatValidate(t *testing.T) {
	t.Parallel()

	valid := func() map[string]any {
		return map[string]any{
			"idAsset":  primitive.NewObjectID().Hex(),
			"idAction": primitive.NewObjectID().Hex(),
			"thesis":   "BTC en miedo extremo sobre el soporte semanal, horizonte 3 meses",
		}
	}

	tests := []struct {
		name         string
		edit         func(body map[string]any)
		wantTracking string
	}{
		{"valida", func(map[string]any) {}, ""},
		{"sin tesis", func(b map[string]any) { delete(b, "thesis") }, "CC003"},
		{"tesis corta", func(b map[string]any) { b["thesis"] = "comprar" }, "CC003"},
		{"tesis larga", func(b map[string]any) { b["thesis"] = strings.Repeat("a", 2001) }, "CC003"},
		{"asset que no es ObjectID", func(b map[string]any) { b["idAsset"] = "BTC" }, "CC003"},
		{"sin accion", func(b map[string]any) { delete(b, "idAction") }, "CC003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := valid()
			tt.edit(body)
			_, tracking := runFormatValidate(t, fv.CreateAgentThesisFormatValidate, body)
			if tracking != tt.wantTracking {
				t.Errorf("tracking = %q, se esperaba %q", tracking, tt.wantTracking)
			}
		})
	}
}

func TestFinishAgentRunFormatValidate(t *testing.T) {
	t.Parallel()

	valid := func() map[string]any {
		return map[string]any{
			"idRun":     primitive.NewObjectID().Hex(),
			"tagStatus": "RUN-SUCCESS",
			"steps": []map[string]any{{
				"agent": "CONDITION_DESIGNER", "attempt": 1, "problems": []string{},
				"output": map[string]any{"status": "OK", "conditions": []any{map[string]any{"intervalSeconds": "3600", "parametersCallback": map[string]any{"threshold": "25"}}}},
			}},
			"conditions": []string{primitive.NewObjectID().Hex()},
			"idTrigger":  primitive.NewObjectID().Hex(),
		}
	}

	tests := []struct {
		name         string
		edit         func(body map[string]any)
		wantTracking string
	}{
		{"valida", func(map[string]any) {}, ""},
		{"sin pasos ni trigger", func(b map[string]any) { delete(b, "steps"); delete(b, "idTrigger"); delete(b, "conditions") }, ""},
		{"con requeue", func(b map[string]any) { b["tagStatus"] = "RUN-FAILED"; b["requeue"] = true; b["error"] = "sin uso" }, ""},
		{"status en ejecucion", func(b map[string]any) { b["tagStatus"] = "RUN-RUNNING" }, "CC003"},
		{"agente desconocido", func(b map[string]any) {
			b["steps"] = []map[string]any{{"agent": "OTRO", "attempt": 1}}
		}, "CC003"},
		{"intento cero", func(b map[string]any) {
			b["steps"] = []map[string]any{{"agent": "PINE_AUTHOR", "attempt": 0}}
		}, "CC003"},
		{"condicion repetida", func(b map[string]any) {
			id := primitive.NewObjectID().Hex()
			b["conditions"] = []string{id, id}
		}, "CC003"},
		{"error muy largo", func(b map[string]any) { b["error"] = strings.Repeat("e", 4001) }, "CC003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body := valid()
			tt.edit(body)
			spy, tracking := runFormatValidate(t, fv.FinishAgentRunFormatValidate, body)
			if tracking != tt.wantTracking {
				t.Fatalf("tracking = %q, se esperaba %q", tracking, tt.wantTracking)
			}
			if tt.name != "valida" {
				return
			}

			// La salida del agente se guarda tal cual, sin convertir los textos numericos
			payload, errCtx := cmm.GetFromRequestContext[*fv.FinishAgentRunFormat](spy.Request, "payload")
			testutil.RequireEmptyError(t, errCtx)
			conditions := payload.Steps[0].Output["conditions"].([]any)
			if conditions[0].(map[string]any)["intervalSeconds"] != "3600" {
				t.Errorf("output = %v, se esperaba conservar el texto", payload.Steps[0].Output)
			}
		})
	}
}

func TestReviewFormatValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		middleware   func(http.Handler) http.Handler
		body         map[string]any
		wantTracking string
	}{
		{"aprobar condicion", fv.ReviewConditionFormatValidate, map[string]any{"idCondition": primitive.NewObjectID().Hex(), "decision": "APPROVE"}, ""},
		{"rechazar trigger", fv.ReviewTriggerFormatValidate, map[string]any{"idTrigger": primitive.NewObjectID().Hex(), "decision": "REJECT"}, ""},
		{"decision desconocida", fv.ReviewTriggerFormatValidate, map[string]any{"idTrigger": primitive.NewObjectID().Hex(), "decision": "PENDING"}, "CC003"},
		{"sin condicion", fv.ReviewConditionFormatValidate, map[string]any{"decision": "APPROVE"}, "CC003"},
		{"borrar tesis", fv.DeleteAgentThesisFormatValidate, map[string]any{"idThesis": primitive.NewObjectID().Hex()}, ""},
		{"borrar tesis sin id", fv.DeleteAgentThesisFormatValidate, map[string]any{}, "CC003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, tracking := runFormatValidate(t, tt.middleware, tt.body)
			if tracking != tt.wantTracking {
				t.Errorf("tracking = %q, se esperaba %q", tracking, tt.wantTracking)
			}
		})
	}
}
