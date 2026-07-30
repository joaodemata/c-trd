package routers

import (
	"net/http"
)

// RegisterAssetRoutes recibe el enrutador maestro y "monta" sus propias rutas.
func listRoutes(server *http.ServeMux) *http.ServeMux  {
	// Agrupamos bajo el prefijo /api/assets
	server.HandleFunc("GET /list/record_assets", handleListRecordAssets())
	
	return server

}


func handleListRecordAssets() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Solo peticiones POST entran aquí"))
	}
}