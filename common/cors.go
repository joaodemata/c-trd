package common

import (
	"net/http"
	"os"
	"slices"
	"strings"
)

// Origen del frontend F-CTRD en desarrollo (Vite) cuando no se define JM_CTRD_CORS_ORIGINS
const defaultCorsOrigin = "http://localhost:5173"

// CorsOriginsFromEnv lee los origenes permitidos de JM_CTRD_CORS_ORIGINS, separados por coma
func CorsOriginsFromEnv() []string {
	origins := make([]string, 0)
	for _, origin := range strings.Split(os.Getenv("JM_CTRD_CORS_ORIGINS"), ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins = append(origins, origin)
		}
	}

	if len(origins) == 0 {
		origins = append(origins, defaultCorsOrigin)
	}

	return origins
}

// CorsMiddleware envuelve todo el servidor para que el frontend pueda consumir la API desde otro origen.
// Los preflight (OPTIONS) se responden aqui porque el mux no tiene rutas OPTIONS
func CorsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := origin != "" && slices.Contains(allowedOrigins, origin)

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}

			// Preflight del navegador
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if !allowed {
					NewResponseHandler(w).Send(http.StatusForbidden, "FAIL", "Origen no permitido", "CMMCOE001", nil)
					return
				}

				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
