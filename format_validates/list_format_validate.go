package format_validates

import (
	"c_trd/common"
	"encoding/json"
)

// Definimos los format validates co la estructura

type ListOpportunitiesFormat struct {
	IdTrigger string `json:"idTrigger" validate:"required,mongodb"`
	// json.RawMessage garantiza que el contenido sea estrictamente un JSON válido.
	// Si mandan un texto suelto sin comillas o un formato roto, el Unmarshal fallará automáticamente.
	Metadata json.RawMessage `json:"metadatos,omitempty"`
}

var ListOpportunitiesFormatValidate = common.FormatValidateMiddleware[TradingViewTriggerFormat]

// ListCallbacksFormat formato de respuesta de la API seleccionada (ej: GOOGLE_TRENDS_SERIES)
type ListCallbacksFormat struct {
	ResponseFormat string `json:"responseFormat" validate:"required,max=64,uppersnake" in:"query"`
}

var ListCallbacksFormatValidate = common.FormatValidateMiddleware[ListCallbacksFormat]

// ListCatalogsFormat tipo de catalogo a listar (type_catalog, ej: TYPE_ACTION o TYPE_CANDLE_STICK)
type ListCatalogsFormat struct {
	TypeCatalog string `json:"typeCatalog" in:"query" validate:"required,max=64,uppersnake"`
}

var ListCatalogsFormatValidate = common.FormatValidateMiddleware[ListCatalogsFormat]
