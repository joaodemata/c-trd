package routers

import (
	"net/http"

	"c_trd/common"
	"c_trd/controllers"
)

func operationRoutes(server *http.ServeMux) *http.ServeMux  {
	// Router Decorator pointing to server
	api := common.NewRouter(server)
	// Routes
	api.Post("/operation/create/oportunity", controllers.CreateOportunityController())
	
	return server

}


