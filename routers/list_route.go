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
	// Sesion opcional: sin sesion solo las oportunidades publicas; con sesion tambien las del usuario y las de lista blanca donde esta
	api.Get("/list/opportunities", c.ListOportunitiesController(), cmm.OptionalAuthenticateRequestMiddleware, dv.OptionalAuthDataValidate, cmm.PaginationFormatValidate)
	// Creacion de condiciones: se listan las APIs y luego los callbacks del response_format de la API elegida
	api.Get("/list/apis", c.ListApisController(), cmm.PaginationFormatValidate)
	api.Get("/list/callbacks", c.ListCallbacksController(), fv.ListCallbacksFormatValidate)
	// Selectores del frontend para crear triggers y subir el CSV de Google Trends
	api.Get("/list/assets", c.ListAssetsController(), cmm.PaginationFormatValidate)
	api.Get("/list/catalogs", c.ListCatalogsController(), fv.ListCatalogsFormatValidate)
	// Sesion opcional: sin sesion o sin privilegio solo las condiciones activas o inactivas; tagStatus solo el super usuario
	api.Get("/list/conditions", c.ListConditionsController(), cmm.OptionalAuthenticateRequestMiddleware, dv.OptionalAuthDataValidate, cmm.PaginationFormatValidate, fv.ListConditionsFormatValidate, dv.ListConditionsDataValidate)
	api.Get("/list/triggers", c.ListTriggersController(), cmm.PaginationFormatValidate, fv.ListTriggersFormatValidate, dv.ListTriggersDataValidate)
	// Agentes: cola de tesis (FIFO) y corridas para que el super usuario las revise
	api.Get("/list/agent_theses", c.ListAgentThesesController(), cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), cmm.PaginationFormatValidate, fv.ListAgentThesesFormatValidate)
	api.Get("/list/agent_runs", c.ListAgentRunsController(), cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), cmm.PaginationFormatValidate, fv.ListAgentRunsFormatValidate)
	
	return server

}

