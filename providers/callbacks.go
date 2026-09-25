package providers

import (
	cmm "c_trd/common"
)

// CallbackFunc defines the signature for dynamic callback functions.
// It receives the original condition, the HTTP response body, and the status code.
// Todos los callbacks solo deberian recibir un bool
type CallbackFunc func(responseBody []byte) (bool, *cmm.ErrorHandler)

// FunctionRegistry maps a string (from the database) to an actual Go function.
var FunctionRegistry = map[string]CallbackFunc{
	"ProcessAlphaVantageData": RealtimePutCallRatioCallback,
}
