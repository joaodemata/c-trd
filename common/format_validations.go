package common

import "time"

// Definimos los format validates co la estructura

type PaginationFormat struct {
	Limit     int       `json:"limit,omitempty"`
	Page      int       `json:"page,omitempty"`
	Search    string    `json:"search,omitempty"`
	StartDate time.Time `json:"startDate,omitempty"`
	EndDate   time.Time `json:"endDate,omitempty"`
}

var PaginationFormatValidate = FormatValidateMiddleware[PaginationFormat]