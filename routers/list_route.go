package routers

import (
	"net/http"

	cmm "c_trd/common"
	c "c_trd/controllers"
	fv "c_trd/format_validates"
)

func listRoutes(server *http.ServeMux) *http.ServeMux  {

	// Router Decorator pointing to server
	api := cmm.NewRouter(server)

	// Routes
	api.Get("/list/opportunities", c.ListOportunitiesController(), cmm.PaginationFormatValidate)
	// Creacion de condiciones: se listan las APIs y luego los callbacks del response_format de la API elegida
	api.Get("/list/apis", c.ListApisController(), fv.ListApisFormatValidate)
	api.Get("/list/callbacks", c.ListCallbacksController(), fv.ListCallbacksFormatValidate)
	
	return server

}

