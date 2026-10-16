package format_validates

import (
	"c_trd/common"
	"encoding/json"
	"time"
)

// Definimos los format validates co la estructura

type ListOpportunitiesFormat struct {
	IdTrigger string `json:"idTrigger" validate:"required,mongodb"`
	// json.RawMessage garantiza que el contenido sea estrictamente un JSON válido.
	// Si mandan un texto suelto sin comillas o un formato roto, el Unmarshal fallará automáticamente.
	Metadata json.RawMessage `json:"metadatos,omitempty"`
}

var ListOpportunitiesFormatValidate = common.FormatValidateMiddleware[TradingViewTriggerFormat]

// ListApisFormat paginacion del listado de APIs, todos los campos vienen por query params.
// Ej: /list/apis?limit=10&page=2&search=trends&startDate=2026-01-01
type ListApisFormat struct {
	Limit     int       `json:"limit" in:"query" validate:"omitempty,min=1,max=100"`
	Page      int       `json:"page" in:"query" validate:"omitempty,min=1"`
	Search    string    `json:"search" in:"query" validate:"omitempty,max=100"`
	StartDate time.Time `json:"startDate" in:"query"`
	EndDate   time.Time `json:"endDate" in:"query"`
}

var ListApisFormatValidate = common.FormatValidateMiddleware[ListApisFormat]

// ListCallbacksFormat formato de respuesta de la API seleccionada (ej: GOOGLE_TRENDS_SERIES)
type ListCallbacksFormat struct {
	ResponseFormat string `json:"responseFormat" validate:"required,max=64,uppersnake" in:"query"`
}

var ListCallbacksFormatValidate = common.FormatValidateMiddleware[ListCallbacksFormat]
