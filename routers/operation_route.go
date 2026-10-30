package routers

import (
	"net/http"

	cmm "c_trd/common"
	"c_trd/controllers"
	dv "c_trd/data_validates"
	fv "c_trd/format_validates"
)

func operationRoutes(server *http.ServeMux) *http.ServeMux {
	// Router Decorator pointing to server
	api := cmm.NewRouter(server)
	// Routes
	api.Post("/operation/create/oportunity", controllers.CreateOportunityController())
	// Sesion -> privilegio -> format validates (trigger y visibilidad del mismo body) -> data validates
	api.Post("/operation/create/trigger", controllers.CreateTriggerController(), cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.CreateTriggerFormatValidate, fv.VisibilityFormatValidate, dv.VisibilityDataValidate, dv.CreateTriggerDataValidate)
	// Reemplaza el trigger: el anterior queda borrado e inactivo y el nuevo apunta a el (id_reference e id_reference_root)
	api.Put("/operation/update/trigger", controllers.UpdateTriggerController(), cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.UpdateTriggerFormatValidate, dv.UpdateTriggerDataValidate)
	api.Post("/operation/create/condition", controllers.CreateConditionController(), cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.CreateConditionFormatValidate, fv.VisibilityFormatValidate, dv.VisibilityDataValidate, dv.CreateConditionDataValidate)

	// CSV de Google Trends por asset: se sube manualmente y las condiciones lo leen desde el mismo servidor
	api.Post("/operation/google_trends/upload", controllers.GoogleTrendsUploadController(), cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.GoogleTrendsUploadFormatValidate, dv.GoogleTrendsUploadDataValidate)
	api.Post("/operation/google_trends/series", controllers.GoogleTrendsSeriesController(), cmm.LocalOnlyMiddleware, fv.GoogleTrendsSeriesFormatValidate, dv.GoogleTrendsSeriesDataValidate)

	return server

}
