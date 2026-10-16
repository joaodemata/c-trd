package providers

import (
	cmm "c_trd/common"
	"sort"
)

// CallbackFunc defines the signature for dynamic callback functions.
// It receives the original condition, the HTTP response body, and the status code.
// Todos los callbacks solo deberian recibir un bool
type CallbackFunc func(responseBody []byte) (bool, *cmm.ErrorHandler)

// CallbackDefinition callback registrado y el formato de respuesta que sabe leer. Solo es compatible con
// las APIs que tengan el mismo response_format
type CallbackDefinition struct {
	Fn             CallbackFunc
	Description    string
	ResponseFormat string
}

// CallbackInfo datos publicos de un callback para listarlo
type CallbackInfo struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	ResponseFormat string `json:"responseFormat"`
}

// FunctionRegistry maps a string (from the database) to an actual Go function.
var FunctionRegistry = map[string]CallbackDefinition{
	"ProcessAlphaVantageData": {
		Fn:             RealtimePutCallRatioCallback,
		Description:    "Put/Call ratio mayor a 1 o menor o igual a 0.6",
		ResponseFormat: cmm.ResponseFormatAlphaVantagePutCallRatio,
	},
	"GoogleTrendsHigh": {
		Fn:             GoogleTrendsHighCallback,
		Description:    "El ultimo valor de interes esta en la zona alta del rango de la serie",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
	},
	"GoogleTrendsLow": {
		Fn:             GoogleTrendsLowCallback,
		Description:    "El ultimo valor de interes esta en la zona baja del rango de la serie",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsSeries,
	},
	"GoogleTrendsFileHigh": {
		Fn:             GoogleTrendsFileHighCallback,
		Description:    "El ultimo periodo del CSV esta en la zona alta del rango",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
	},
	"GoogleTrendsFileLow": {
		Fn:             GoogleTrendsFileLowCallback,
		Description:    "El ultimo periodo del CSV esta en la zona baja del rango",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
	},
	"GoogleTrendsFileNewMax": {
		Fn:             GoogleTrendsFileNewMaxCallback,
		Description:    "El ultimo periodo del CSV es un nuevo maximo de la serie",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
	},
	"GoogleTrendsFileNewMin": {
		Fn:             GoogleTrendsFileNewMinCallback,
		Description:    "El ultimo periodo del CSV es un nuevo minimo de la serie",
		ResponseFormat: cmm.ResponseFormatGoogleTrendsFile,
	},
}

// ListCallbacksByResponseFormat retorna los callbacks que leen el formato de respuesta indicado, ordenados por nombre.
// Si ninguno coincide retorna [] y no nil
func ListCallbacksByResponseFormat(responseFormat string) []CallbackInfo {
	result := make([]CallbackInfo, 0)
	for name, definition := range FunctionRegistry {
		if definition.ResponseFormat == responseFormat {
			result = append(result, CallbackInfo{Name: name, Description: definition.Description, ResponseFormat: definition.ResponseFormat})
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
