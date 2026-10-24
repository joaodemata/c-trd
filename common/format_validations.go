package common

import "time"

// Definimos los format validates co la estructura

// Paginacion por query params: el listado es un GET y los navegadores no envian body en un GET.
// Formato general de todos los listados paginados. Ej: /list/apis?limit=10&page=2&search=trends&startDate=2026-01-01
type PaginationFormat struct {
	Limit     int       `json:"limit,omitempty" in:"query" validate:"omitempty,min=1,max=100"`
	Page      int       `json:"page,omitempty" in:"query" validate:"omitempty,min=1"`
	Search    string    `json:"search,omitempty" in:"query" validate:"omitempty,max=100"`
	StartDate time.Time `json:"startDate,omitempty" in:"query"`
	EndDate   time.Time `json:"endDate,omitempty" in:"query"`
}

var PaginationFormatValidate = FormatValidateMiddleware[PaginationFormat]
