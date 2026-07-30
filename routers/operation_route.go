package routers

import (
	"net/http"
)

// RegisterAssetRoutes recibe el enrutador maestro y "monta" sus propias rutas.
func operationRoutes(server *http.ServeMux) *http.ServeMux  {
	// Agrupamos bajo el prefijo /api/assets
	server.HandleFunc("GET /operation/record_assets", handleOperationRecordAssets())

	return server

}


func handleOperationRecordAssets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Solo peticiones POST entran aquí"))
	}
}