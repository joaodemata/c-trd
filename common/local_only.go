package common

import (
	"net"
	"net/http"
)

// Headers que agrega un reverse proxy. Si vienen, la peticion llego desde afuera aunque la IP sea local
var proxyHeaders = []string{"X-Forwarded-For", "X-Real-Ip", "Forwarded"}

// LocalOnlyMiddleware solo deja pasar peticiones hechas desde el mismo servidor (127.0.0.1 o ::1),
// se usa en las rutas que solo consume el cronjob de condiciones
func LocalOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := NewResponseHandler(w)

		host, _, err := net.SplitHostPort(r.RemoteAddr)
		ip := net.ParseIP(host)
		if err != nil || ip == nil || !ip.IsLoopback() {
			res.Send(http.StatusForbidden, "FAIL", "Ruta solo disponible desde el servidor", "CMMLOE001", nil)
			return
		}

		for _, header := range proxyHeaders {
			if r.Header.Get(header) != "" {
				res.Send(http.StatusForbidden, "FAIL", "Ruta solo disponible desde el servidor", "CMMLOE002", nil)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
