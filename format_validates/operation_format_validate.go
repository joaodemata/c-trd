package format_validates

import (
	"c_trd/common"
)

// Definimos los format validates co la estructura

// GoogleTrendsUploadFormat campos del multipart que acompañan al CSV exportado de Google Trends
type GoogleTrendsUploadFormat struct {
	IdAsset string `form:"idAsset" validate:"required,mongodb"`
}

// GoogleTrendsSeriesFormat asset del que se quiere leer la serie del CSV
type GoogleTrendsSeriesFormat struct {
	IdAsset string `json:"idAsset" validate:"required,mongodb"`
}

// CreateTriggerFormat datos para registrar un trigger. Conditions es opcional y no puede repetir ids. Code es opcional.
// La visibilidad se valida aparte con VisibilityFormatValidate
type CreateTriggerFormat struct {
	Tag                    string   `json:"tag" validate:"required"`
	Name                   string   `json:"name" validate:"required"`
	Description            string   `json:"description" validate:"required"`
	IdAsset                string   `json:"idAsset" validate:"required,mongodb"`
	IdAction               string   `json:"idAction" validate:"required,mongodb"`
	Conditions             []string `json:"conditions" validate:"omitempty,unique,dive,mongodb"`
	IdCandlestickTimeframe string   `json:"idCandlestickTimeframe" validate:"required,mongodb"`
	MaxCandlestickQty      int      `json:"maxCandlestickQty" validate:"required,gt=0"`
	// Codigo que dispara el trigger en el sistema externo (ej: Pine de TradingView). Opcional, maximo 64 KB
	Code *string `json:"code" validate:"omitempty,max=65536"`
}

var CreateTriggerFormatValidate = common.FormatValidateMiddleware[CreateTriggerFormat]

// UpdateTriggerFormat trigger a reemplazar y todos los datos del trigger nuevo (mismas reglas que CreateTriggerFormat).
// Si code no se envia se conserva el del trigger anterior.
// Los campos no se embeben porque el validador reportaria los errores como CreateTriggerFormat.<campo>
type UpdateTriggerFormat struct {
	IdTrigger              string   `json:"idTrigger" validate:"required,mongodb"`
	Tag                    string   `json:"tag" validate:"required"`
	Name                   string   `json:"name" validate:"required"`
	Description            string   `json:"description" validate:"required"`
	IdAsset                string   `json:"idAsset" validate:"required,mongodb"`
	IdAction               string   `json:"idAction" validate:"required,mongodb"`
	Conditions             []string `json:"conditions" validate:"omitempty,unique,dive,mongodb"`
	IdCandlestickTimeframe string   `json:"idCandlestickTimeframe" validate:"required,mongodb"`
	MaxCandlestickQty      int      `json:"maxCandlestickQty" validate:"required,gt=0"`
	// Codigo que dispara el trigger en el sistema externo (ej: Pine de TradingView). Opcional, maximo 64 KB
	Code *string `json:"code" validate:"omitempty,max=65536"`
}

var UpdateTriggerFormatValidate = common.FormatValidateMiddleware[UpdateTriggerFormat]

// CreateConditionFormat datos para registrar una condicion sobre una API. Los valores de los parametros se envian
// como texto y se validan contra el tipo de dato de la API en el data validate. Solo se crean condiciones que evalua
// el cronjob: isWebhookOnly es opcional y si se envia debe ser false. El intervalo minimo es el del ticker (60s).
// parametersCallback sigue las mismas reglas que parameters pero contra los parametros del callback.
// La visibilidad se valida aparte con VisibilityFormatValidate
type CreateConditionFormat struct {
	Tag                string            `json:"tag" validate:"required"`
	Name               string            `json:"name" validate:"required"`
	Description        string            `json:"description" validate:"required"`
	IdTypeCondition    string            `json:"idTypeCondition" validate:"required,mongodb"`
	IdApi              string            `json:"idApi" validate:"required,mongodb"`
	IsWebhookOnly      bool              `json:"isWebhookOnly" validate:"isdefault"`
	Parameters         map[string]string `json:"parameters"`
	CallbackFunction   string            `json:"callbackFunction" validate:"required"`
	ParametersCallback map[string]string `json:"parametersCallback"`
	IntervalSeconds    int               `json:"intervalSeconds" validate:"required,min=60"`
}

var CreateConditionFormatValidate = common.FormatValidateMiddleware[CreateConditionFormat]

// VisibilityFormat visibilidad (catalogo TYPE_VISIBILITY) de un trigger o condicion, viene en el mismo body.
// allowedUsers solo se envia si la visibilidad es lista blanca
type VisibilityFormat struct {
	IdTypeVisibility string   `json:"idTypeVisibility" validate:"required,mongodb"`
	AllowedUsers     []string `json:"allowedUsers" validate:"omitempty,unique,dive,mongodb"`
}

// Se inyecta en "visibilityPayload" porque "payload" lo ocupa el format validate de la ruta
var VisibilityFormatValidate = common.FormatValidateMiddlewareWithKey[VisibilityFormat]("visibilityPayload")

// El CSV llega en el campo "file" (maximo 1 MB)
var GoogleTrendsUploadFormatValidate = common.FileFormatValidateMiddleware[GoogleTrendsUploadFormat](common.FileUploadConfig{
	Field:      "file",
	MaxBytes:   1 << 20,
	Extensions: []string{".csv"},
})

var GoogleTrendsSeriesFormatValidate = common.FormatValidateMiddleware[GoogleTrendsSeriesFormat]

// ---------- Agentes ----------

// CreateAgentThesisFormat tesis que el super usuario deja en la cola de los agentes (asset, accion e idea)
type CreateAgentThesisFormat struct {
	IdAsset  string `json:"idAsset" validate:"required,mongodb"`
	IdAction string `json:"idAction" validate:"required,mongodb"`
	Thesis   string `json:"thesis" validate:"required,min=20,max=2000"`
}

var CreateAgentThesisFormatValidate = common.FormatValidateMiddleware[CreateAgentThesisFormat]

// DeleteAgentThesisFormat tesis pendiente a sacar de la cola
type DeleteAgentThesisFormat struct {
	IdThesis string `json:"idThesis" validate:"required,mongodb"`
}

var DeleteAgentThesisFormatValidate = common.FormatValidateMiddleware[DeleteAgentThesisFormat]

// AgentRunStepFormat un intento de un agente. output es la salida JSON del agente tal cual
type AgentRunStepFormat struct {
	Agent    string         `json:"agent" validate:"required,oneof=CONDITION_DESIGNER TRIGGER_COMPOSER PINE_AUTHOR FINANCIAL_JUDGE"`
	Attempt  int            `json:"attempt" validate:"required,min=1,max=10"`
	Output   map[string]any `json:"output"`
	Problems []string       `json:"problems" validate:"omitempty,max=100,dive,max=4000"`
}

// FinishAgentRunFormat resultado con el que el runner cierra la corrida de una tesis. requeue devuelve la tesis a la
// cola (ej: se acabo el uso del plan de Claude) en vez de dejarla procesada
type FinishAgentRunFormat struct {
	IdRun      string               `json:"idRun" validate:"required,mongodb"`
	TagStatus  string               `json:"tagStatus" validate:"required,oneof=RUN-SUCCESS RUN-NEEDS-INPUT RUN-FAILED"`
	Steps      []AgentRunStepFormat `json:"steps" validate:"omitempty,max=50,dive"`
	Conditions []string             `json:"conditions" validate:"omitempty,unique,dive,mongodb"`
	IdTrigger  string               `json:"idTrigger" validate:"omitempty,mongodb"`
	Error      string               `json:"error" validate:"omitempty,max=4000"`
	Requeue    bool                 `json:"requeue"`
}

var FinishAgentRunFormatValidate = common.FormatValidateMiddleware[FinishAgentRunFormat]

// Decisiones del super usuario sobre lo que queda pendiente de aprobacion
const (
	ReviewDecisionApprove = "APPROVE"
	ReviewDecisionReject  = "REJECT"
)

// ReviewConditionFormat aprobacion o rechazo de una condicion pendiente de aprobacion
type ReviewConditionFormat struct {
	IdCondition string `json:"idCondition" validate:"required,mongodb"`
	Decision    string `json:"decision" validate:"required,oneof=APPROVE REJECT"`
}

var ReviewConditionFormatValidate = common.FormatValidateMiddleware[ReviewConditionFormat]

// ReviewTriggerFormat aprobacion o rechazo de un trigger pendiente de aprobacion
type ReviewTriggerFormat struct {
	IdTrigger string `json:"idTrigger" validate:"required,mongodb"`
	Decision  string `json:"decision" validate:"required,oneof=APPROVE REJECT"`
}

var ReviewTriggerFormatValidate = common.FormatValidateMiddleware[ReviewTriggerFormat]
