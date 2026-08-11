package routers

import (
	"net/http"

	"c_trd/common"
	"c_trd/controllers"
)


func hookRoutes(server *http.ServeMux) *http.ServeMux  {
	// Router Decorator pointing to server
	api := common.NewRouter(server)
	// Routes

	// Ruta para recibir los triggers de trading view 
	api.Post("/hook/trading_view", controllers.TradingViewHookController())
	
	return server

}


