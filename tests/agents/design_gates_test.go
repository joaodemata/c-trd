package agents_test

import (
	"c_trd/agents"
	"c_trd/models"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// requireProblems valida que haya un problema por cada fragmento esperado, o ninguno si want esta vacio
func requireProblems(t *testing.T, problems []string, want []string) {
	t.Helper()
	if len(problems) != len(want) {
		t.Fatalf("problemas = %v, se esperaban %d que contengan %v", problems, len(want), want)
	}
	for _, fragment := range want {
		found := false
		for _, p := range problems {
			if strings.Contains(p, fragment) {
				found = true
			}
		}
		if !found {
			t.Errorf("ningun problema contiene %q: %v", fragment, problems)
		}
	}
}

func designed(callback string, params map[string]string) agents.DesignedCondition {
	return agents.DesignedCondition{
		Tag: "TAG-" + callback, CallbackFunction: callback, ParametersCallback: params,
		IntervalSeconds: 3600, DirectionCheck: agents.DirectionSupportsAction,
	}
}

func TestValidateDesignedConditions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		conditions []agents.DesignedCondition
		want       []string
	}{
		{"valida", []agents.DesignedCondition{designed("FearAndGreedBelow", map[string]string{"threshold": "25"})}, nil},
		{"sin parametros", []agents.DesignedCondition{designed("FearAndGreedExtremeFear", nil)}, nil},
		{"callback inexistente", []agents.DesignedCondition{designed("NoExiste", nil)}, []string{`callback "NoExiste" no existe`}},
		{"parametro desconocido", []agents.DesignedCondition{designed("FearAndGreedExtremeFear", map[string]string{"x": "1"})}, []string{`no acepta el parametro "x"`}},
		{"umbral no numerico", []agents.DesignedCondition{designed("FearAndGreedBelow", map[string]string{"threshold": "bajo"})}, []string{"no es"}},
		{"umbral fuera de rango", []agents.DesignedCondition{designed("BtcDominanceAbove", map[string]string{"threshold": "150"})}, []string{"mayor que"}},
		{"intervalo corto", []agents.DesignedCondition{func() agents.DesignedCondition {
			c := designed("FearAndGreedExtremeFear", nil)
			c.IntervalSeconds = 60
			return c
		}()}, []string{"intervalSeconds=60"}},
		{"tag repetido", []agents.DesignedCondition{designed("FearAndGreedFear", nil), designed("FearAndGreedFear", nil)}, []string{"repetido"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			problems := agents.ValidateDesignedConditions(agents.ConditionDesignerOutput{Status: "OK", Conditions: tc.conditions})
			requireProblems(t, problems, tc.want)
		})
	}
}

// stored condicion como la devuelve GET /list/conditions
func stored(callback string, params map[string]any, intervalSeconds int) models.ConditionsModelType {
	return models.ConditionsModelType{
		ID: primitive.NewObjectID(), Name: callback, CallbackFunction: callback, ParametersCallback: params,
		IntervalSeconds: &intervalSeconds,
	}
}

func composed(timeframeQty int, chosen ...models.ConditionsModelType) agents.TriggerComposerOutput {
	out := agents.TriggerComposerOutput{Status: "OK", Trigger: agents.ComposedTrigger{MaxCandlestickQty: timeframeQty, Conditions: []string{}}}
	for _, c := range chosen {
		out.Trigger.Conditions = append(out.Trigger.Conditions, c.ID.Hex())
		out.ConditionChoices = append(out.ConditionChoices, agents.ConditionChoice{IdCondition: c.ID.Hex(), DirectionCheck: agents.DirectionRiskFilter})
	}
	return out
}

func TestValidateComposedTrigger(t *testing.T) {
	t.Parallel()

	fear := stored("FearAndGreedFear", map[string]any{}, 3600)
	btcAbove60 := stored("BtcDominanceAbove60", map[string]any{}, 3600)
	fredDaily := stored("FredSeriesRising", map[string]any{}, 86400)
	available := []models.ConditionsModelType{fear, btcAbove60, fredDaily}

	missingChoice := composed(30, fear, fredDaily)
	missingChoice.ConditionChoices = missingChoice.ConditionChoices[:1]

	extraChoice := composed(30, fear)
	extraChoice.ConditionChoices = append(extraChoice.ConditionChoices, agents.ConditionChoice{IdCondition: fredDaily.ID.Hex(), DirectionCheck: agents.DirectionRiskFilter})

	badDirection := composed(30, fear)
	badDirection.ConditionChoices[0].DirectionCheck = "MAYBE"

	unknown := composed(30)
	unknown.Trigger.Conditions = []string{primitive.NewObjectID().Hex()}

	repeated := composed(30, fear)
	repeated.Trigger.Conditions = append(repeated.Trigger.Conditions, fear.ID.Hex())

	cases := []struct {
		name      string
		out       agents.TriggerComposerOutput
		timeframe string
		want      []string
	}{
		{"sin condiciones", composed(30), "1-DAY", nil},
		{"valido", composed(30, fear, btcAbove60, fredDaily), "1-DAY", nil},
		{"ventana corta", composed(6, fredDaily), "4-HOURS", []string{"se evalua cada"}},
		{"ventana justa", composed(12, fredDaily), "4-HOURS", nil},
		{"temporalidad desconocida", composed(30), "3-DAYS", []string{"duracion conocida"}},
		{"velas fuera de rango", composed(0), "1-DAY", []string{"maxCandlestickQty=0"}},
		{"demasiadas velas", composed(501), "1-DAY", []string{"maxCandlestickQty=501"}},
		{"condicion inexistente", unknown, "1-DAY", []string{"no esta en las condiciones existentes", "falta la justificacion"}},
		{"condicion repetida", repeated, "1-DAY", []string{"repetida"}},
		{"falta justificacion", missingChoice, "1-DAY", []string{"falta la justificacion"}},
		{"justificacion de otra condicion", extraChoice, "1-DAY", []string{"no esta en trigger.conditions"}},
		{"direccion invalida", badDirection, "1-DAY", []string{"directionCheck"}},
		{"needs input no se valida", agents.TriggerComposerOutput{Status: "NEEDS_INPUT"}, "", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			problems := agents.ValidateComposedTrigger(tc.out, available, tc.timeframe)
			requireProblems(t, problems, tc.want)
		})
	}
}
