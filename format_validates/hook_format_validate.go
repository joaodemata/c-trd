package format_validates

import (
	"c_trd/common"
	"encoding/json"
)

// Definimos los format validates co la estructura

type TradingViewTriggerFormat struct {
    IdTrigger string   `json:"idTrigger" validate:"required,mongodb"` 
    // json.RawMessage garantiza que el contenido sea estrictamente un JSON válido.
	// Si mandan un texto suelto sin comillas o un formato roto, el Unmarshal fallará automáticamente.
  	Metadata json.RawMessage `json:"metadatos,omitempty"`
}


var TradingViewTriggerFormatValidate = common.FormatValidateMiddleware[TradingViewTriggerFormat]