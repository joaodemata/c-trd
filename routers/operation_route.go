package routers

import (
	"net/http"

	cmm "c_trd/common"
	"c_trd/controllers"
)

func operationRoutes(server *http.ServeMux) *http.ServeMux  {
	// Router Decorator pointing to server
	api := cmm.NewRouter(server)
	// Routes
	api.Post("/operation/create/oportunity", controllers.CreateOportunityController())
	
	return server

}


