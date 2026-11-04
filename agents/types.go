package agents

// Tipos de entrada y salida de los agentes. Deben coincidir 1:1 con schemas/*.schema.json.
// Todo en camelCase: las salidas de los agentes 1 y 2 se envian tal cual a las rutas de creacion de c-trd
//
// Flujo:
//  1. Condition Designer: crea las condiciones que faltan (POST /operation/create/condition)
//  2. Trigger Composer: arma el trigger con TODAS las condiciones existentes (POST /operation/create/trigger)
//  3. Pine Author: escribe el pine del trigger y su gemelo strategy() para el backtest
//  4. Financial Judge: aprueba, rechaza o pide cambios con las metricas calculadas en Go

// Agentes a los que el juez puede pedir cambios
const (
	AgentConditionDesigner = "CONDITION_DESIGNER"
	AgentTriggerComposer   = "TRIGGER_COMPOSER"
	AgentPineAuthor        = "PINE_AUTHOR"
)

// Rol de una condicion respecto a la accion del trigger
const (
	DirectionSupportsAction = "SUPPORTS_ACTION"
	DirectionRiskFilter     = "RISK_FILTER"
)

// Limites de diseño (mas estrictos que los del backend)
const (
	// El backend acepta 60s; los agentes no consultan una fuente externa mas de una vez cada 15 minutos
	AgentMinIntervalSeconds = 900
	MaxConditionsPerDesign  = 4
	MaxConditionsPerTrigger = 5
	MaxCandlestickQty       = 500
)

// ---------- Agente 1: Condition Designer ----------

type ConditionDesignerOutput struct {
	Status string `json:"status"` // OK | NEEDS_INPUT
	// Condiciones existentes que ya cubren la tesis, se le pasan al Trigger Composer como sugerencia
	ReusedConditions []ReusedCondition   `json:"reusedConditions"`
	Conditions       []DesignedCondition `json:"conditions"`
	TechnicalHints   []string            `json:"technicalHints"`
	MissingCallbacks []MissingCallback   `json:"missingCallbacks"`
	Questions        []string            `json:"questions"`
}

type ReusedCondition struct {
	IdCondition string `json:"idCondition"`
	Rationale   string `json:"rationale"`
}

// DesignedCondition body de POST /operation/create/condition (CreateConditionFormat + VisibilityFormat)
// mas los campos de razonamiento (rationale, directionCheck) que no se envian
type DesignedCondition struct {
	Tag                string            `json:"tag"`
	Name               string            `json:"name"`
	Description        string            `json:"description"`
	IdTypeCondition    string            `json:"idTypeCondition"`
	IdApi              string            `json:"idApi"`
	Parameters         map[string]string `json:"parameters"`
	CallbackFunction   string            `json:"callbackFunction"`
	ParametersCallback map[string]string `json:"parametersCallback"`
	IntervalSeconds    int               `json:"intervalSeconds"`
	IdTypeVisibility   string            `json:"idTypeVisibility"`
	AllowedUsers       []string          `json:"allowedUsers"`
	Rationale          string            `json:"rationale"`
	DirectionCheck     string            `json:"directionCheck"` // SUPPORTS_ACTION | RISK_FILTER
}

type MissingCallback struct {
	ProposedID     string   `json:"proposedId"`
	Provider       string   `json:"provider"`
	DataNeeded     string   `json:"dataNeeded"`
	EvaluationRule string   `json:"evaluationRule"`
	Parameters     []string `json:"parameters"`
}

// ---------- Agente 2: Trigger Composer ----------

type TriggerComposerOutput struct {
	Status  string          `json:"status"` // OK | NEEDS_INPUT
	Trigger ComposedTrigger `json:"trigger"`
	// Una entrada por cada id de trigger.conditions, explica por que se eligio
	ConditionChoices []ConditionChoice `json:"conditionChoices"`
	// Condiciones existentes descartadas que parecian relevantes y por que
	DiscardedConditions []DiscardedCondition `json:"discardedConditions"`
	// Condiciones que faltan y que el Condition Designer deberia crear antes de repetir este paso
	RequestedConditions []string `json:"requestedConditions"`
	// Idea tecnica de entrada para el Pine Author (setup, temporalidad, invalidacion)
	TechnicalBrief string   `json:"technicalBrief"`
	Questions      []string `json:"questions"`
}

// ComposedTrigger body de POST /operation/create/trigger (CreateTriggerFormat + VisibilityFormat)
type ComposedTrigger struct {
	Tag                    string   `json:"tag"`
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	IdAsset                string   `json:"idAsset"`
	IdAction               string   `json:"idAction"`
	Conditions             []string `json:"conditions"`
	IdCandlestickTimeframe string   `json:"idCandlestickTimeframe"`
	MaxCandlestickQty      int      `json:"maxCandlestickQty"`
	IdTypeVisibility       string   `json:"idTypeVisibility"`
	AllowedUsers           []string `json:"allowedUsers"`
}

type ConditionChoice struct {
	IdCondition    string `json:"idCondition"`
	DirectionCheck string `json:"directionCheck"` // SUPPORTS_ACTION | RISK_FILTER
	Rationale      string `json:"rationale"`
}

type DiscardedCondition struct {
	IdCondition string `json:"idCondition"`
	Reason      string `json:"reason"`
}

// ---------- Agente 3: Pine Author ----------

// PineAuthorOutput el tag, accion, temporalidad y velas los decide el Trigger Composer; el pine debe respetarlos
type PineAuthorOutput struct {
	PineIndicator string    `json:"pineIndicator"` // se envia como "code" al crear el trigger (triggers.pine_code)
	PineStrategy  string    `json:"pineStrategy"`  // solo backtest, se guarda en la corrida
	RiskLogic     RiskLogic `json:"riskLogic"`
	AlertSetup    string    `json:"alertSetup"`
	Changes       []string  `json:"changes"`
}

type RiskLogic struct {
	StopLossMethod   string  `json:"stopLossMethod"`
	TakeProfitMethod string  `json:"takeProfitMethod"`
	PlannedRR        float64 `json:"plannedRR"`
}

// ---------- Agente 4: Financial Judge ----------

type FinancialJudgeOutput struct {
	Decision        string           `json:"decision"` // APPROVED | REJECTED | CHANGES_REQUESTED
	Score           int              `json:"score"`
	Summary         string           `json:"summary"`
	Reasons         []JudgeReason    `json:"reasons"`
	RequiredChanges []RequiredChange `json:"requiredChanges"`
}

type JudgeReason struct {
	Metric  string `json:"metric"`
	Value   string `json:"value"`
	Impact  string `json:"impact"` // POSITIVE | NEGATIVE | NEUTRAL
	Comment string `json:"comment"`
}

type RequiredChange struct {
	TargetAgent string `json:"targetAgent"` // CONDITION_DESIGNER | TRIGGER_COMPOSER | PINE_AUTHOR
	Change      string `json:"change"`
}

// ---------- Entrada del agente 4 (calculada en Go, nunca por el LLM) ----------

// Trade de la lista de operaciones del Strategy Tester (export CSV) o de un backtest propio
type Trade struct {
	EntryPrice float64 `json:"entryPrice"`
	ExitPrice  float64 `json:"exitPrice"`
	StopLoss   float64 `json:"stopLoss"` // necesario para medir en R
	IsLong     bool    `json:"isLong"`
	BarsHeld   int     `json:"barsHeld"`
	ExitByTime bool    `json:"exitByTime"`
	FeesPct    float64 `json:"feesPct"`
}

type BacktestMetrics struct {
	Trades             int     `json:"trades"`
	WinRate            float64 `json:"winRate"`
	ProfitFactor       float64 `json:"profitFactor"`
	ExpectancyR        float64 `json:"expectancyR"`
	AvgWinR            float64 `json:"avgWinR"`
	AvgLossR           float64 `json:"avgLossR"`
	MaxDrawdownPct     float64 `json:"maxDrawdownPct"`
	CAGRPct            float64 `json:"cagrPct"`
	BuyAndHoldCAGRPct  float64 `json:"buyAndHoldCagrPct"`
	AvgBarsHeld        float64 `json:"avgBarsHeld"`
	PctExitsByTime     float64 `json:"pctExitsByTime"`
	TopTradesProfitPct float64 `json:"topTradesProfitPct"` // % del beneficio que viene del 10% mejor de trades
}

// Thresholds limites duros del usuario. Se guardan en catalogs o en config
type Thresholds struct {
	MinTrades          int     `json:"minTrades"`
	MinProfitFactor    float64 `json:"minProfitFactor"`
	MinExpectancyR     float64 `json:"minExpectancyR"`
	MaxDrawdownPct     float64 `json:"maxDrawdownPct"`
	MinOOSExpectancyR  float64 `json:"minOosExpectancyR"`
	MaxTopTradesProfit float64 `json:"maxTopTradesProfitPct"`
}

func DefaultThresholds() Thresholds {
	return Thresholds{
		MinTrades:          30,
		MinProfitFactor:    1.3,
		MinExpectancyR:     0.15,
		MaxDrawdownPct:     25,
		MinOOSExpectancyR:  0.05,
		MaxTopTradesProfit: 60,
	}
}

type GateResult struct {
	Name      string  `json:"name"`
	Threshold float64 `json:"threshold"`
	Value     float64 `json:"value"`
	Passed    bool    `json:"passed"`
}

// JudgeCondition datos de una condicion del trigger que ve el juez (sacados de la base de datos, no del LLM)
type JudgeCondition struct {
	IdCondition        string         `json:"idCondition"`
	Name               string         `json:"name"`
	CallbackFunction   string         `json:"callbackFunction"`
	ParametersCallback map[string]any `json:"parametersCallback"`
	IntervalSeconds    int            `json:"intervalSeconds"`
	DirectionCheck     string         `json:"directionCheck"`
	Rationale          string         `json:"rationale"`
}

type JudgeInput struct {
	Asset       string                `json:"asset"`
	Thesis      string                `json:"thesis"`
	Action      string                `json:"action"`
	Trigger     TriggerComposerOutput `json:"trigger"`
	Conditions  []JudgeCondition      `json:"conditions"`
	Pine        PineAuthorOutput      `json:"pine"`
	Metrics     BacktestMetrics       `json:"metrics"`
	OutOfSample BacktestMetrics       `json:"outOfSample"`
	Gates       []GateResult          `json:"gates"`
	Thresholds  Thresholds            `json:"thresholds"`
}
