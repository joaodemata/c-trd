package data_validates

import (
	cmm "c_trd/common"
	fv "c_trd/format_validates"
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

		idAsset, ok := validateAssetExists(res, payload.IdAsset)
		if !ok {
			return
		}

		// Se parsea antes de guardar para no reemplazar un archivo valido por uno invalido
		series, err := services.ParseGoogleTrendsCSVService(file.Content, time.Now().UTC())
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, cmm.NewEmptyErrorHandler())
			return
		}

		r = cmm.SetInRequestContext(r, "idAsset", idAsset)
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

		idAsset, ok := validateAssetExists(res, payload.IdAsset)
		if !ok {
			return
		}

		r = cmm.SetInRequestContext(r, "idAsset", idAsset)
		next.ServeHTTP(w, r)
	})
}

// validateAssetExists busca el asset y responde el error si no existe. Retorna false si ya se respondio
func validateAssetExists(res *cmm.ResponseHandler, idAssetHex string) (primitive.ObjectID, bool) {
	idAsset, err := cmm.ParseHexToObjectId(idAssetHex)
	if !err.IsEmtpy() {
		res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
		return idAsset, false
	}

	asset, err := services.GetAssetService(idAsset)
	if !err.IsEmtpy() {
		res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
		return idAsset, false
	}

	if asset.ID.IsZero() {
		res.Error("FAIL", "El asset enviado no existe", "DVOPR003", idAssetHex, cmm.NewEmptyErrorHandler())
		return idAsset, false
	}

	return idAsset, true
}
