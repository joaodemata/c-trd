package controllers

import (
	"c_trd/models"
	"c_trd/services"
	"net/http"

	cmm "c_trd/common"
)

func TradingViewHookController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Creamos el response
		res := cmm.NewResponseHandler(w)

		triggerData, err := cmm.GetFromRequestContext[*models.TriggersModelType](r, "triggerData")

		// Verificamos si hay error de parseo
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		// Id de la oportunidad creada
		var idOpportunity string

		// Validamos si ya existe oportunidad activa del trigger
		existOpportunity, err := services.GetActiveOpportunity(triggerData.ID)

		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		// Si no existe lo creamos
		if existOpportunity.ID.IsZero() {

			opportunity := models.NewOpportunityModel(triggerData.ID,
				triggerData.Tag,
				triggerData.Name,
				triggerData.IDAsset,
				triggerData.TagAsset,
				triggerData.Asset,
				triggerData.IDAction,
				triggerData.TagAction,
				triggerData.Action,
				triggerData.IDStatus,
				triggerData.TagStatus,
				triggerData.Status,
				triggerData.MaxCandlestickQty,
				triggerData.IDCandlestickTimeframe,
				triggerData.TagCandlestickTimeframe,
				triggerData.CandlestickTimeframe,
				nil)
			// Oportunidad creada
			createdOpportunity, err := services.CreateOpportunityService(opportunity)

			idOpportunity = createdOpportunity

			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}
		} else {
			idOpportunity = existOpportunity.ID.Hex()
		}

		// Response Data
		data := map[string]any{
			"idOpportunity": idOpportunity,
		}

		res.Send(http.StatusAccepted, "SUCCESS", "Oportunidad creada exitosamente", "CHOOK001", data)
	}
}
