package format_validates

import (
	"c_trd/common"
)

// Definimos los format validates co la estructura

// GoogleTrendsUploadFormat campos del multipart que acompañan al CSV exportado de Google Trends
type GoogleTrendsUploadFormat struct {
	IdAsset string `form:"idAsset" validate:"required,mongodb"`
}

// GoogleTrendsSeriesFormat asset del que se quiere leer la serie del CSV
type GoogleTrendsSeriesFormat struct {
	IdAsset string `json:"idAsset" validate:"required,mongodb"`
}

// El CSV llega en el campo "file" (maximo 1 MB)
var GoogleTrendsUploadFormatValidate = common.FileFormatValidateMiddleware[GoogleTrendsUploadFormat](common.FileUploadConfig{
	Field:      "file",
	MaxBytes:   1 << 20,
	Extensions: []string{".csv"},
})

var GoogleTrendsSeriesFormatValidate = common.FormatValidateMiddleware[GoogleTrendsSeriesFormat]
