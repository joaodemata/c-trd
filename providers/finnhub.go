package providers

import (
	cmm "c_trd/common"
	"encoding/json"
	"fmt"
)

// finnhubQuoteResponse estructura minima de la respuesta de /api/v1/quote. c es el precio actual y t la fecha
// en unix. Un simbolo desconocido o fuera del plan no responde error, responde todos los valores en 0
type finnhubQuoteResponse struct {
	Current   *float64 `json:"c"`
	Timestamp int64    `json:"t"`
}

// FinnhubQuotePrice retorna el precio actual de la respuesta de /api/v1/quote
func FinnhubQuotePrice(responseData []byte) (float64, *cmm.ErrorHandler) {
	var data finnhubQuoteResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return 0, cmm.NewErrorHandler(err, "Error parsing Finnhub quote response", cmm.LevelFatal, "PCFINE001")
	}

	if data.Current == nil || *data.Current <= 0 || data.Timestamp == 0 {
		return 0, cmm.NewErrorHandler(fmt.Errorf("c %v t %d", data.Current, data.Timestamp), "Finnhub has no price for the symbol (unknown symbol or not included in the plan)", cmm.LevelError, "PCFINE002")
	}

	return *data.Current, cmm.NewEmptyErrorHandler()
}
