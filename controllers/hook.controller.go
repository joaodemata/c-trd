package controllers

import (
	fv "c_trd/format_validates"
	"c_trd/models"
	"c_trd/services"
	"context"
	"log"
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

		payload, err := cmm.GetFromRequestContext[*fv.TradingViewTriggerFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		conditions, err := cmm.GetFromRequestContext[[]models.ConditionsModelType](r, "conditions")
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

			// Sin condiciones la oportunidad nace activa y se guarda el precio de mercado del asset. Si la consulta
			// falla se crea igual sin ese precio para no perder la señal de TradingView
			var priceActive *float64
			if len(conditions) == 0 {
				ctx, cancel := context.WithTimeout(r.Context(), cmm.HookAssetPriceTimeout)
				price, errPrice := services.GetAssetMarketPriceService(ctx, triggerData.IDAsset)
				cancel()

				if errPrice.IsEmtpy() {
					priceActive = &price
				} else {
					log.Printf("No se obtuvo el precio del asset %s, la oportunidad se crea activa sin precio: %v", triggerData.IDAsset.Hex(), errPrice)
				}
			}

			opportunity, err := models.NewOpportunityModel(triggerData.ID,
				triggerData.Tag,
				triggerData.Name,
				triggerData.Description,
				triggerData.IDAsset,
				triggerData.TagAsset,
				triggerData.Asset,
				triggerData.IDAction,
				triggerData.TagAction,
				triggerData.Action,
				triggerData.MaxCandlestickQty,
				triggerData.IDCandlestickTimeframe,
				triggerData.TagCandlestickTimeframe,
				triggerData.CandlestickTimeframe,
				&models.OpportunityOptionalData{
					Conditions:             conditions,
					Metadata:               payload.Metadata,
					PriceTriggered:         &payload.Price,
					PriceActiveOpportunity: priceActive,
				})

			// Validamos que no haya errores

			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}

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
