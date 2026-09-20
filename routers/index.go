package routers

import (
	cmm "c_trd/common"
	"net/http"
)

// Call and group all the routes
func SetupRouter(server *http.ServeMux, hub *cmm.Hub) *http.ServeMux {
	// Instance for websocket 
	router := cmm.NewRouter(server)
	router.WebSocketOn(hub)
	
	// Load all the routes
	listRoutes(server)
	operationRoutes(server)
	hookRoutes(server)

	return server
}