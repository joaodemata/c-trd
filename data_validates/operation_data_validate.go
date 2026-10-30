package data_validates

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

// GoogleTrendsUploadDataValidate valida que el asset exista y que el CSV sea un export valido de Google Trends
func GoogleTrendsUploadDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.GoogleTrendsUploadFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR001", nil, err)
			return
		}

		file, err := cmm.GetFromRequestContext[*cmm.UploadedFile](r, "file")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR004", nil, err)
			return
		}

		asset, ok := validateAssetExists(res, payload.IdAsset)
		if !ok {
			return
		}

		// Se parsea antes de guardar para no reemplazar un archivo valido por uno invalido
		series, err := services.ParseGoogleTrendsCSVService(file.Content, time.Now().UTC())
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, cmm.NewEmptyErrorHandler())
			return
		}

		r = cmm.SetInRequestContext(r, "idAsset", asset.ID)
		r = cmm.SetInRequestContext(r, "trendsSeries", &series)
		next.ServeHTTP(w, r)
	})
}

// GoogleTrendsSeriesDataValidate valida que el asset exista
func GoogleTrendsSeriesDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.GoogleTrendsSeriesFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR002", nil, err)
			return
		}

		asset, ok := validateAssetExists(res, payload.IdAsset)
		if !ok {
			return
		}

		r = cmm.SetInRequestContext(r, "idAsset", asset.ID)
		next.ServeHTTP(w, r)
	})
}

// CreateTriggerDataValidate valida que el tag no este en uso y que el asset, la accion, la temporalidad
// y las condiciones existan. Deja en el contexto los documentos que el controller copia al trigger
func CreateTriggerDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.CreateTriggerFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR005", nil, err)
			return
		}

		// El tag es unico entre los triggers no borrados
		existing, err := services.GetTriggerByTagService(payload.Tag)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if !existing.ID.IsZero() {
			res.Error("FAIL", "Ya existe un trigger con el tag enviado", "DVOPR006", payload.Tag, cmm.NewEmptyErrorHandler())
			return
		}

		references, ok := validateTriggerReferences(res, payload.IdAsset, payload.IdAction, payload.IdCandlestickTimeframe, payload.Conditions)
		if !ok {
			return
		}

		r = setTriggerReferencesInContext(r, references)
		next.ServeHTTP(w, r)
	})
}

// UpdateTriggerDataValidate valida que el trigger a reemplazar exista, que el tag no lo use otro trigger y que el
// asset, la accion, la temporalidad y las condiciones existan. Deja en el contexto el trigger anterior y los
// documentos que el controller copia al trigger nuevo
func UpdateTriggerDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.UpdateTriggerFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR017", nil, err)
			return
		}

		idTrigger, err := cmm.ParseHexToObjectId(payload.IdTrigger)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		previousTrigger, err := services.GetTriggerService(idTrigger)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if previousTrigger.ID.IsZero() {
			res.Error("FAIL", "El trigger enviado no existe", "DVOPR018", payload.IdTrigger, cmm.NewEmptyErrorHandler())
			return
		}

		// El tag puede ser el mismo del trigger anterior, que se borra en la misma transaccion
		existing, err := services.GetTriggerByTagService(payload.Tag)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if !existing.ID.IsZero() && existing.ID != previousTrigger.ID {
			res.Error("FAIL", "Ya existe un trigger con el tag enviado", "DVOPR019", payload.Tag, cmm.NewEmptyErrorHandler())
			return
		}

		references, ok := validateTriggerReferences(res, payload.IdAsset, payload.IdAction, payload.IdCandlestickTimeframe, payload.Conditions)
		if !ok {
			return
		}

		r = cmm.SetInRequestContext(r, "previousTrigger", &previousTrigger)
		r = setTriggerReferencesInContext(r, references)
		next.ServeHTTP(w, r)
	})
}

// triggerReferences documentos que el controller copia al trigger
type triggerReferences struct {
	asset      models.AssetsModelType
	action     models.CatalogsModelType
	timeframe  models.CatalogsModelType
	conditions []models.ConditionsModelType
}

// validateTriggerReferences valida que el asset, la accion, la temporalidad y las condiciones del trigger existan.
// Retorna false si ya se respondio
func validateTriggerReferences(res *cmm.ResponseHandler, idAsset string, idAction string, idTimeframe string, idConditions []string) (triggerReferences, bool) {
	var references triggerReferences

	asset, ok := validateAssetExists(res, idAsset)
	if !ok {
		return references, false
	}

	action, ok := findCatalog(res, idAction, cmm.CatalogTypeAction)
	if !ok {
		return references, false
	}
	if action.ID.IsZero() {
		res.Error("FAIL", "La accion enviada no existe", "DVOPR007", idAction, cmm.NewEmptyErrorHandler())
		return references, false
	}

	timeframe, ok := findCatalog(res, idTimeframe, cmm.CatalogTypeCandleStick)
	if !ok {
		return references, false
	}
	if timeframe.ID.IsZero() {
		res.Error("FAIL", "La temporalidad enviada no existe", "DVOPR008", idTimeframe, cmm.NewEmptyErrorHandler())
		return references, false
	}

	// Sin una duracion conocida no se puede calcular la expiracion de las oportunidades
	if !cmm.IsKnownTimeframeTag(timeframe.Tag) {
		res.Error("FAIL", "La temporalidad enviada no tiene una duracion de vela valida", "DVOPR009", timeframe.Tag, cmm.NewEmptyErrorHandler())
		return references, false
	}

	conditions, ok := validateConditionsExist(res, idConditions)
	if !ok {
		return references, false
	}

	references = triggerReferences{asset: asset, action: action, timeframe: timeframe, conditions: conditions}
	return references, true
}

// setTriggerReferencesInContext deja en el contexto los documentos que el controller copia al trigger
func setTriggerReferencesInContext(r *http.Request, references triggerReferences) *http.Request {
	r = cmm.SetInRequestContext(r, "asset", &references.asset)
	r = cmm.SetInRequestContext(r, "action", &references.action)
	r = cmm.SetInRequestContext(r, "timeframe", &references.timeframe)
	r = cmm.SetInRequestContext(r, "conditions", references.conditions)
	return r
}

// findCatalog busca el catalogo del tipo indicado y responde los errores de parseo o de base de datos. Si no existe retorna
// el catalogo vacio sin responder, cada llamador responde con su propio tracking code. Retorna false si ya se respondio
func findCatalog(res *cmm.ResponseHandler, idCatalogHex string, typeCatalog string) (models.CatalogsModelType, bool) {
	idCatalog, err := cmm.ParseHexToObjectId(idCatalogHex)
	if !err.IsEmtpy() {
		res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
		return models.CatalogsModelType{}, false
	}

	catalog, err := services.GetCatalogService(idCatalog, typeCatalog)
	if !err.IsEmtpy() {
		res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
		return catalog, false
	}

	return catalog, true
}

// validateConditionsExist busca las condiciones y responde el error si alguna no existe. Se retornan en el
// mismo orden en que fueron enviadas. Retorna false si ya se respondio
func validateConditionsExist(res *cmm.ResponseHandler, idConditionsHex []string) ([]models.ConditionsModelType, bool) {
	ids := make([]primitive.ObjectID, 0, len(idConditionsHex))
	for _, idHex := range idConditionsHex {
		id, err := cmm.ParseHexToObjectId(idHex)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return nil, false
		}
		ids = append(ids, id)
	}

	found, err := services.GetConditionsByIDsService(ids)
	if !err.IsEmtpy() {
		res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
		return nil, false
	}

	byID := make(map[primitive.ObjectID]models.ConditionsModelType, len(found))
	for _, condition := range found {
		byID[condition.ID] = condition
	}

	conditions := make([]models.ConditionsModelType, 0, len(ids))
	missing := []string{}
	for _, id := range ids {
		condition, exists := byID[id]
		if !exists {
			missing = append(missing, id.Hex())
			continue
		}
		conditions = append(conditions, condition)
	}

	if len(missing) > 0 {
		res.Error("FAIL", "Las condiciones enviadas no existen", "DVOPR010", missing, cmm.NewEmptyErrorHandler())
		return nil, false
	}

	return conditions, true
}

// validateAssetExists busca el asset y responde el error si no existe. Retorna false si ya se respondio
func validateAssetExists(res *cmm.ResponseHandler, idAssetHex string) (models.AssetsModelType, bool) {
	idAsset, err := cmm.ParseHexToObjectId(idAssetHex)
	if !err.IsEmtpy() {
		res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
		return models.AssetsModelType{}, false
	}

	asset, err := services.GetAssetService(idAsset)
	if !err.IsEmtpy() {
		res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
		return asset, false
	}

	if asset.ID.IsZero() {
		res.Error("FAIL", "El asset enviado no existe", "DVOPR003", idAssetHex, cmm.NewEmptyErrorHandler())
		return asset, false
	}

	return asset, true
}

// CreateConditionDataValidate valida que la API este activa, que su proveedor exista y que el callback lea el formato
// de respuesta de la API. Construye la peticion con los parametros enviados y valida que pueda enviarse al proveedor.
// Deja en el contexto el tipo de condicion, la API, el proveedor, la peticion y los parametros del callback
func CreateConditionDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.CreateConditionFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR011", nil, err)
			return
		}

		// El tag es unico entre las condiciones no borradas
		existing, err := services.GetConditionByTagService(payload.Tag)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if !existing.ID.IsZero() {
			res.Error("FAIL", "Ya existe una condicion con el tag enviado", "DVOPR020", payload.Tag, cmm.NewEmptyErrorHandler())
			return
		}

		idTypeCondition, err := cmm.ParseHexToObjectId(payload.IdTypeCondition)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		typeCondition, err := services.GetCatalogService(idTypeCondition, cmm.CatalogTypeCondition)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if typeCondition.ID.IsZero() {
			res.Error("FAIL", "El tipo de condicion enviado no existe", "DVOPR021", payload.IdTypeCondition, cmm.NewEmptyErrorHandler())
			return
		}

		idApi, err := cmm.ParseHexToObjectId(payload.IdApi)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		api, err := services.GetActiveApiService(idApi)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if api.ID.IsZero() {
			res.Error("FAIL", "La API enviada no existe o no esta activa", "DVOPR012", payload.IdApi, cmm.NewEmptyErrorHandler())
			return
		}

		providersByID, err := services.GetProvidersByIDsService([]primitive.ObjectID{api.IDProvider})
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		provider, exists := providersByID[api.IDProvider]
		if !exists {
			res.Error("FAIL", "El proveedor de la API no existe", "DVOPR013", api.IDProvider.Hex(), cmm.NewEmptyErrorHandler())
			return
		}

		callback, exists := providers.FunctionRegistry[payload.CallbackFunction]
		if !exists {
			res.Error("FAIL", "El callback enviado no existe", "DVOPR015", payload.CallbackFunction, cmm.NewEmptyErrorHandler())
			return
		}
		if callback.ResponseFormat != api.ResponseFormat {
			res.Error("FAIL", "El callback enviado no procesa el formato de respuesta de la API", "DVOPR016", map[string]string{
				"callbackFormat": callback.ResponseFormat,
				"apiFormat":      api.ResponseFormat,
			}, cmm.NewEmptyErrorHandler())
			return
		}

		parametersCallback, err := services.BuildConditionCallbackParametersService(callback, payload.ParametersCallback)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		httpRequest, err := services.BuildConditionHTTPRequestService(api, payload.Parameters)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		credentials := providers.RequestCredentials{SecretRef: provider.SecretRef, AllowedHosts: provider.AllowedHosts}
		if err := providers.ValidateRequestTarget(&httpRequest, credentials); !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		r = cmm.SetInRequestContext(r, "typeCondition", &typeCondition)
		r = cmm.SetInRequestContext(r, "parametersCallback", parametersCallback)
		r = cmm.SetInRequestContext(r, "api", &api)
		r = cmm.SetInRequestContext(r, "provider", &provider)
		r = cmm.SetInRequestContext(r, "httpRequest", &httpRequest)
		next.ServeHTTP(w, r)
	})
}

// VisibilityDataValidate valida que la visibilidad exista en el catalogo TYPE_VISIBILITY, que allowedUsers corresponda
// a la visibilidad (solo la lista blanca tiene usuarios) y que esos usuarios existan y esten activos. Va despues de
// fv.VisibilityFormatValidate y deja la visibilidad en el contexto (visibility)
func VisibilityDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.VisibilityFormat](r, "visibilityPayload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR024", nil, err)
			return
		}

		idTypeVisibility, err := cmm.ParseHexToObjectId(payload.IdTypeVisibility)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		catalog, err := services.GetCatalogService(idTypeVisibility, cmm.CatalogTypeVisibility)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if catalog.ID.IsZero() {
			res.Error("FAIL", "La visibilidad enviada no existe", "DVOPR022", payload.IdTypeVisibility, cmm.NewEmptyErrorHandler())
			return
		}

		allowedUsers, err := cmm.ParseHexListToObjectIds(payload.AllowedUsers)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		visibility, err := models.NewRecordVisibilityData(catalog, allowedUsers)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, catalog.Tag, err)
			return
		}

		users, err := services.GetActiveUsersByIDsService(allowedUsers)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		// allowedUsers no tiene repetidos (format validate), si falta alguno no existe o esta inactivo
		missing := models.MissingUsers(allowedUsers, users)
		if len(missing) > 0 {
			res.Error("FAIL", "Los usuarios de allowedUsers no existen o estan inactivos", "DVOPR023", missing, cmm.NewEmptyErrorHandler())
			return
		}

		r = cmm.SetInRequestContext(r, "visibility", &visibility)
		next.ServeHTTP(w, r)
	})
}
