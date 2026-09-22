package controllers

import (
	cmm "c_trd/common"
	"c_trd/services"
	"fmt"
	"net/http"
)

// Listado de oportunidades
func ListOportunitiesController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Creamos el response
		res := cmm.NewResponseHandler(w)

		// Extraemos el payload
		// payload, ok := r.Context().Value("payload").(*fv.TradingViewTriggerFormat)
		payload, err := cmm.GetFromRequestContext[*cmm.PaginationFormat](r, "payload")

		fmt.Println(err)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE001", nil, err)
			return
		}

		// Buscamos el trigger en mongoDB para validar si existe
		opportunities, page, err := services.GetOpportunitiesService(payload.Limit, payload.Page, payload.Search, payload.StartDate, payload.EndDate)
		fmt.Println(err)
		// Verificamos si hay error de parseo
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		// Response Data
		data := map[string]any{
			"opportunities": opportunities,
			"page":          page,
		}

		res.Send(http.StatusAccepted, "SUCCESS", "Listado de oportunidades.", "CLIS001", data)
	}
}
