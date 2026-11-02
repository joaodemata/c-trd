package data_validates

import (
	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/services"
	"net/http"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TriggerHookDataValidate valida el trigger del webhook y deja en el contexto los datos para el controller
func TriggerHookDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Creamos el response
		res := cmm.NewResponseHandler(w)

		// Extraemos el payload
		// payload, ok := r.Context().Value("payload").(*fv.TriggerHookFormat)
		payload, err := cmm.GetFromRequestContext[*fv.TriggerHookFormat](r, "payload")

		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVHOOK001", nil, err)
			return
		}

		idWebhook, err := cmm.ParseHexToObjectId(payload.IdWebhook)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		// Buscamos la version vigente del trigger con ese id de webhook para validar si existe
		triggerData, err := services.GetTriggerByWebhookService(idWebhook)

		// Validamos que no este vacio
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		if triggerData.ID.IsZero() {
			res.Error("FAIL", "El trigger enviado no existe", "DVHOOK002", payload.IdWebhook, cmm.NewEmptyErrorHandler())
			return
		}

		// Un trigger creado por un agente no dispara oportunidades hasta que el super usuario lo apruebe
		if triggerData.TagStatus == cmm.GenericStatusPendingForApprove.Tag {
			res.Error("FAIL", "El trigger esta pendiente de aprobacion", "DVHOOK005", payload.IdWebhook, cmm.NewEmptyErrorHandler())
			return
		}
		if triggerData.TagStatus == cmm.GenericStatusNotApproved.Tag {
			res.Error("FAIL", "El trigger no fue aprobado", "DVHOOK006", payload.IdWebhook, cmm.NewEmptyErrorHandler())
			return
		}

		// Cancelacion: no importan las condiciones, solo la razon y la oportunidad en curso o activa del trigger
		if payload.IdCancel != "" {
			idCancel, err := cmm.ParseHexToObjectId(payload.IdCancel)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}

			cancelReason, err := services.GetCatalogService(idCancel, cmm.CatalogTypeCancelReason)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}

			if cancelReason.ID.IsZero() {
				res.Error("FAIL", "La razon de cancelacion enviada no existe", "DVHOOK003", payload.IdCancel, cmm.NewEmptyErrorHandler())
				return
			}

			opportunity, err := services.GetActiveOpportunity(triggerData.ID)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}

			if opportunity.ID.IsZero() {
				res.Error("FAIL", "El trigger no tiene una oportunidad en curso o activa para cancelar", "DVHOOK004", payload.IdWebhook, cmm.NewEmptyErrorHandler())
				return
			}

			r = cmm.SetInRequestContext(r, "cancelReason", &cancelReason)
			r = cmm.SetInRequestContext(r, "opportunity", &opportunity)

			next.ServeHTTP(w, r)
			return
		}

		// Buscamos el status actual de las condiciones del trigger para copiarlas a la oportunidad
		idConditions := make([]primitive.ObjectID, 0, len(triggerData.Conditions))
		for _, condition := range triggerData.Conditions {
			idConditions = append(idConditions, condition.IDCondition)
		}

		conditions, err := services.GetConditionsByIDsService(idConditions)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		// Inyectar en el contexto
		r = cmm.SetInRequestContext(r, "triggerData", &triggerData)
		r = cmm.SetInRequestContext(r, "conditions", conditions)

		// Seguimos al controlador
		next.ServeHTTP(w, r)
	})
}
