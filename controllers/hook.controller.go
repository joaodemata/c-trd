package controllers

import (
	"net/http"

	cmm "c_trd/common"
	"c_trd/models"
)


func TradingViewHookController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Creamos el response
		res := cmm.NewResponseHandler(w)
	

		triggerData, err := cmm.GetFromRequestContext[*models.TriggersModelType](r, "triggerData")

		// Verificamos si hay error de parseo 
		if !err.IsEmtpy(){
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		// TODO: crear y verificar que no hay una oportunidad activa para no duplicar 

		// Response Data
    	data := map[string]any{
        "idOpportunity":  triggerData.ID,
    	}


		res.Send(http.StatusAccepted, "SUCCESS", "Oportunidad creada exitosamente", "CHOOK001", data)
	}
}