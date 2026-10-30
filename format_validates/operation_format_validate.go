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

// CreateTriggerFormat datos para registrar un trigger. Conditions es opcional y no puede repetir ids.
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
}

var CreateTriggerFormatValidate = common.FormatValidateMiddleware[CreateTriggerFormat]

// UpdateTriggerFormat trigger a reemplazar y todos los datos del trigger nuevo (mismas reglas que CreateTriggerFormat).
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
