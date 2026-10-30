package common

import (
	"net/http"
	"strings"
)

// AuthenticateRequestMiddleware deja pasar solo peticiones con el token de sesion en el header Authorization (Bearer).
// Deja el token en el contexto (sessionToken); dv.AuthDataValidate busca la sesion con ese token
func AuthenticateRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := NewResponseHandler(w)

		token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		token = strings.TrimSpace(token)
		if !found || token == "" {
			res.Send(http.StatusUnauthorized, "FAIL", "Debe iniciar sesion", "CMMARE001", nil)
			return
		}

		r = SetInRequestContext(r, "sessionToken", token)
		next.ServeHTTP(w, r)
	})
}
