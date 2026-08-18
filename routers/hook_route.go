package routers

import (
	"net/http"

	"c_trd/common"
	c "c_trd/controllers"
	dv "c_trd/data_validates"
	fv "c_trd/format_validates"
)


func hookRoutes(server *http.ServeMux) *http.ServeMux  {
	// Router Decorator pointing to server
	api := common.NewRouter(server)
	// Routes

	// Ruta para recibir los triggers de trading view 
	// Controller, format, data, etc
	api.Post("/hook/trading_view", c.TradingViewHookController(),  fv.TradingViewTriggerFormatValidate, dv.TradingViewHookDataValidate)
	
	return server

}


