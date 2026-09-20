package routers

import (
	"net/http"

	cmm "c_trd/common"
	c "c_trd/controllers"
)

func listRoutes(server *http.ServeMux) *http.ServeMux  {

	// Router Decorator pointing to server
	api := cmm.NewRouter(server)

	// Routes
	api.Get("/list/opportunities", c.ListOportunitiesController(), cmm.PaginationFormatValidate)
	
	return server

}

