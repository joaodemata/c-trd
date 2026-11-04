package agents

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	cmm "c_trd/common"
)

// Los prompts y schemas van dentro del binario del runner, no hay que copiarlos al servidor
//
//go:embed prompts/*.md schemas/*.json
var promptFiles embed.FS

const (
	PromptConditionDesigner = "01_condition_designer"
	PromptTriggerComposer   = "02_trigger_composer"
	PromptPineAuthor        = "03_pine_author"
)

// LoadPrompt retorna el system prompt de un agente con su JSON schema al final, para que sepa el formato exacto
func LoadPrompt(name string) (string, *cmm.ErrorHandler) {
	prompt, err := promptFiles.ReadFile("prompts/" + name + ".md")
	if err != nil {
		return "", cmm.NewErrorHandler(err, fmt.Sprintf("No existe el prompt %s", name), cmm.LevelFatal, "AGOUTE001")
	}
	schema, err := promptFiles.ReadFile("schemas/" + name + ".schema.json")
	if err != nil {
		return "", cmm.NewErrorHandler(err, fmt.Sprintf("No existe el schema %s", name), cmm.LevelFatal, "AGOUTE006")
	}
	return fmt.Sprintf("%s\n\n## JSON schema of your answer (schemas/%s.schema.json)\n\n```json\n%s\n```\n", prompt, name, schema), cmm.NewEmptyErrorHandler()
}

// ExtractJSONObject toma el objeto JSON de la respuesta del agente aunque venga dentro de ``` o con texto alrededor
func ExtractJSONObject(text string) (string, *cmm.ErrorHandler) {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return "", cmm.NewErrorHandler(fmt.Errorf("no json object"), "La respuesta no contiene un objeto JSON", cmm.LevelError, "AGOUTE002")
	}

	candidate := text[start : end+1]
	if !json.Valid([]byte(candidate)) {
		return "", cmm.NewErrorHandler(fmt.Errorf("invalid json"), "La respuesta no es un JSON valido", cmm.LevelError, "AGOUTE003")
	}
	return candidate, cmm.NewEmptyErrorHandler()
}

// DecodeAgentOutput decodifica la salida de un agente al tipo de su schema (sin campos desconocidos) y retorna tambien
// el objeto crudo que se guarda en los pasos de la corrida
func DecodeAgentOutput[T any](text string) (T, map[string]any, *cmm.ErrorHandler) {
	var out T
	raw := map[string]any{}

	object, err := ExtractJSONObject(text)
	if !err.IsEmtpy() {
		return out, raw, err
	}

	if errRaw := json.Unmarshal([]byte(object), &raw); errRaw != nil {
		return out, map[string]any{}, cmm.NewErrorHandler(errRaw, "La respuesta no es un objeto JSON", cmm.LevelError, "AGOUTE004")
	}

	decoder := json.NewDecoder(bytes.NewReader([]byte(object)))
	decoder.DisallowUnknownFields()
	if errDecode := decoder.Decode(&out); errDecode != nil {
		return out, raw, cmm.NewErrorHandler(errDecode, fmt.Sprintf("La respuesta no cumple el schema: %v", errDecode), cmm.LevelError, "AGOUTE005")
	}
	return out, raw, cmm.NewEmptyErrorHandler()
}

// ValidatePineOutput revisa lo minimo del contrato del Pine Author antes de guardar el codigo en el trigger
func ValidatePineOutput(out PineAuthorOutput) []string {
	problems := []string{}
	checks := []struct {
		ok      bool
		problem string
	}{
		{strings.Contains(out.PineIndicator, "//@version=6"), "pineIndicator: falta //@version=6"},
		{strings.Contains(out.PineIndicator, "indicator("), "pineIndicator: falta indicator(...)"},
		{strings.Contains(out.PineIndicator, "alert("), "pineIndicator: falta alert(...)"},
		{strings.Contains(out.PineIndicator, cmm.TriggerCodeWebhookPlaceholder), "pineIndicator: falta el placeholder " + cmm.TriggerCodeWebhookPlaceholder},
		{!strings.Contains(out.PineIndicator, "{{ID_CANCEL}}"), "pineIndicator: {{ID_CANCEL}} debe reemplazarse por un id de cancelReasons"},
		{strings.Contains(out.PineStrategy, "//@version=6"), "pineStrategy: falta //@version=6"},
		{strings.Contains(out.PineStrategy, "strategy("), "pineStrategy: falta strategy(...)"},
		{len(out.PineIndicator) <= 65536, "pineIndicator: supera 64 KB"},
		{out.RiskLogic.PlannedRR >= 2, fmt.Sprintf("riskLogic.plannedRR=%v menor que 2", out.RiskLogic.PlannedRR)},
	}
	for _, check := range checks {
		if !check.ok {
			problems = append(problems, check.problem)
		}
	}
	return problems
}

// RequestBody body de POST /operation/create/condition: la condicion sin los campos de razonamiento
func (c DesignedCondition) RequestBody() map[string]any {
	return map[string]any{
		"tag":                c.Tag,
		"name":               c.Name,
		"description":        c.Description,
		"idTypeCondition":    c.IdTypeCondition,
		"idApi":              c.IdApi,
		"parameters":         emptyIfNil(c.Parameters),
		"callbackFunction":   c.CallbackFunction,
		"parametersCallback": emptyIfNil(c.ParametersCallback),
		"intervalSeconds":    c.IntervalSeconds,
		"idTypeVisibility":   c.IdTypeVisibility,
		"allowedUsers":       nonNilStrings(c.AllowedUsers),
	}
}

// RequestBody body de POST /operation/create/trigger con el codigo del Pine Author
func (t ComposedTrigger) RequestBody(code string) map[string]any {
	return map[string]any{
		"tag":                    t.Tag,
		"name":                   t.Name,
		"description":            t.Description,
		"idAsset":                t.IdAsset,
		"idAction":               t.IdAction,
		"conditions":             nonNilStrings(t.Conditions),
		"idCandlestickTimeframe": t.IdCandlestickTimeframe,
		"maxCandlestickQty":      t.MaxCandlestickQty,
		"idTypeVisibility":       t.IdTypeVisibility,
		"allowedUsers":           nonNilStrings(t.AllowedUsers),
		"code":                   code,
	}
}

func emptyIfNil(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	return values
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
