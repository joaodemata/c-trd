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
		}

		res.Send(http.StatusCreated, "SUCCESS", "Trigger creado exitosamente", "COPR003", data)
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
			provider.ID, provider.Name,
			payload.NameCondition, payload.DescriptionCondition,
			api.ID,
			payload.Metadata,
			models.ConditionCronjobData{
				HttpRequest:      *httpRequest,
				CallbackFunction: payload.CallbackFunction,
				IntervalSeconds:  payload.IntervalSeconds,
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
