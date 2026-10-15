package controllers

import (
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
