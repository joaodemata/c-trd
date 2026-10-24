package data_validates

import (
	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/services"
	"net/http"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TradingViewHookDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Creamos el response
		res := cmm.NewResponseHandler(w)

		// Extraemos el payload
		// payload, ok := r.Context().Value("payload").(*fv.TradingViewTriggerFormat)
		payload, err := cmm.GetFromRequestContext[*fv.TradingViewTriggerFormat](r, "payload")

		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVHOOK001", nil, err)
			return
		}

		idTrigger, err := cmm.ParseHexToObjectId(payload.IdTrigger)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		// Buscamos el trigger en mongoDB para validar si existe
		triggerData, err := services.GetTriggerService(idTrigger)

		// Validamos que no este vacio
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		if triggerData.ID.IsZero() {
			res.Error("FAIL", "El trigger enviado no existe", "DVHOOK002", payload.IdTrigger, cmm.NewEmptyErrorHandler())
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
