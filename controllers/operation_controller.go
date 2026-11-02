package controllers

import (
	fv "c_trd/format_validates"
	"c_trd/models"
	"c_trd/providers"
	"c_trd/services"
	"net/http"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func CreateOportunityController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Solo peticiones POST entran aquí"))
	}
}

// CreateTriggerController registra un trigger nuevo con los datos ya validados en el data validate. Guarda el usuario
// de la sesion (copiado en la sesion) y la visibilidad validada
func CreateTriggerController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.CreateTriggerFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		asset, err := cmm.GetFromRequestContext[*models.AssetsModelType](r, "asset")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		action, err := cmm.GetFromRequestContext[*models.CatalogsModelType](r, "action")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		timeframe, err := cmm.GetFromRequestContext[*models.CatalogsModelType](r, "timeframe")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		conditions, err := cmm.GetFromRequestContext[[]models.ConditionsModelType](r, "conditions")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		authSession, err := cmm.GetFromRequestContext[*models.SessionsModelType](r, "authSession")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		visibility, err := cmm.GetFromRequestContext[*models.RecordVisibilityData](r, "visibility")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		trigger, err := models.NewTriggerModel(
			payload.Tag, payload.Name, payload.Description,
			asset.ID, asset.Tag, asset.Name,
			action.ID, action.Tag, action.Name,
			payload.MaxCandlestickQty, timeframe.ID, timeframe.Tag, timeframe.Name,
			authSession.RecordOwnerData, *visibility,
			&models.TriggerOptionalData{Conditions: conditions, PineCode: payload.Code},
		)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		idTrigger, err := services.CreateTriggerService(trigger)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		data := map[string]any{
			"idTrigger": idTrigger,
			"idWebhook": trigger.IDWebhook.Hex(),
		}

		res.Send(http.StatusCreated, "SUCCESS", "Trigger creado exitosamente", "COPR003", data)
	}
}

// UpdateTriggerController reemplaza un trigger: el anterior queda borrado e inactivo y el nuevo apunta a el con
// id_reference y a la raiz de la cadena con id_reference_root. El id_webhook se conserva para no cambiar las alertas. Los datos ya fueron validados en el data validate
func UpdateTriggerController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.UpdateTriggerFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		previousTrigger, err := cmm.GetFromRequestContext[*models.TriggersModelType](r, "previousTrigger")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		asset, err := cmm.GetFromRequestContext[*models.AssetsModelType](r, "asset")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		action, err := cmm.GetFromRequestContext[*models.CatalogsModelType](r, "action")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		timeframe, err := cmm.GetFromRequestContext[*models.CatalogsModelType](r, "timeframe")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		conditions, err := cmm.GetFromRequestContext[[]models.ConditionsModelType](r, "conditions")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		idReference := previousTrigger.ID
		idReferenceRoot := models.TriggerReferenceRoot(*previousTrigger)
		idWebhook := models.TriggerWebhookID(*previousTrigger)

		// El usuario y la visibilidad no se envian en la peticion, se conservan los del trigger anterior.
		// El codigo se conserva solo si no se envia uno nuevo
		code := previousTrigger.PineCode
		if payload.Code != nil {
			code = payload.Code
		}
		// El super usuario es quien aprueba: si edita un trigger pendiente de un agente, queda activo
		active := cmm.GenericStatusActive
		trigger, err := models.NewTriggerModel(
			payload.Tag, payload.Name, payload.Description,
			asset.ID, asset.Tag, asset.Name,
			action.ID, action.Tag, action.Name,
			payload.MaxCandlestickQty, timeframe.ID, timeframe.Tag, timeframe.Name,
			previousTrigger.RecordOwnerData, previousTrigger.RecordVisibilityData,
			&models.TriggerOptionalData{
				Conditions:      conditions,
				IdWebhook:       &idWebhook,
				IdReference:     &idReference,
				IdReferenceRoot: &idReferenceRoot,
				PineCode:        code,
				Status:          &active,
			},
		)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		idTrigger, err := services.ReplaceTriggerService(previousTrigger.ID, trigger)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		data := map[string]any{
			"idTrigger":         idTrigger,
			"idPreviousTrigger": previousTrigger.ID.Hex(),
			"idReferenceRoot":   idReferenceRoot.Hex(),
			"idWebhook":         idWebhook.Hex(),
		}

		res.Send(http.StatusCreated, "SUCCESS", "Trigger actualizado exitosamente", "COPR005", data)
	}
}

// GoogleTrendsUploadController reemplaza el CSV de Google Trends del asset (ya validado en el data validate)
func GoogleTrendsUploadController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		file, err := cmm.GetFromRequestContext[*cmm.UploadedFile](r, "file")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		idAsset, err := cmm.GetFromRequestContext[primitive.ObjectID](r, "idAsset")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		series, err := cmm.GetFromRequestContext[*providers.GoogleTrendsFileSeries](r, "trendsSeries")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		if err := services.SaveGoogleTrendsFileService(idAsset, file.Content); !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		last := series.Points[len(series.Points)-1]
		data := map[string]any{
			"idAsset":       idAsset.Hex(),
			"keyword":       series.Keyword,
			"points":        len(series.Points),
			"periodSeconds": series.PeriodSeconds,
			"lastDate":      last.Date,
			"lastValue":     last.Value,
		}

		res.Send(http.StatusCreated, "SUCCESS", "CSV de Google Trends guardado", "COPR001", data)
	}
}

// GoogleTrendsSeriesController retorna la serie del CSV del asset. Solo la consumen las condiciones del cronjob
func GoogleTrendsSeriesController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		idAsset, err := cmm.GetFromRequestContext[primitive.ObjectID](r, "idAsset")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		series, err := services.GetGoogleTrendsFileService(idAsset)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusOK, "SUCCESS", "Serie de Google Trends", "COPR002", series)
	}
}

// CreateConditionController registra una condicion nueva con la API, el proveedor y la peticion validados en el data validate.
// Guarda el usuario de la sesion y la visibilidad validada
func CreateConditionController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.CreateConditionFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		typeCondition, err := cmm.GetFromRequestContext[*models.CatalogsModelType](r, "typeCondition")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		parametersCallback, err := cmm.GetFromRequestContext[map[string]any](r, "parametersCallback")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		api, err := cmm.GetFromRequestContext[*models.ApisModelType](r, "api")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		provider, err := cmm.GetFromRequestContext[*models.ProvidersModelType](r, "provider")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		httpRequest, err := cmm.GetFromRequestContext[*providers.HTTPRequestConfig](r, "httpRequest")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		authSession, err := cmm.GetFromRequestContext[*models.SessionsModelType](r, "authSession")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		visibility, err := cmm.GetFromRequestContext[*models.RecordVisibilityData](r, "visibility")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		condition, err := models.NewConditionModel(
			payload.Tag, payload.Name, payload.Description,
			provider.ID, provider.Name,
			models.ConditionTypeData{ID: typeCondition.ID, Tag: typeCondition.Tag, Name: typeCondition.Name},
			api.ID,
			models.ConditionCronjobData{
				HttpRequest:        *httpRequest,
				CallbackFunction:   payload.CallbackFunction,
				ParametersCallback: parametersCallback,
				IntervalSeconds:    payload.IntervalSeconds,
			},
			authSession.RecordOwnerData, *visibility,
		)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		idCondition, err := services.CreateConditionService(condition)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		data := map[string]any{
			"idCondition": idCondition,
		}

		res.Send(http.StatusCreated, "SUCCESS", "Condicion creada exitosamente", "COPR004", data)
	}
}

// ---------- Agentes ----------

// CreateAgentThesisController agrega una tesis al final de la cola de los agentes
func CreateAgentThesisController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.CreateAgentThesisFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		authSession, err := cmm.GetFromRequestContext[*models.SessionsModelType](r, "authSession")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		asset, err := cmm.GetFromRequestContext[*models.AssetsModelType](r, "asset")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		action, err := cmm.GetFromRequestContext[*models.CatalogsModelType](r, "action")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		thesis, err := models.NewAgentThesisModel(
			payload.Thesis,
			asset.ID, asset.Tag, asset.Name,
			action.ID, action.Tag, action.Name,
			authSession.RecordOwnerData,
		)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		idThesis, err := services.CreateAgentThesisService(thesis)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusCreated, "SUCCESS", "Tesis agregada a la cola de los agentes", "COPR006", map[string]any{"idThesis": idThesis})
	}
}

// DeleteAgentThesisController saca de la cola una tesis pendiente (borrado logico)
func DeleteAgentThesisController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		thesis, err := cmm.GetFromRequestContext[*models.AgentThesesModelType](r, "thesis")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		err = services.DeleteAgentThesisService(thesis.ID, time.Now().UTC())
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusOK, "SUCCESS", "Tesis borrada de la cola", "COPR007", map[string]any{"idThesis": thesis.ID.Hex()})
	}
}

// agentRunStart corrida creada y la tesis que debe procesar el runner
type agentRunStart struct {
	IDRun  string                      `json:"idRun"`
	Thesis models.AgentThesesModelType `json:"thesis"`
}

// StartAgentRunController toma las tesis mas antiguas de la cola (maximo cmm.AgentThesesPerRun) y crea una corrida por
// cada una a nombre del usuario agente. Sin tesis pendientes responde la lista vacia
func StartAgentRunController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		authSession, err := cmm.GetFromRequestContext[*models.SessionsModelType](r, "authSession")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		theses, err := services.ClaimAgentThesesService(time.Now().UTC())
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		runs := make([]agentRunStart, 0, len(theses))
		for _, thesis := range theses {
			run, err := models.NewAgentRunModel(thesis, authSession.RecordOwnerData)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}

			idRun, err := services.CreateAgentRunService(run)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}
			runs = append(runs, agentRunStart{IDRun: idRun, Thesis: thesis})
		}

		res.Send(http.StatusOK, "SUCCESS", "Corridas iniciadas", "COPR008", map[string]any{"runs": runs})
	}
}

// FinishAgentRunController cierra la corrida con sus pasos y deja la tesis procesada o de vuelta en la cola
func FinishAgentRunController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		finish, err := cmm.GetFromRequestContext[*services.AgentRunFinish](r, "runFinish")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		err = services.FinishAgentRunService(*finish, time.Now().UTC())
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusOK, "SUCCESS", "Corrida cerrada", "COPR009", map[string]any{"idRun": finish.IDRun.Hex()})
	}
}

// ReviewConditionController aprueba o rechaza una condicion pendiente de aprobacion
func ReviewConditionController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.ReviewConditionFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		condition, err := cmm.GetFromRequestContext[*models.ConditionsModelType](r, "condition")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		approve := payload.Decision == fv.ReviewDecisionApprove
		err = services.ReviewConditionService(condition.ID, approve, time.Now().UTC())
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		data := map[string]any{"idCondition": condition.ID.Hex(), "decision": payload.Decision}
		res.Send(http.StatusOK, "SUCCESS", "Condicion revisada", "COPR010", data)
	}
}

// ReviewTriggerController aprueba o rechaza un trigger pendiente de aprobacion. Al aprobarlo tambien aprueba sus
// condiciones pendientes
func ReviewTriggerController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.ReviewTriggerFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		trigger, err := cmm.GetFromRequestContext[*models.TriggersModelType](r, "trigger")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		idConditions := make([]primitive.ObjectID, 0, len(trigger.Conditions))
		for _, condition := range trigger.Conditions {
			idConditions = append(idConditions, condition.IDCondition)
		}

		approve := payload.Decision == fv.ReviewDecisionApprove
		err = services.ReviewTriggerService(trigger.ID, approve, idConditions, time.Now().UTC())
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		data := map[string]any{"idTrigger": trigger.ID.Hex(), "decision": payload.Decision}
		res.Send(http.StatusOK, "SUCCESS", "Trigger revisado", "COPR011", data)
	}
}
