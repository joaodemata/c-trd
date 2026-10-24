package common

import "time"

// Definimos los format validates co la estructura

// Paginacion por query params: el listado es un GET y los navegadores no envian body en un GET
type PaginationFormat struct {
	Limit     int       `json:"limit,omitempty" in:"query"`
	Page      int       `json:"page,omitempty" in:"query"`
	Search    string    `json:"search,omitempty" in:"query"`
	StartDate time.Time `json:"startDate,omitempty" in:"query"`
	EndDate   time.Time `json:"endDate,omitempty" in:"query"`
}

var PaginationFormatValidate = FormatValidateMiddleware[PaginationFormat]
