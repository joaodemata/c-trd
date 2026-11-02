package routers

import (
	"net/http"

	cmm "c_trd/common"
	c "c_trd/controllers"
	dv "c_trd/data_validates"
	fv "c_trd/format_validates"
)


func hookRoutes(server *http.ServeMux) *http.ServeMux  {
	// Router Decorator pointing to server
	api := cmm.NewRouter(server)
	// Routes

	// Ruta generica para recibir los triggers de sistemas externos (TradingView u otros)
	// Controller, format, data, etc
	api.Post("/hook/trigger", c.TriggerHookController(),  fv.TriggerHookFormatValidate, dv.TriggerHookDataValidate)
	
	return server

}


