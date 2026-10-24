package controllers

import (
	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/providers"
	"c_trd/services"
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

		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE001", nil, err)
			return
		}

		// Buscamos el trigger en mongoDB para validar si existe
		opportunities, page, err := services.GetOpportunitiesService(payload.Limit, payload.Page, payload.Search, payload.StartDate, payload.EndDate)
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

		payload, err := cmm.GetFromRequestContext[*cmm.PaginationFormat](r, "payload")
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

		callbacks := providers.ListCallbacksByResponseFormat(payload.ResponseFormat)
		res.Send(http.StatusOK, "SUCCESS", "Listado de callbacks.", "CLIS003", map[string]any{"callbacks": callbacks})
	}
}

// ListAssetsController lista paginados los assets para los selectores del frontend
func ListAssetsController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*cmm.PaginationFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE004", nil, err)
			return
		}

		assets, total, err := services.GetAssetsService(payload.Limit, payload.Page, payload.Search)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		// "page" es el total de documentos para el paginador, igual que los demas listados
		data := map[string]any{
			"assets": assets,
			"page":   total,
		}

		res.Send(http.StatusOK, "SUCCESS", "Listado de assets.", "CLIS004", data)
	}
}

// ListCatalogsController lista los catalogos de un type_catalog (ej: acciones o temporalidades de vela)
func ListCatalogsController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.ListCatalogsFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE005", nil, err)
			return
		}

		catalogs, err := services.GetCatalogsByTypeService(payload.TypeCatalog)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusOK, "SUCCESS", "Listado de catalogos.", "CLIS005", map[string]any{"catalogs": catalogs})
	}
}

// ListConditionsController lista paginadas las condiciones para asignarlas a un trigger
func ListConditionsController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*cmm.PaginationFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE006", nil, err)
			return
		}

		conditions, total, err := services.GetConditionsService(payload.Limit, payload.Page, payload.Search)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		data := map[string]any{
			"conditions": conditions,
			"page":       total,
		}

		res.Send(http.StatusOK, "SUCCESS", "Listado de condiciones.", "CLIS006", data)
	}
}

// ListTriggersController lista paginados los triggers con su asset, accion, condiciones y temporalidad. Los inactivos
// y reemplazados solo se listan con showReferences
func ListTriggersController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*cmm.PaginationFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE007", nil, err)
			return
		}

		filters, err := cmm.GetFromRequestContext[*fv.ListTriggersFormat](r, "filters")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE008", nil, err)
			return
		}

		triggers, total, err := services.GetTriggersService(payload.Limit, payload.Page, payload.Search, payload.StartDate, payload.EndDate, filters.TagStatus, filters.ShowReferences)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		data := map[string]any{
			"triggers": triggers,
			"page":     total,
		}

		res.Send(http.StatusOK, "SUCCESS", "Listado de triggers.", "CLIS007", data)
	}
}
