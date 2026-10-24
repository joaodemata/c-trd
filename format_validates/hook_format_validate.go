package format_validates

import (
	"c_trd/common"
)

// Definimos los format validates co la estructura

type TradingViewTriggerFormat struct {
	IdTrigger string `json:"idTrigger" validate:"required,mongodb"`
	// Precio del asset cuando se disparo la alerta (en TradingView: "price": {{close}}). Se guarda en price_triggered
	Price float64 `json:"price" validate:"required,gt=0"`
	// Objeto opcional que se guarda en la metadata de la oportunidad. Si no es un objeto JSON el Unmarshal falla
	Metadata map[string]any `json:"metadatos,omitempty"`
}

var TradingViewTriggerFormatValidate = common.FormatValidateMiddleware[TradingViewTriggerFormat]
