package routers

import (
	"net/http"

	"c_trd/common"
	"c_trd/controllers"
)

func listRoutes(server *http.ServeMux) *http.ServeMux  {

	// Router Decorator pointing to server
	api := common.NewRouter(server)
	// Routes
	api.Get("/list/oportunities", controllers.ListOportunitiesController())
	
	return server

}

