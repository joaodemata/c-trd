package providers

import (
	cmm "c_trd/common"
	"encoding/json"
	"strconv"
)

// TODO: seria bueno crear una coleccion que guarde las respuestas de las logicas de los callbacks y acciones de los webhooks
func RealtimePutCallRatioCallback(responseData []byte) (bool, *cmm.ErrorHandler) {
	// 1. Definir un struct local para extraer exactamente lo que necesitamos del JSON
	type PCRResponse struct {
		Symbol                string `json:"symbol"`
		PutCallRatioFullChain string `json:"put_call_ratio_full_chain"`
	}

	// 2. Deserializar el JSON desde el []byte
	var data PCRResponse
	if err := json.Unmarshal(responseData, &data); err != nil {
		return false, cmm.NewErrorHandler(err, "Error parsing response data", cmm.LevelFatal, "PCALVE001")
	}

	// 3. Convertir el ratio de string a float64
	ratio, err := strconv.ParseFloat(data.PutCallRatioFullChain, 64)
	if err != nil {
		return false, cmm.NewErrorHandler(err, "Error parsing put call ratio number", cmm.LevelFatal, "PCALVE002")
	}

	// 4. Aplicar la lógica de negocio: > 1 o <= 0.6 indica Bullish
	isBullish := false
	if ratio > 1.0 || ratio <= 0.6 {
		isBullish = true
	}

	// return

	return isBullish, cmm.NewEmptyErrorHandler()
}
