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

// ListTriggersFormat filtros propios del listado de triggers, se concatena con cmm.PaginationFormatValidate.
// Sin showReferences solo se listan los triggers activos; con showReferences tambien los inactivos y los reemplazados.
// idReferenceRoot limita el listado a las versiones de un trigger: la raiz y los que la tienen como id_reference_root
type ListTriggersFormat struct {
	TagStatus       string `json:"tagStatus" in:"query" validate:"omitempty,max=64"`
	ShowReferences  bool   `json:"showReferences" in:"query"`
	IdReferenceRoot string `json:"idReferenceRoot" in:"query" validate:"omitempty,mongodb"`
}

// Se inyecta en "filters" porque "payload" lo ocupa la paginacion
var ListTriggersFormatValidate = common.FormatValidateMiddlewareWithKey[ListTriggersFormat]("filters")
