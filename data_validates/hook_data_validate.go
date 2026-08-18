package data_validates

import (
	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/services"
	"fmt"
	"net/http"
)


func TradingViewHookDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Creamos el response
		res := cmm.NewResponseHandler(w)

		// Extraemos el payload
		// payload, ok := r.Context().Value("payload").(*fv.TradingViewTriggerFormat)
		payload, err := cmm.GetFromRequestContext[*fv.TradingViewTriggerFormat](r, "payload")
		fmt.Printf("%+v\n", payload)


		if err != nil {
			res.Error("FAIL", "Error parseando el payload", "DVHOOK001", nil)
			return
		}

		// Buscamos el trigger en mongoDB para validar si existe 
		triggerData, err := services.GetTriggerService(payload.IdTrigger)
		//TODO: crear un tipo ERROR personalizado
		if err != nil {
			res.Error("FAIL", "Database error", "DVHOOK002", nil)
			return
		}

		if triggerData.ID.IsZero() {
			res.Error("FAIL", "El trigger enviado no existe", "DVHOOK003", payload.IdTrigger)
			return
		}

		// Inyectar en el contexto
		r = cmm.SetInRequestContext(r, "triggerData", &triggerData)

		// Seguimos al controlador
		next.ServeHTTP(w, r)
	})
}