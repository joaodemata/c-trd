package agents_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"c_trd/agents"
	cmm "c_trd/common"
	"c_trd/tests/testutil"
)

// Ids fijos del c-trd falso
const (
	fakeIDRun       = "6ae86ce7b01de16516690001"
	fakeIDThesis    = "6ae86ce7b01de16516690002"
	fakeIDAsset     = "6ae86ce7b01de16516690003"
	fakeIDAction    = "6ae86ce7b01de16516690004"
	fakeIDTimeframe = "6ae86ce7b01de16516690005"
	fakeIDApi       = "6ae86ce7b01de16516690006"
	fakeIDCondition = "6ae86ce7b01de16516690007"
	fakeIDTrigger   = "6ae86ce7b01de16516690008"
	fakeIDTypeCond  = "6ae86ce7b01de16516690009"
	fakeIDVisib     = "6ae86ce7b01de16516690010"
)

// fakeCtrd simula la API de c-trd: guarda lo que crea el runner y el cierre de la corrida
type fakeCtrd struct {
	mu                 sync.Mutex
	conditions         []map[string]any
	triggerBodies      []map[string]any
	rejectFirstTrigger bool
	finishes           []agents.FinishRunRequest
}

func respond(w http.ResponseWriter, status string, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": status, "message": "ok", "tracking": "TEST001", "data": data})
}

func (f *fakeCtrd) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/login", func(w http.ResponseWriter, r *http.Request) {
		respond(w, "SUCCESS", map[string]any{"token": "tkn"})
	})
	mux.HandleFunc("POST /auth/logout", func(w http.ResponseWriter, r *http.Request) { respond(w, "SUCCESS", nil) })
	mux.HandleFunc("POST /operation/agent/start_run", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tkn" {
			t.Errorf("start_run sin el token de la sesion")
		}
		respond(w, "SUCCESS", map[string]any{"runs": []map[string]any{{
			"idRun": fakeIDRun,
			"thesis": map[string]any{
				"id": fakeIDThesis, "thesis": "BTC en miedo extremo con soporte semanal", "idAsset": fakeIDAsset,
				"tagAsset": "BTC", "asset": "Bitcoin", "idAction": fakeIDAction, "tagAction": "BUY", "action": "Compra",
			},
		}}})
	})
	mux.HandleFunc("GET /list/conditions", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		respond(w, "SUCCESS", map[string]any{"conditions": f.conditions, "page": len(f.conditions)})
	})
	mux.HandleFunc("GET /list/apis", func(w http.ResponseWriter, r *http.Request) {
		respond(w, "SUCCESS", map[string]any{"apis": []map[string]any{{"id": fakeIDApi, "responseFormat": cmm.ResponseFormatCoinMarketCapFearGreed}}, "page": 1})
	})
	mux.HandleFunc("GET /list/callbacks", func(w http.ResponseWriter, r *http.Request) {
		respond(w, "SUCCESS", map[string]any{"callbacks": []map[string]any{{"id": "FearAndGreedBelow", "responseFormat": r.URL.Query().Get("responseFormat")}}})
	})
	mux.HandleFunc("GET /list/catalogs", func(w http.ResponseWriter, r *http.Request) {
		catalogs := map[string][]map[string]any{
			cmm.CatalogTypeCandleStick: {{"id": fakeIDTimeframe, "tag": "1-DAY", "name": "1 dia"}},
			cmm.CatalogTypeAction:      {{"id": fakeIDAction, "tag": "BUY", "name": "Compra"}},
			cmm.CatalogTypeCondition:   {{"id": fakeIDTypeCond, "tag": "SENTIMENT", "name": "Sentimiento"}},
			cmm.CatalogTypeVisibility:  {{"id": fakeIDVisib, "tag": "VISIBILITY_PRIVATE", "name": "Privado"}},
		}
		respond(w, "SUCCESS", map[string]any{"catalogs": catalogs[r.URL.Query().Get("typeCatalog")]})
	})
	mux.HandleFunc("POST /operation/create/condition", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if _, has := body["rationale"]; has {
			t.Errorf("la condicion se envio con rationale")
		}
		f.mu.Lock()
		f.conditions = append(f.conditions, map[string]any{
			"id": fakeIDCondition, "tag": body["tag"], "callbackFunction": body["callbackFunction"],
			"intervalSeconds": body["intervalSeconds"], "parametersCallback": map[string]any{"threshold": 25},
		})
		f.mu.Unlock()
		respond(w, "SUCCESS", map[string]any{"idCondition": fakeIDCondition})
	})
	mux.HandleFunc("POST /operation/create/trigger", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.triggerBodies = append(f.triggerBodies, body)
		if f.rejectFirstTrigger && len(f.triggerBodies) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]any{"status": "FAIL", "message": "Ya existe un trigger con el tag enviado", "tracking": "DVOPR006", "data": body["tag"]})
			return
		}
		respond(w, "SUCCESS", map[string]any{"idTrigger": fakeIDTrigger, "idWebhook": "x"})
	})
	mux.HandleFunc("PUT /operation/agent/finish_run", func(w http.ResponseWriter, r *http.Request) {
		var finish agents.FinishRunRequest
		json.NewDecoder(r.Body).Decode(&finish)
		f.mu.Lock()
		f.finishes = append(f.finishes, finish)
		f.mu.Unlock()
		respond(w, "SUCCESS", map[string]any{"idRun": finish.IdRun})
	})
	return mux
}

// scriptedLLM responde por agente (segun el system prompt) la siguiente salida de su lista
type scriptedLLM struct {
	mu        sync.Mutex
	responses map[string][]string
	inputs    map[string][]string
	fail      bool
}

func (s *scriptedLLM) Complete(ctx context.Context, systemPrompt string, input string) (string, *cmm.ErrorHandler) {
	if s.fail {
		return "", cmm.NewErrorHandler(fmt.Errorf("usage limit"), "Se acabo el uso del plan", cmm.LevelFatal, "AGTEST001")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, agent := range []string{"Condition Designer", "Trigger Composer", "Pine Author"} {
		if strings.Contains(systemPrompt, agent) {
			s.inputs[agent] = append(s.inputs[agent], input)
			if len(s.responses[agent]) == 0 {
				return "", cmm.NewErrorHandler(fmt.Errorf("no response"), "sin respuesta para "+agent, cmm.LevelFatal, "AGTEST002")
			}
			next := s.responses[agent][0]
			s.responses[agent] = s.responses[agent][1:]
			return next, cmm.NewEmptyErrorHandler()
		}
	}
	return "", cmm.NewErrorHandler(fmt.Errorf("unknown agent"), "agente desconocido", cmm.LevelFatal, "AGTEST003")
}

func designerJSON() string {
	return fmt.Sprintf(`{"status":"OK","reusedConditions":[],"conditions":[{"tag":"FNG-BELOW-25","name":"Miedo extremo",
"description":"Fear and Greed menor a 25","idTypeCondition":%q,"idApi":%q,"parameters":{},
"callbackFunction":"FearAndGreedBelow","parametersCallback":{"threshold":"25"},"intervalSeconds":3600,
"idTypeVisibility":%q,"allowedUsers":[],"rationale":"capitulacion","directionCheck":"SUPPORTS_ACTION"}],
"technicalHints":["pullback a la EMA 50 diaria"],"missingCallbacks":[],"questions":[]}`, fakeIDTypeCond, fakeIDApi, fakeIDVisib)
}

func composerJSON(tag string) string {
	return fmt.Sprintf("```json\n"+`{"status":"OK","trigger":{"tag":%q,"name":"BTC pullback con miedo","description":"Compra en pullback con miedo",
"idAsset":%q,"idAction":%q,"conditions":[%q],"idCandlestickTimeframe":%q,"maxCandlestickQty":30,
"idTypeVisibility":%q,"allowedUsers":[]},"conditionChoices":[{"idCondition":%q,"directionCheck":"SUPPORTS_ACTION","rationale":"miedo"}],
"discardedConditions":[],"requestedConditions":[],"technicalBrief":"Pullback a EMA 50 en 1D","questions":[]}`+"\n```",
		tag, fakeIDAsset, fakeIDAction, fakeIDCondition, fakeIDTimeframe, fakeIDVisib, fakeIDCondition)
}

func pineJSON() string {
	out := validPine()
	out.RiskLogic = agents.RiskLogic{StopLossMethod: "2 ATR", TakeProfitMethod: "2R y 3R", PlannedRR: 3}
	out.AlertSetup = "Alerta en cierre de vela"
	out.Changes = []string{}
	encoded, _ := json.Marshal(out)
	return string(encoded)
}

func newRunner(t *testing.T, fake *fakeCtrd, llm agents.LLM) (*agents.Runner, *agents.CtrdClient) {
	t.Helper()
	server := httptest.NewServer(fake.handler(t))
	t.Cleanup(server.Close)

	client := agents.NewCtrdClient(server.URL + "/")
	testutil.RequireEmptyError(t, client.Login(context.Background(), "claude", "secret"))
	return &agents.Runner{Client: client, LLM: llm, MaxAttempts: 3}, client
}

// TestRunnerSuccess: salida invalida del designer (reintento), trigger rechazado por c-trd (el composer corrige el
// tag y el Pine no se reescribe porque no cambio la temporalidad) y corrida cerrada como exitosa
func TestRunnerSuccess(t *testing.T) {
	t.Parallel()

	// Una condicion rechazada por el super usuario: el Trigger Composer no debe verla
	rejected := map[string]any{"id": "6ae86ce7b01de16516690099", "tag": "FNG-RECHAZADA", "tagStatus": cmm.GenericStatusNotApproved.Tag}
	fake := &fakeCtrd{conditions: []map[string]any{rejected}, rejectFirstTrigger: true}
	llm := &scriptedLLM{inputs: map[string][]string{}, responses: map[string][]string{
		"Condition Designer": {"no tengo json", designerJSON()},
		"Trigger Composer":   {composerJSON("BTC-1D-FEAR"), composerJSON("BTC-1D-FEAR-V2")},
		"Pine Author":        {pineJSON()},
	}}
	runner, _ := newRunner(t, fake, llm)

	testutil.RequireEmptyError(t, runner.Run(context.Background()))

	if len(fake.finishes) != 1 {
		t.Fatalf("cierres = %d, se esperaba 1", len(fake.finishes))
	}
	finish := fake.finishes[0]
	if finish.TagStatus != cmm.AgentRunStatusSuccess.Tag || finish.IdTrigger != fakeIDTrigger || finish.Requeue {
		t.Fatalf("cierre = %+v", finish)
	}
	if len(finish.Conditions) != 1 || finish.Conditions[0] != fakeIDCondition {
		t.Errorf("condiciones creadas = %v", finish.Conditions)
	}

	gotSteps := []string{}
	for _, step := range finish.Steps {
		gotSteps = append(gotSteps, fmt.Sprintf("%s#%d:%d", step.Agent, step.Attempt, len(step.Problems)))
	}
	wantSteps := []string{"CONDITION_DESIGNER#1:1", "CONDITION_DESIGNER#2:0", "PINE_AUTHOR#1:0", "TRIGGER_COMPOSER#1:1", "TRIGGER_COMPOSER#2:0"}
	if strings.Join(gotSteps, ",") != strings.Join(wantSteps, ",") {
		t.Errorf("pasos = %v, se esperaba %v", gotSteps, wantSteps)
	}

	for _, input := range llm.inputs["Trigger Composer"] {
		if strings.Contains(input, "FNG-RECHAZADA") {
			t.Error("el Trigger Composer recibio una condicion no aprobada")
		}
	}

	// El segundo intento del composer recibe el error de c-trd
	if len(llm.inputs["Trigger Composer"]) != 2 || !strings.Contains(llm.inputs["Trigger Composer"][1], "DVOPR006") {
		t.Errorf("el composer no recibio el rechazo de c-trd: %v", llm.inputs["Trigger Composer"])
	}
	if len(fake.triggerBodies) != 2 || fake.triggerBodies[1]["tag"] != "BTC-1D-FEAR-V2" || !strings.Contains(fake.triggerBodies[1]["code"].(string), "{{ID_WEBHOOK}}") {
		t.Errorf("triggers enviados = %v", fake.triggerBodies)
	}
}

// TestRunnerNeedsInput: el designer pide aclarar la tesis y la corrida termina sin crear nada
func TestRunnerNeedsInput(t *testing.T) {
	t.Parallel()

	fake := &fakeCtrd{conditions: []map[string]any{}}
	llm := &scriptedLLM{inputs: map[string][]string{}, responses: map[string][]string{
		"Condition Designer": {`{"status":"NEEDS_INPUT","reusedConditions":[],"conditions":[],"technicalHints":[],"missingCallbacks":[],"questions":["Que horizonte?"]}`},
	}}
	runner, _ := newRunner(t, fake, llm)

	testutil.RequireEmptyError(t, runner.Run(context.Background()))
	if len(fake.finishes) != 1 || fake.finishes[0].TagStatus != cmm.AgentRunStatusNeedsInput.Tag || !strings.Contains(fake.finishes[0].Error, "Que horizonte?") {
		t.Fatalf("cierre = %+v", fake.finishes)
	}
	if len(fake.triggerBodies) != 0 {
		t.Errorf("no se debia crear trigger")
	}
}

// TestRunnerClaudeFailureRequeues: si Claude falla (ej: se acabo el uso) la tesis vuelve a la cola y Run retorna el error
func TestRunnerClaudeFailureRequeues(t *testing.T) {
	t.Parallel()

	fake := &fakeCtrd{conditions: []map[string]any{}}
	runner, _ := newRunner(t, fake, &scriptedLLM{fail: true})

	testutil.RequireTrackingCode(t, runner.Run(context.Background()), "AGTEST001")
	if len(fake.finishes) != 1 || !fake.finishes[0].Requeue || fake.finishes[0].TagStatus != cmm.AgentRunStatusFailed.Tag {
		t.Fatalf("cierre = %+v, se esperaba FAILED con requeue", fake.finishes)
	}
}
