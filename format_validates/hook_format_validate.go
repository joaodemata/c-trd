package format_validates

import (
	"c_trd/common"
)

// Definimos los format validates co la estructura

type TradingViewTriggerFormat struct {
    Asset     string   `json:"asset" validate:"required,min=3"`
    
    // Usamos punteros para campos dinámicos u opcionales
    // omitempty: Si el valor es nil, ignora la validación. Si trae un dato, asegúrate de que sea > 0 o diferente a ""
    Precio    *float64 `json:"precio,omitempty" validate:"omitempty,gt=0"` 
    Volumen   *float64 `json:"volumen,omitempty" validate:"omitempty,gt=0"`
    RSI       *float64 `json:"rsi,omitempty" validate:"omitempty,min=0,max=100"`
}


var TradingViewTriggerFormatValidate = common.FormatValidateMiddleware[TradingViewTriggerFormat]