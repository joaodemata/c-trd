package data_validates

import (
	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/models"
	"c_trd/providers"
	"c_trd/services"
	"fmt"
	"net/http"
	"strings"
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

		asset, err := validateAssetExists(payload.IdAsset)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
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

		asset, err := validateAssetExists(payload.IdAsset)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
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

		references, err := validateTriggerReferences(payload.IdAsset, payload.IdAction, payload.IdCandlestickTimeframe, payload.Conditions)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		r = cmm.SetInRequestContext(r, "asset", &references.asset)
		r = cmm.SetInRequestContext(r, "action", &references.action)
		r = cmm.SetInRequestContext(r, "timeframe", &references.timeframe)
		r = cmm.SetInRequestContext(r, "conditions", references.conditions)
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

		references, err := validateTriggerReferences(payload.IdAsset, payload.IdAction, payload.IdCandlestickTimeframe, payload.Conditions)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		r = cmm.SetInRequestContext(r, "previousTrigger", &previousTrigger)
		r = cmm.SetInRequestContext(r, "asset", &references.asset)
		r = cmm.SetInRequestContext(r, "action", &references.action)
		r = cmm.SetInRequestContext(r, "timeframe", &references.timeframe)
		r = cmm.SetInRequestContext(r, "conditions", references.conditions)
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

// validateTriggerReferences valida que el asset, la accion, la temporalidad y las condiciones del trigger existan, que
// ninguna condicion este rechazada y que puedan cumplirse a la vez. El middleware responde el error retornado
func validateTriggerReferences(idAsset string, idAction string, idTimeframe string, idConditions []string) (triggerReferences, *cmm.ErrorHandler) {
	var references triggerReferences

	asset, err := validateAssetExists(idAsset)
	if !err.IsEmtpy() {
		return references, err
	}

	action, err := findCatalog(idAction, cmm.CatalogTypeAction)
	if !err.IsEmtpy() {
		return references, err
	}
	if action.ID.IsZero() {
		return references, cmm.NewErrorHandler(fmt.Errorf("action %s", idAction), fmt.Sprintf("La accion enviada no existe: %s", idAction), cmm.LevelError, "DVOPR007")
	}

	timeframe, err := findCatalog(idTimeframe, cmm.CatalogTypeCandleStick)
	if !err.IsEmtpy() {
		return references, err
	}
	if timeframe.ID.IsZero() {
		return references, cmm.NewErrorHandler(fmt.Errorf("timeframe %s", idTimeframe), fmt.Sprintf("La temporalidad enviada no existe: %s", idTimeframe), cmm.LevelError, "DVOPR008")
	}

	// Sin una duracion conocida no se puede calcular la expiracion de las oportunidades
	if !cmm.IsKnownTimeframeTag(timeframe.Tag) {
		return references, cmm.NewErrorHandler(fmt.Errorf("timeframe tag %s", timeframe.Tag), fmt.Sprintf("La temporalidad enviada no tiene una duracion de vela valida: %s", timeframe.Tag), cmm.LevelError, "DVOPR009")
	}

	conditions, err := validateConditionsExist(idConditions)
	if !err.IsEmtpy() {
		return references, err
	}

	// Una condicion rechazada nunca se evalua: el trigger nunca pasaria a ACTIVE
	notApproved := []string{}
	for _, condition := range conditions {
		if condition.TagStatus == cmm.GenericStatusNotApproved.Tag {
			notApproved = append(notApproved, condition.ID.Hex())
		}
	}
	if len(notApproved) > 0 {
		return references, cmm.NewErrorHandler(fmt.Errorf("not approved %v", notApproved), "Las condiciones enviadas no fueron aprobadas: "+strings.Join(notApproved, ", "), cmm.LevelError, "DVOPR043")
	}

	// Condiciones que no pueden cumplirse a la vez: la oportunidad nunca pasaria a ACTIVE
	incompatible := services.FindIncompatibleConditionsService(conditions)
	if len(incompatible) > 0 {
		return references, cmm.NewErrorHandler(fmt.Errorf("incompatible conditions"), "Las condiciones enviadas no pueden cumplirse a la vez: "+strings.Join(incompatible, "; "), cmm.LevelError, "DVOPR025")
	}

	references = triggerReferences{asset: asset, action: action, timeframe: timeframe, conditions: conditions}
	return references, cmm.NewEmptyErrorHandler()
}

// findCatalog busca el catalogo del tipo indicado. Retorna los errores de parseo o de base de datos; si no existe
// retorna el catalogo vacio sin error y cada middleware responde con su propio tracking code
func findCatalog(idCatalogHex string, typeCatalog string) (models.CatalogsModelType, *cmm.ErrorHandler) {
	idCatalog, err := cmm.ParseHexToObjectId(idCatalogHex)
	if !err.IsEmtpy() {
		return models.CatalogsModelType{}, err
	}

	return services.GetCatalogService(idCatalog, typeCatalog)
}

// validateConditionsExist busca las condiciones y retorna error si alguna no existe. Se retornan en el mismo orden en
// que fueron enviadas
func validateConditionsExist(idConditionsHex []string) ([]models.ConditionsModelType, *cmm.ErrorHandler) {
	ids := make([]primitive.ObjectID, 0, len(idConditionsHex))
	for _, idHex := range idConditionsHex {
		id, err := cmm.ParseHexToObjectId(idHex)
		if !err.IsEmtpy() {
			return []models.ConditionsModelType{}, err
		}
		ids = append(ids, id)
	}

	found, err := services.GetConditionsByIDsService(ids)
	if !err.IsEmtpy() {
		return []models.ConditionsModelType{}, err
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
		return []models.ConditionsModelType{}, cmm.NewErrorHandler(fmt.Errorf("missing conditions %v", missing), "Las condiciones enviadas no existen: "+strings.Join(missing, ", "), cmm.LevelError, "DVOPR010")
	}

	return conditions, cmm.NewEmptyErrorHandler()
}

// validateAssetExists busca el asset. Retorna error si el id no es valido, si falla la base de datos o si no existe
func validateAssetExists(idAssetHex string) (models.AssetsModelType, *cmm.ErrorHandler) {
	idAsset, err := cmm.ParseHexToObjectId(idAssetHex)
	if !err.IsEmtpy() {
		return models.AssetsModelType{}, err
	}

	asset, err := services.GetAssetService(idAsset)
	if !err.IsEmtpy() {
		return asset, err
	}

	if asset.ID.IsZero() {
		return asset, cmm.NewErrorHandler(fmt.Errorf("asset %s", idAssetHex), fmt.Sprintf("El asset enviado no existe: %s", idAssetHex), cmm.LevelError, "DVOPR003")
	}

	return asset, cmm.NewEmptyErrorHandler()
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

// ---------- Agentes ----------

// CreateAgentThesisDataValidate valida que el asset y la accion existan. Deja en el contexto el asset y la accion
func CreateAgentThesisDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.CreateAgentThesisFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR026", nil, err)
			return
		}

		asset, err := validateAssetExists(payload.IdAsset)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		action, err := findCatalog(payload.IdAction, cmm.CatalogTypeAction)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if action.ID.IsZero() {
			res.Error("FAIL", "La accion enviada no existe", "DVOPR027", payload.IdAction, cmm.NewEmptyErrorHandler())
			return
		}

		r = cmm.SetInRequestContext(r, "asset", &asset)
		r = cmm.SetInRequestContext(r, "action", &action)
		next.ServeHTTP(w, r)
	})
}

// DeleteAgentThesisDataValidate valida que la tesis exista y siga pendiente. Deja la tesis en el contexto (thesis)
func DeleteAgentThesisDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.DeleteAgentThesisFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR028", nil, err)
			return
		}

		idThesis, err := cmm.ParseHexToObjectId(payload.IdThesis)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		thesis, err := services.GetAgentThesisService(idThesis)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if thesis.ID.IsZero() {
			res.Error("FAIL", "La tesis enviada no existe", "DVOPR029", payload.IdThesis, cmm.NewEmptyErrorHandler())
			return
		}
		if thesis.TagStatus != cmm.AgentThesisStatusPending.Tag {
			res.Error("FAIL", "Solo se pueden borrar tesis pendientes", "DVOPR030", thesis.TagStatus, cmm.NewEmptyErrorHandler())
			return
		}

		r = cmm.SetInRequestContext(r, "thesis", &thesis)
		next.ServeHTTP(w, r)
	})
}

// agentRunStatusByTag status de cierre de una corrida que acepta FinishAgentRunFormat
var agentRunStatusByTag = map[string]cmm.CatalogStatus{
	cmm.AgentRunStatusSuccess.Tag:    cmm.AgentRunStatusSuccess,
	cmm.AgentRunStatusNeedsInput.Tag: cmm.AgentRunStatusNeedsInput,
	cmm.AgentRunStatusFailed.Tag:     cmm.AgentRunStatusFailed,
}

// FinishAgentRunDataValidate valida que la corrida exista, siga en ejecucion y sea del usuario de la sesion. Deja en el
// contexto el cierre listo para el servicio (runFinish)
func FinishAgentRunDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.FinishAgentRunFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR031", nil, err)
			return
		}

		authSession, err := cmm.GetFromRequestContext[*models.SessionsModelType](r, "authSession")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR032", nil, err)
			return
		}

		idRun, err := cmm.ParseHexToObjectId(payload.IdRun)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		run, err := services.GetAgentRunService(idRun)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if run.ID.IsZero() {
			res.Error("FAIL", "La corrida enviada no existe", "DVOPR033", payload.IdRun, cmm.NewEmptyErrorHandler())
			return
		}
		if run.TagStatus != cmm.AgentRunStatusRunning.Tag {
			res.Error("FAIL", "La corrida ya fue cerrada", "DVOPR034", run.TagStatus, cmm.NewEmptyErrorHandler())
			return
		}
		if run.IDUser != authSession.IDUser {
			res.Error("FAIL", "La corrida pertenece a otro usuario", "DVOPR035", payload.IdRun, cmm.NewEmptyErrorHandler())
			return
		}

		idConditions := make([]primitive.ObjectID, 0, len(payload.Conditions))
		for _, idHex := range payload.Conditions {
			id, err := cmm.ParseHexToObjectId(idHex)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}
			idConditions = append(idConditions, id)
		}

		var idTrigger *primitive.ObjectID
		if payload.IdTrigger != "" {
			id, err := cmm.ParseHexToObjectId(payload.IdTrigger)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}
			idTrigger = &id
		}

		var runError *string
		if payload.Error != "" {
			runError = &payload.Error
		}

		now := time.Now().UTC()
		steps := make([]models.AgentRunStep, 0, len(payload.Steps))
		for _, step := range payload.Steps {
			output := step.Output
			if output == nil {
				output = map[string]any{}
			}
			problems := step.Problems
			if problems == nil {
				problems = []string{}
			}
			steps = append(steps, models.AgentRunStep{Agent: step.Agent, Attempt: step.Attempt, Output: output, Problems: problems, CreatedAt: now})
		}

		finish := services.AgentRunFinish{
			IDRun:        run.ID,
			IDThesis:     run.IDThesis,
			Status:       agentRunStatusByTag[payload.TagStatus],
			Steps:        steps,
			IDConditions: idConditions,
			IDTrigger:    idTrigger,
			Error:        runError,
			Requeue:      payload.Requeue,
		}

		r = cmm.SetInRequestContext(r, "runFinish", &finish)
		next.ServeHTTP(w, r)
	})
}

// ReviewConditionDataValidate valida que la condicion exista y este pendiente de aprobacion. Solo se rechaza si ningun
// trigger la usa (hay que rechazar primero el trigger). Deja la condicion en el contexto (condition)
func ReviewConditionDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.ReviewConditionFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR036", nil, err)
			return
		}

		idCondition, err := cmm.ParseHexToObjectId(payload.IdCondition)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		found, err := services.GetConditionsByIDsService([]primitive.ObjectID{idCondition})
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if len(found) == 0 {
			res.Error("FAIL", "La condicion enviada no existe", "DVOPR037", payload.IdCondition, cmm.NewEmptyErrorHandler())
			return
		}
		condition := found[0]
		if condition.TagStatus != cmm.GenericStatusPendingForApprove.Tag {
			res.Error("FAIL", "La condicion no esta pendiente de aprobacion", "DVOPR038", condition.TagStatus, cmm.NewEmptyErrorHandler())
			return
		}

		if payload.Decision == fv.ReviewDecisionReject {
			triggers, err := services.CountTriggersByConditionService(condition.ID)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}
			if triggers > 0 {
				res.Error("FAIL", "La condicion la usa un trigger, rechace primero el trigger", "DVOPR039", triggers, cmm.NewEmptyErrorHandler())
				return
			}
		}

		r = cmm.SetInRequestContext(r, "condition", &condition)
		next.ServeHTTP(w, r)
	})
}

// ReviewTriggerDataValidate valida que el trigger exista y este pendiente de aprobacion. Deja el trigger en el contexto
// (trigger)
func ReviewTriggerDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.ReviewTriggerFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVOPR040", nil, err)
			return
		}

		idTrigger, err := cmm.ParseHexToObjectId(payload.IdTrigger)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		trigger, err := services.GetTriggerService(idTrigger)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if trigger.ID.IsZero() {
			res.Error("FAIL", "El trigger enviado no existe", "DVOPR041", payload.IdTrigger, cmm.NewEmptyErrorHandler())
			return
		}
		if trigger.TagStatus != cmm.GenericStatusPendingForApprove.Tag {
			res.Error("FAIL", "El trigger no esta pendiente de aprobacion", "DVOPR042", trigger.TagStatus, cmm.NewEmptyErrorHandler())
			return
		}

		// Aprobado con una condicion rechazada nunca pasaria a ACTIVE: hay que rechazarlo o crear otro trigger
		if payload.Decision == fv.ReviewDecisionApprove {
			idConditions := make([]primitive.ObjectID, 0, len(trigger.Conditions))
			for _, condition := range trigger.Conditions {
				idConditions = append(idConditions, condition.IDCondition)
			}

			conditions, err := services.GetConditionsByIDsService(idConditions)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}

			notApproved := []string{}
			for _, condition := range conditions {
				if condition.TagStatus == cmm.GenericStatusNotApproved.Tag {
					notApproved = append(notApproved, condition.ID.Hex())
				}
			}
			if len(notApproved) > 0 {
				res.Error("FAIL", "El trigger usa condiciones no aprobadas", "DVOPR044", notApproved, cmm.NewEmptyErrorHandler())
				return
			}
		}

		r = cmm.SetInRequestContext(r, "trigger", &trigger)
		next.ServeHTTP(w, r)
	})
}
