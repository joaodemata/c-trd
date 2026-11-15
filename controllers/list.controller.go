package controllers

import (
	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/models"
	"c_trd/providers"
	"c_trd/services"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Listado de oportunidades
func ListOportunitiesController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Creamos el response
		res := cmm.NewResponseHandler(w)

		// Extraemos el payload
		// payload, ok := r.Context().Value("payload").(*fv.TriggerHookFormat)
		payload, err := cmm.GetFromRequestContext[*cmm.PaginationFormat](r, "payload")

		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE001", nil, err)
			return
		}

		authSession, err := cmm.GetFromRequestContext[*models.SessionsModelType](r, "authSession")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE009", nil, err)
			return
		}

		// Sin sesion (usuario sin registro) se usa el id cero y solo se listan las oportunidades publicas
		idUser := primitive.NilObjectID
		if authSession != nil {
			idUser = authSession.IDUser
		}

		opportunities, page, err := services.GetOpportunitiesService(payload.Limit, payload.Page, payload.Search, payload.StartDate, payload.EndDate, idUser)
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

		// Status permitidos segun el privilegio y el filtro, los arma ListConditionsDataValidate
		idStatuses, err := cmm.GetFromRequestContext[[]primitive.ObjectID](r, "idStatuses")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE014", nil, err)
			return
		}

		conditions, total, err := services.GetConditionsService(payload.Limit, payload.Page, payload.Search, idStatuses)
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

		// Sin idReferenceRoot se listan todos los triggers (ObjectID cero)
		var idReferenceRoot primitive.ObjectID
		if filters.IdReferenceRoot != "" {
			idReferenceRoot, err = cmm.ParseHexToObjectId(filters.IdReferenceRoot)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}
		}

		triggers, total, err := services.GetTriggersService(payload.Limit, payload.Page, payload.Search, payload.StartDate, payload.EndDate, filters.TagStatus, filters.ShowReferences, idReferenceRoot)
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

// ListAgentThesesController lista la cola de tesis de los agentes en orden FIFO
func ListAgentThesesController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*cmm.PaginationFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE010", nil, err)
			return
		}

		filters, err := cmm.GetFromRequestContext[*fv.ListAgentThesesFormat](r, "filters")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE011", nil, err)
			return
		}

		theses, total, err := services.GetAgentThesesService(payload.Limit, payload.Page, payload.Search, filters.TagStatus)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusOK, "SUCCESS", "Listado de tesis.", "CLIS008", map[string]any{"theses": theses, "page": total})
	}
}

// ListAgentRunsController lista las corridas de los agentes, las mas recientes primero, con los pasos de cada agente
func ListAgentRunsController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*cmm.PaginationFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE012", nil, err)
			return
		}

		filters, err := cmm.GetFromRequestContext[*fv.ListAgentRunsFormat](r, "filters")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE013", nil, err)
			return
		}

		var idThesis primitive.ObjectID
		if filters.IdThesis != "" {
			idThesis, err = cmm.ParseHexToObjectId(filters.IdThesis)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}
		}

		runs, total, err := services.GetAgentRunsService(payload.Limit, payload.Page, payload.Search, filters.TagStatus, idThesis)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusOK, "SUCCESS", "Listado de corridas.", "CLIS009", map[string]any{"runs": runs, "page": total})
	}
}

// ListPortfoliosController lista los portfolios del usuario de la sesion con la posicion de cada asset
func ListPortfoliosController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*cmm.PaginationFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE015", nil, err)
			return
		}

		authSession, err := cmm.GetFromRequestContext[*models.SessionsModelType](r, "authSession")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE016", nil, err)
			return
		}

		portfolios, total, err := services.GetPortfoliosService(payload.Limit, payload.Page, payload.Search, payload.StartDate, payload.EndDate, authSession.IDUser)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusOK, "SUCCESS", "Listado de portfolios.", "CLIS010", map[string]any{"portfolios": portfolios, "page": total})
	}
}

// ListPortfolioTransactionsController lista las transacciones de un portfolio del usuario, las ejecutadas mas
// recientemente primero
func ListPortfolioTransactionsController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*cmm.PaginationFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE017", nil, err)
			return
		}

		filters, err := cmm.GetFromRequestContext[*fv.ListPortfolioTransactionsFormat](r, "filters")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE018", nil, err)
			return
		}

		portfolio, err := cmm.GetFromRequestContext[*models.PortfolioModelType](r, "portfolio")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE019", nil, err)
			return
		}

		transactionFilters := services.PortfolioTransactionsFilter{TagTypeTransaction: filters.TagTypeTransaction}
		if filters.IdAsset != "" {
			transactionFilters.IDAsset, err = cmm.ParseHexToObjectId(filters.IdAsset)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}
		}
		if filters.IdOpportunity != "" {
			transactionFilters.IDOpportunity, err = cmm.ParseHexToObjectId(filters.IdOpportunity)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}
		}

		transactions, total, err := services.GetPortfolioTransactionsService(payload.Limit, payload.Page, payload.Search, payload.StartDate, payload.EndDate, portfolio.ID, portfolio.IDUser, transactionFilters)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusOK, "SUCCESS", "Listado de transacciones.", "CLIS011", map[string]any{"transactions": transactions, "page": total})
	}
}

// ListPortfolioRoiController calcula el ROI del portfolio, de cada asset y de cada oportunidad ligada con los precios
// de mercado actuales. No se guarda: cambia con cada precio
func ListPortfolioRoiController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		portfolio, err := cmm.GetFromRequestContext[*models.PortfolioModelType](r, "portfolio")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "CLISE020", nil, err)
			return
		}

		transactions, err := services.GetAllPortfolioTransactionsService(portfolio.ID, portfolio.IDUser)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		// Precio de los assets en cartera y de los ligados a oportunidades (una consulta por asset)
		idAssets := make([]primitive.ObjectID, 0, len(portfolio.Assets))
		idOpportunities := make([]primitive.ObjectID, 0)
		for _, asset := range portfolio.Assets {
			if asset.Quantity > 0 {
				idAssets = append(idAssets, asset.IDAsset)
			}
		}
		for _, transaction := range transactions {
			if transaction.IDOpportunity != nil && transaction.IDAsset != nil {
				idAssets = append(idAssets, *transaction.IDAsset)
				idOpportunities = append(idOpportunities, *transaction.IDOpportunity)
			}
		}

		prices := services.GetAssetsMarketPricesService(r.Context(), idAssets)

		opportunities, err := services.GetVisibleOpportunitiesByIDsService(idOpportunities, portfolio.IDUser)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		roi := services.BuildPortfolioRoiService(*portfolio, transactions, prices, opportunities, time.Now().UTC())

		res.Send(http.StatusOK, "SUCCESS", "ROI del portfolio.", "CLIS012", map[string]any{"roi": roi})
	}
}
