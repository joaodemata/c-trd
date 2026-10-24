package routers

import (
	"net/http"

	cmm "c_trd/common"
	c "c_trd/controllers"
	dv "c_trd/data_validates"
	fv "c_trd/format_validates"
)

func listRoutes(server *http.ServeMux) *http.ServeMux  {

	// Router Decorator pointing to server
	api := cmm.NewRouter(server)

	// Routes
	api.Get("/list/opportunities", c.ListOportunitiesController(), cmm.PaginationFormatValidate)
	// Creacion de condiciones: se listan las APIs y luego los callbacks del response_format de la API elegida
	api.Get("/list/apis", c.ListApisController(), cmm.PaginationFormatValidate)
	api.Get("/list/callbacks", c.ListCallbacksController(), fv.ListCallbacksFormatValidate)
	// Selectores del frontend para crear triggers y subir el CSV de Google Trends
	api.Get("/list/assets", c.ListAssetsController(), cmm.PaginationFormatValidate)
	api.Get("/list/catalogs", c.ListCatalogsController(), fv.ListCatalogsFormatValidate)
	api.Get("/list/conditions", c.ListConditionsController(), cmm.PaginationFormatValidate)
	api.Get("/list/triggers", c.ListTriggersController(), cmm.PaginationFormatValidate, fv.ListTriggersFormatValidate, dv.ListTriggersDataValidate)
	
	return server

}

