package controllers

import (
	fv "c_trd/format_validates"
	"c_trd/models"
	"c_trd/providers"
	"c_trd/services"
	"net/http"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func CreateOportunityController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Solo peticiones POST entran aquí"))
	}
}

// CreateTriggerController registra un trigger nuevo con los datos ya validados en el data validate
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

		trigger, err := models.NewTriggerModel(
			payload.Tag, payload.Name, payload.Description,
			asset.ID, asset.Tag, asset.Name,
			action.ID, action.Tag, action.Name,
			payload.MaxCandlestickQty, timeframe.ID, timeframe.Tag, timeframe.Name,
			&models.TriggerOptionalData{Conditions: conditions},
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

		// El pine_code no se envia en la peticion, se conserva el del trigger anterior
		trigger, err := models.NewTriggerModel(
			payload.Tag, payload.Name, payload.Description,
			asset.ID, asset.Tag, asset.Name,
			action.ID, action.Tag, action.Name,
			payload.MaxCandlestickQty, timeframe.ID, timeframe.Tag, timeframe.Name,
			&models.TriggerOptionalData{
				Conditions:      conditions,
				IdWebhook:       &idWebhook,
				IdReference:     &idReference,
				IdReferenceRoot: &idReferenceRoot,
				PineCode:        previousTrigger.PineCode,
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

// CreateConditionController registra una condicion nueva con la API, el proveedor y la peticion validados en el data validate
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
