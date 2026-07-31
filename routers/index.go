package routers

import (
	"net/http"
)

// Call and group all the routes
func SetupRouter(server *http.ServeMux) *http.ServeMux {
	// Load all the routes
	listRoutes(server)
	operationRoutes(server)
	hookRoutes(server)

	return server
}