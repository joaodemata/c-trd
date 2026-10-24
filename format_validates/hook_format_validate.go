package format_validates

import (
	"c_trd/common"
)

// Definimos los format validates co la estructura

// RiskManagementFormat objetivo de precio de stop loss o take profit. Se guarda como target_price y probability
type RiskManagementFormat struct {
	Price       float64  `json:"price" validate:"required,gt=0"`
	Probability *float64 `json:"probability,omitempty" validate:"omitempty,gte=0"`
}

type TradingViewTriggerFormat struct {
	IdTrigger string `json:"idTrigger" validate:"required,mongodb"`
	// Precio del asset cuando se disparo la alerta (en TradingView: "price": {{close}}). Se guarda en price_triggered
	Price float64 `json:"price" validate:"required,gt=0"`
	// Objeto opcional que se guarda en la metadata de la oportunidad. Si no es un objeto JSON el Unmarshal falla
	Metadata map[string]any `json:"metadatos,omitempty"`
	// Razon de cancelacion (catalogo CANCEL_REASON). Si viene, la oportunidad en curso o activa del trigger se cancela
	// y el precio se guarda en price_achieved
	IdCancel string `json:"idCancel" validate:"omitempty,mongodb"`
	// Gestion de riesgo opcional, se guarda solo al crear la oportunidad
	StopLoss   []RiskManagementFormat `json:"stopLoss,omitempty" validate:"omitempty,dive"`
	TakeProfit []RiskManagementFormat `json:"takeProfit,omitempty" validate:"omitempty,dive"`
}

var TradingViewTriggerFormatValidate = common.FormatValidateMiddleware[TradingViewTriggerFormat]
