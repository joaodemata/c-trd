package routers

import (
	"net/http"

	cmm "c_trd/common"
	"c_trd/controllers"
	dv "c_trd/data_validates"
	fv "c_trd/format_validates"
)

func operationRoutes(server *http.ServeMux) *http.ServeMux  {
	// Router Decorator pointing to server
	api := cmm.NewRouter(server)
	// Routes
	api.Post("/operation/create/oportunity", controllers.CreateOportunityController())
	api.Post("/operation/create/trigger", controllers.CreateTriggerController(), fv.CreateTriggerFormatValidate, dv.CreateTriggerDataValidate)

	// CSV de Google Trends por asset: se sube manualmente y las condiciones lo leen desde el mismo servidor
	api.Post("/operation/google_trends/upload", controllers.GoogleTrendsUploadController(), fv.GoogleTrendsUploadFormatValidate, dv.GoogleTrendsUploadDataValidate)
	api.Post("/operation/google_trends/series", controllers.GoogleTrendsSeriesController(), cmm.LocalOnlyMiddleware, fv.GoogleTrendsSeriesFormatValidate, dv.GoogleTrendsSeriesDataValidate)
	
	return server

}


