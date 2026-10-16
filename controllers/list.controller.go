package controllers

import (
	cmm "c_trd/common"
	fv "c_trd/format_validates"
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

// ListApisController lista paginadas las APIs activas con sus parametros y su formato de respuesta para crear condiciones
func ListApisController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.ListApisFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE003", nil, err)
			return
		}

		apis, total, err := services.GetActiveApisService(payload.Limit, payload.Page, payload.Search, payload.StartDate, payload.EndDate)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		// Misma estructura que el listado de oportunidades: "page" es el total de documentos para el paginador
		data := map[string]any{
			"apis": apis,
			"page": total,
		}

		res.Send(http.StatusOK, "SUCCESS", "Listado de APIs.", "CLIS002", data)
	}
}

// ListCallbacksController lista los callbacks compatibles con el formato de respuesta de la API seleccionada
func ListCallbacksController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.ListCallbacksFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE002", nil, err)
			return
		}

		callbacks := services.ListCallbacksService(payload.ResponseFormat)
		res.Send(http.StatusOK, "SUCCESS", "Listado de callbacks.", "CLIS003", map[string]any{"callbacks": callbacks})
	}
}
