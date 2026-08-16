package data_validates

import (
	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/services"
	"context"
	"net/http"
)


func TradingViewHookDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Creamos el response
		res := cmm.NewResponseHandler(w)

		// Extraemos el payload
		payload, ok := r.Context().Value("payload").(*fv.TradingViewTriggerFormat)

		if !ok {
			res.Error("FAIL", "Error parseando el payload", "DVHOOK001", nil)
			return
		}

		// Buscamos el trigger en mongoDB para validar si existe 
		triggerData, err := services.GetTriggerService(payload.IdTrigger)
		
		if err != nil {
			res.Error("FAIL", "Database error", "DVHOOK002", nil)
			return
		}

		if triggerData != nil {
			res.Error("FAIL", "El trigger enviado no existe", "DVHOOK003", payload.IdTrigger)
			return
		}

		// Guardamos el trigger en formato oportunidad para crearlo en el controlador 
		ctx := context.WithValue(r.Context(), "opportunity",triggerData)

		// Seguimos al controlador
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}