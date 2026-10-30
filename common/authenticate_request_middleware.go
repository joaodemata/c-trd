package common

import (
	"net/http"
	"strings"
)

// ParseBearerToken extrae el token del header Authorization con formato "Bearer <token>". Retorna false si el
// header no tiene ese formato o el token esta vacio
func ParseBearerToken(authorization string) (string, bool) {
	token, found := strings.CutPrefix(authorization, "Bearer ")
	token = strings.TrimSpace(token)

	return token, found && token != ""
}

// AuthenticateRequestMiddleware deja pasar solo peticiones con el token de sesion en el header Authorization (Bearer).
// Deja el token en el contexto (sessionToken); dv.AuthDataValidate busca la sesion con ese token
func AuthenticateRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := NewResponseHandler(w)

		token, ok := ParseBearerToken(r.Header.Get("Authorization"))
		if !ok {
			res.Send(http.StatusUnauthorized, "FAIL", "Debe iniciar sesion", "CMMARE001", nil)
			return
		}

		r = SetInRequestContext(r, "sessionToken", token)
		next.ServeHTTP(w, r)
	})
}

// OptionalAuthenticateRequestMiddleware para rutas abiertas a usuarios sin registro: sin header Authorization deja
// el token vacio en el contexto (sessionToken) y la peticion sigue como anonima. Si el header viene debe ser un
// token Bearer valido. Va antes de dv.OptionalAuthDataValidate
func OptionalAuthenticateRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := NewResponseHandler(w)

		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			r = SetInRequestContext(r, "sessionToken", "")
			next.ServeHTTP(w, r)
			return
		}

		token, ok := ParseBearerToken(authorization)
		if !ok {
			res.Send(http.StatusUnauthorized, "FAIL", "El header Authorization debe ser un token Bearer", "CMMARE002", nil)
			return
		}

		r = SetInRequestContext(r, "sessionToken", token)
		next.ServeHTTP(w, r)
	})
}
