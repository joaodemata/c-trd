package routers

import (
	"net/http"
)

// SetupRouter actúa como el index.js abstracto
func SetupRouter(server *http.ServeMux) *http.ServeMux {
	// Aquí delegas de forma abstracta. El index no sabe si AssetRoutes 
	// tiene 1 o 100 rutas, ni si son POST o GET. Delega la responsabilidad.
	listRoutes(server)
	operationRoutes(server)

	return server
}