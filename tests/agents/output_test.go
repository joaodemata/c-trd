package agents_test

import (
	"strings"
	"testing"

	"c_trd/agents"
	"c_trd/tests/testutil"
)

func TestExtractJSONObject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		text         string
		want         string
		wantTracking string
	}{
		{name: "solo json", text: `{"a":1}`, want: `{"a":1}`},
		{name: "con bloque de codigo", text: "```json\n{\"a\":1}\n```", want: `{"a":1}`},
		{name: "con texto alrededor", text: "Aqui esta:\n{\"a\":{\"b\":2}}\nListo", want: `{"a":{"b":2}}`},
		{name: "sin objeto", text: "no hay json", wantTracking: "AGOUTE002"},
		{name: "json roto", text: `{"a":}`, wantTracking: "AGOUTE003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := agents.ExtractJSONObject(tt.text)
			if tt.wantTracking != "" {
				testutil.RequireTrackingCode(t, err, tt.wantTracking)
				return
			}
			testutil.RequireEmptyError(t, err)
			if got != tt.want {
				t.Errorf("json = %q, se esperaba %q", got, tt.want)
			}
		})
	}
}

func TestDecodeAgentOutput(t *testing.T) {
	t.Parallel()

	out, raw, err := agents.DecodeAgentOutput[agents.RiskLogic](`{"stopLossMethod":"ATR","takeProfitMethod":"2R","plannedRR":2.5}`)
	testutil.RequireEmptyError(t, err)
	if out.PlannedRR != 2.5 || raw["stopLossMethod"] != "ATR" {
		t.Errorf("salida = %+v, raw = %v", out, raw)
	}

	// Campo que no esta en el schema: el agente debe corregirlo
	_, raw, err = agents.DecodeAgentOutput[agents.RiskLogic](`{"stopLossMethod":"ATR","extra":true}`)
	testutil.RequireTrackingCode(t, err, "AGOUTE005")
	if raw["extra"] != true {
		t.Errorf("raw = %v, se esperaba conservar la salida para la corrida", raw)
	}

	_, _, err = agents.DecodeAgentOutput[agents.RiskLogic](`[1,2]`)
	testutil.RequireTrackingCode(t, err, "AGOUTE002")
}

func TestLoadPromptIncludesSchema(t *testing.T) {
	t.Parallel()

	for _, name := range []string{agents.PromptConditionDesigner, agents.PromptTriggerComposer, agents.PromptPineAuthor} {
		prompt, err := agents.LoadPrompt(name)
		testutil.RequireEmptyError(t, err)
		if !strings.Contains(prompt, "## JSON schema of your answer") || !strings.Contains(prompt, `"$schema"`) {
			t.Errorf("el prompt %s no incluye su schema", name)
		}
	}

	_, err := agents.LoadPrompt("99_no_existe")
	testutil.RequireTrackingCode(t, err, "AGOUTE001")
}

func validPine() agents.PineAuthorOutput {
	return agents.PineAuthorOutput{
		PineIndicator: `//@version=6
indicator("x")
alert("{\"idWebhook\":\"{{ID_WEBHOOK}}\"}")`,
		PineStrategy: "//@version=6\nstrategy(\"x\")",
		RiskLogic:    agents.RiskLogic{PlannedRR: 2},
	}
}

func TestValidatePineOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		edit func(out *agents.PineAuthorOutput)
		want []string
	}{
		{"valido", func(*agents.PineAuthorOutput) {}, nil},
		{"version vieja", func(o *agents.PineAuthorOutput) { o.PineIndicator = strings.Replace(o.PineIndicator, "=6", "=5", 1) }, []string{"//@version=6"}},
		{"sin alert", func(o *agents.PineAuthorOutput) {
			o.PineIndicator = strings.Replace(o.PineIndicator, "alert(", "plot(", 1)
		}, []string{"alert"}},
		{"sin placeholder", func(o *agents.PineAuthorOutput) {
			o.PineIndicator = strings.Replace(o.PineIndicator, "{{ID_WEBHOOK}}", "abc", 1)
		}, []string{"placeholder"}},
		{"placeholder de cancelacion", func(o *agents.PineAuthorOutput) { o.PineIndicator += "\n// {{ID_CANCEL}}" }, []string{"ID_CANCEL"}},
		{"sin strategy", func(o *agents.PineAuthorOutput) { o.PineStrategy = "//@version=6" }, []string{"strategy"}},
		{"riesgo bajo", func(o *agents.PineAuthorOutput) { o.RiskLogic.PlannedRR = 1.5 }, []string{"plannedRR"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := validPine()
			tt.edit(&out)
			requireProblems(t, agents.ValidatePineOutput(out), tt.want)
		})
	}
}

func TestRequestBodies(t *testing.T) {
	t.Parallel()

	condition := agents.DesignedCondition{Tag: "FNG-LOW", Rationale: "porque", DirectionCheck: agents.DirectionRiskFilter}
	body := condition.RequestBody()
	if _, has := body["rationale"]; has {
		t.Error("el body de la condicion no debe llevar rationale")
	}
	if _, has := body["directionCheck"]; has {
		t.Error("el body de la condicion no debe llevar directionCheck")
	}
	if body["parameters"] == nil || body["allowedUsers"] == nil {
		t.Errorf("los mapas y listas vacios deben ir como {} y [], body = %v", body)
	}

	trigger := agents.ComposedTrigger{Tag: "BTC-1D"}.RequestBody("//@version=6")
	if trigger["code"] != "//@version=6" || trigger["conditions"] == nil {
		t.Errorf("body del trigger = %v", trigger)
	}
}
