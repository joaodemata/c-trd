package controllers

import (
	fv "c_trd/format_validates"
	"c_trd/models"
	"c_trd/services"
	"net/http"
	"time"

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

		// Cancelacion: la oportunidad pasa a CANCELED de inmediato con el precio de TradingView
		if payload.IdCancel != "" {
			cancelReason, err := cmm.GetFromRequestContext[*models.CatalogsModelType](r, "cancelReason")
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}

			opportunity, err := cmm.GetFromRequestContext[*models.OpportunitiesModelType](r, "opportunity")
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}

			err = services.CancelOpportunityService(opportunity.ID, *cancelReason, payload.Price, time.Now().UTC())
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}

			data := map[string]any{
				"idOpportunity": opportunity.ID.Hex(),
			}

			res.Send(http.StatusAccepted, "SUCCESS", "Oportunidad cancelada exitosamente", "CHOOK002", data)
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
				// La oportunidad es del usuario del trigger y la ven los mismos que ven el trigger
				triggerData.RecordOwnerData,
				triggerData.RecordVisibilityData,
				&models.OpportunityOptionalData{
					StopLoss:       newRiskManagement(payload.StopLoss),
					TakeProfit:     newRiskManagement(payload.TakeProfit),
					Conditions:     conditions,
					Metadata:       payload.Metadata,
					PriceTriggered: &payload.Price,
					// Sin condiciones nace activa con el mismo precio de TradingView, el modelo lo ignora si nace en curso
					PriceActiveOpportunity: &payload.Price,
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

// newRiskManagement convierte los objetivos del hook al modelo de la oportunidad. Retorna nil si no vienen,
// asi el modelo los guarda como []
func newRiskManagement(targets []fv.RiskManagementFormat) []models.RiskManagement {
	if len(targets) == 0 {
		return nil
	}

	result := make([]models.RiskManagement, 0, len(targets))
	for _, target := range targets {
		result = append(result, models.RiskManagement{TargetPrice: target.Price, Probability: target.Probability})
	}

	return result
}
