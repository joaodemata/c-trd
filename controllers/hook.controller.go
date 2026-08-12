package controllers

import (
	"net/http"

	fv "c_trd/format_validates"
)


func TradingViewHookController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Extraemos el valor usando la llave pública que exportaste en 'common'
		// Extracción y aserción en la misma línea
		datos, ok := r.Context().Value("payload").(*fv.TradingViewTriggerFormat)

		if !ok {
			http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
			return
		}

		w.Write([]byte("Solo peticiones POST entran aquí test" + datos.Asset))
	}
}