package data_validates

import (
	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/services"
	"net/http"
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

		// Buscamos el trigger en mongoDB para validar si existe 
		triggerData, err := services.GetTriggerService(payload.IdTrigger)

		// Validamos que no este vacio
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		if triggerData.ID.IsZero() {
			res.Error("FAIL", "El trigger enviado no existe", "DVHOOK002", payload.IdTrigger, cmm.NewEmptyErrorHandler())
			return
		}

		// Inyectar en el contexto
		r = cmm.SetInRequestContext(r, "triggerData", &triggerData)

		// Seguimos al controlador
		next.ServeHTTP(w, r)
	})
}