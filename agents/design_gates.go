package agents

import (
	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/providers"
	"fmt"
	"slices"
	"strconv"
	"time"
)

// ---------- Validaciones deterministas de las salidas de los agentes 1 y 2 ----------
// Devuelven la lista de problemas; vacia = valida. Se reenvian al agente para que corrija.
// No reemplazan a los data validates de las rutas: son un filtro previo que no necesita MongoDB

// ValidateDesignedConditions revisa la salida del Condition Designer contra FunctionRegistry
func ValidateDesignedConditions(out ConditionDesignerOutput) []string {
	problems := []string{}
	if len(out.Conditions) > MaxConditionsPerDesign {
		problems = append(problems, fmt.Sprintf("conditions: %d condiciones, el maximo es %d", len(out.Conditions), MaxConditionsPerDesign))
	}

	tags := map[string]bool{}
	for i, c := range out.Conditions {
		prefix := fmt.Sprintf("conditions[%d]", i)
		if tags[c.Tag] {
			problems = append(problems, fmt.Sprintf("%s: tag %q repetido", prefix, c.Tag))
		}
		tags[c.Tag] = true

		if c.IntervalSeconds < AgentMinIntervalSeconds {
			problems = append(problems, fmt.Sprintf("%s: intervalSeconds=%d menor que %d", prefix, c.IntervalSeconds, AgentMinIntervalSeconds))
		}
		if !isValidDirection(c.DirectionCheck) {
			problems = append(problems, fmt.Sprintf("%s: directionCheck %q invalido", prefix, c.DirectionCheck))
		}

		def, exists := providers.FunctionRegistry[c.CallbackFunction]
		if !exists {
			problems = append(problems, fmt.Sprintf("%s: callback %q no existe", prefix, c.CallbackFunction))
			continue
		}
		problems = append(problems, validateCallbackParameters(prefix, def.Parameters, c.ParametersCallback)...)
	}
	return problems
}

// validateCallbackParameters mismas reglas que BuildConditionCallbackParametersService: sin parametros desconocidos,
// los requeridos sin valor por defecto deben venir y los numericos dentro de [minValue, maxValue]
func validateCallbackParameters(prefix string, defs []providers.CallbackParameter, values map[string]string) []string {
	problems := []string{}
	for name := range values {
		known := slices.ContainsFunc(defs, func(p providers.CallbackParameter) bool { return p.Name == name })
		if !known {
			problems = append(problems, fmt.Sprintf("%s: el callback no acepta el parametro %q", prefix, name))
		}
	}

	for _, p := range defs {
		raw, present := values[p.Name]
		if !present {
			if p.IsRequired && p.DefaultValue == nil {
				problems = append(problems, fmt.Sprintf("%s: falta parametro requerido %q", prefix, p.Name))
			}
			continue
		}
		problems = append(problems, validateCallbackParameterValue(prefix, p, raw)...)
	}
	return problems
}

func validateCallbackParameterValue(prefix string, p providers.CallbackParameter, raw string) []string {
	switch p.DataType {
	case cmm.ApiParameterTypeBoolean:
		if _, err := strconv.ParseBool(raw); err != nil {
			return []string{fmt.Sprintf("%s: %q=%q no es BOOLEAN", prefix, p.Name, raw)}
		}
		return nil
	case cmm.ApiParameterTypeInteger, cmm.ApiParameterTypeFloat:
	default:
		return nil
	}

	num, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return []string{fmt.Sprintf("%s: %q=%q no es %s", prefix, p.Name, raw, p.DataType)}
	}
	if p.DataType == cmm.ApiParameterTypeInteger && num != float64(int64(num)) {
		return []string{fmt.Sprintf("%s: %q=%q no es INTEGER", prefix, p.Name, raw)}
	}
	if p.MinValue != nil && num < *p.MinValue {
		return []string{fmt.Sprintf("%s: %q=%v menor que %v", prefix, p.Name, num, *p.MinValue)}
	}
	if p.MaxValue != nil && num > *p.MaxValue {
		return []string{fmt.Sprintf("%s: %q=%v mayor que %v", prefix, p.Name, num, *p.MaxValue)}
	}
	return nil
}

func isValidDirection(direction string) bool {
	return direction == DirectionSupportsAction || direction == DirectionRiskFilter
}

// ValidateComposedTrigger revisa la salida del Trigger Composer contra las condiciones existentes (GET /list/conditions)
// y la temporalidad elegida (tag del catalogo TYPE_CANDLE_STICK). Las combinaciones imposibles de condiciones las
// rechaza el backend al crear el trigger (services.FindIncompatibleConditionsService)
func ValidateComposedTrigger(out TriggerComposerOutput, available []models.ConditionsModelType, timeframeTag string) []string {
	problems := []string{}
	if out.Status != "OK" {
		return problems
	}

	trigger := out.Trigger
	if trigger.MaxCandlestickQty < 1 || trigger.MaxCandlestickQty > MaxCandlestickQty {
		problems = append(problems, fmt.Sprintf("trigger.maxCandlestickQty=%d fuera de [1, %d]", trigger.MaxCandlestickQty, MaxCandlestickQty))
	}
	if !cmm.IsKnownTimeframeTag(timeframeTag) {
		problems = append(problems, fmt.Sprintf("trigger.idCandlestickTimeframe: la temporalidad %q no tiene duracion conocida", timeframeTag))
	}
	if len(trigger.Conditions) > MaxConditionsPerTrigger {
		problems = append(problems, fmt.Sprintf("trigger.conditions: %d condiciones, el maximo es %d", len(trigger.Conditions), MaxConditionsPerTrigger))
	}

	byID := make(map[string]models.ConditionsModelType, len(available))
	for _, c := range available {
		byID[c.ID.Hex()] = c
	}

	chosen := []models.ConditionsModelType{}
	seen := map[string]bool{}
	for i, id := range trigger.Conditions {
		if seen[id] {
			problems = append(problems, fmt.Sprintf("trigger.conditions[%d]: %s repetida", i, id))
			continue
		}
		seen[id] = true
		condition, exists := byID[id]
		if !exists {
			problems = append(problems, fmt.Sprintf("trigger.conditions[%d]: %s no esta en las condiciones existentes", i, id))
			continue
		}
		chosen = append(chosen, condition)
	}

	problems = append(problems, validateConditionChoices(out.ConditionChoices, seen)...)
	problems = append(problems, validateTriggerWindow(trigger.MaxCandlestickQty, timeframeTag, chosen)...)
	return problems
}

// validateConditionChoices cada condicion del trigger debe tener exactamente una justificacion y viceversa
func validateConditionChoices(choices []ConditionChoice, inTrigger map[string]bool) []string {
	problems := []string{}
	explained := map[string]bool{}
	for i, choice := range choices {
		if !inTrigger[choice.IdCondition] {
			problems = append(problems, fmt.Sprintf("conditionChoices[%d]: %s no esta en trigger.conditions", i, choice.IdCondition))
		}
		if explained[choice.IdCondition] {
			problems = append(problems, fmt.Sprintf("conditionChoices[%d]: %s repetida", i, choice.IdCondition))
		}
		explained[choice.IdCondition] = true
		if !isValidDirection(choice.DirectionCheck) {
			problems = append(problems, fmt.Sprintf("conditionChoices[%d]: directionCheck %q invalido", i, choice.DirectionCheck))
		}
	}
	for id := range inTrigger {
		if !explained[id] {
			problems = append(problems, fmt.Sprintf("conditionChoices: falta la justificacion de %s", id))
		}
	}
	return problems
}

// validateTriggerWindow la ventana de la oportunidad (velas x temporalidad) debe alcanzar para que cada condicion
// se evalue al menos 2 veces; si no, la oportunidad puede expirar antes de que la condicion se actualice
func validateTriggerWindow(maxCandles int, timeframeTag string, chosen []models.ConditionsModelType) []string {
	problems := []string{}
	if !cmm.IsKnownTimeframeTag(timeframeTag) {
		return problems
	}

	window := cmm.ParseTimeframeTag(timeframeTag) * time.Duration(maxCandles)
	for _, c := range chosen {
		if c.IntervalSeconds == nil {
			continue
		}
		interval := time.Duration(*c.IntervalSeconds) * time.Second
		if window < 2*interval {
			problems = append(problems, fmt.Sprintf("condicion %s (%s): se evalua cada %s y la ventana del trigger es %s", c.ID.Hex(), c.Name, interval, window))
		}
	}
	return problems
}
