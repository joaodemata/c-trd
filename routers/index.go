package routers

import (
	cmm "c_trd/common"
	"net/http"
	"time"
)

// Limites de peticiones por IP, en memoria. Las rutas que comparten la misma variable comparten el limite.
// El login es estricto contra fuerza bruta y el hook es amplio porque TradingView manda rafagas desde pocas IPs.
// La ruta LocalOnly del cron de condiciones no lleva limite
var (
	generalRateLimit = cmm.RateLimitMiddleware(120, time.Minute)
	loginRateLimit   = cmm.RateLimitMiddleware(5, time.Minute)
	hookRateLimit    = cmm.RateLimitMiddleware(60, time.Minute)
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
	authRoutes(server)

	return server
}
