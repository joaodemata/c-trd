package data_validates

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/models"
	"c_trd/services"
)

// Mensaje del login cuando falla el usuario o la contraseña, es el mismo en ambos casos para no revelar que usuarios existen
const invalidCredentialsMessage = "Usuario o contraseña inválidos"

// Hash con formato valido que se verifica cuando el usuario no existe, asi el login tarda lo mismo exista o no
var dummyPasswordHash = fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", cmm.PasswordHashIterations, strings.Repeat("A", 22), strings.Repeat("A", 43))

// validateUserIsUnique valida que el username y el email (si viene) no los use otro usuario. Si falla responde al cliente
func validateUserIsUnique(res *cmm.ResponseHandler, username string, email *string) bool {
	existing, err := services.GetUserByUsernameService(username)
	if !err.IsEmtpy() {
		res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
		return false
	}
	if !existing.ID.IsZero() {
		res.Error("FAIL", "Ya existe un usuario con el username enviado", "DVAUT004", username, cmm.NewEmptyErrorHandler())
		return false
	}

	if email == nil {
		return true
	}

	existing, err = services.GetUserByEmailService(*email)
	if !err.IsEmtpy() {
		res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
		return false
	}
	if !existing.ID.IsZero() {
		res.Error("FAIL", "Ya existe un usuario con el email enviado", "DVAUT005", *email, cmm.NewEmptyErrorHandler())
		return false
	}

	return true
}

// AuthDataValidate busca la sesion no vencida del token que dejo cmm.AuthenticateRequestMiddleware. La sesion trae la
// copia del usuario y su privilegio, asi se autentica con una sola consulta. Deja la sesion en el contexto (authSession)
func AuthDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		token, err := cmm.GetFromRequestContext[string](r, "sessionToken")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVAUT020", nil, err)
			return
		}

		session, err := services.GetActiveSessionService(services.HashSessionTokenService(token), time.Now().UTC())
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if session.ID.IsZero() {
			res.Send(http.StatusUnauthorized, "FAIL", "La sesion no existe o expiro, inicie sesion nuevamente", "DVAUT002", nil)
			return
		}
		// Sesiones abiertas antes de guardar el usuario en la sesion
		if session.IDUser.IsZero() || session.TagTypePrivilege == "" {
			res.Send(http.StatusUnauthorized, "FAIL", "La sesion no es valida, inicie sesion nuevamente", "DVAUT021", nil)
			return
		}

		r = cmm.SetInRequestContext(r, "authSession", &session)
		next.ServeHTTP(w, r)
	})
}

// OptionalAuthDataValidate para rutas abiertas a usuarios sin registro, va despues de
// cmm.OptionalAuthenticateRequestMiddleware. Sin token la peticion sigue como anonima y authSession queda nil en el
// contexto; con token la sesion debe existir y no estar vencida para que el cliente sepa que debe iniciar sesion de nuevo
func OptionalAuthDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		token, err := cmm.GetFromRequestContext[string](r, "sessionToken")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVAUT023", nil, err)
			return
		}

		// Usuario sin registro
		if token == "" {
			var anonymous *models.SessionsModelType
			r = cmm.SetInRequestContext(r, "authSession", anonymous)
			next.ServeHTTP(w, r)
			return
		}

		session, err := services.GetActiveSessionService(services.HashSessionTokenService(token), time.Now().UTC())
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if session.ID.IsZero() {
			res.Send(http.StatusUnauthorized, "FAIL", "La sesion no existe o expiro, inicie sesion nuevamente", "DVAUT024", nil)
			return
		}
		// Sesiones abiertas antes de guardar el usuario en la sesion
		if session.IDUser.IsZero() || session.TagTypePrivilege == "" {
			res.Send(http.StatusUnauthorized, "FAIL", "La sesion no es valida, inicie sesion nuevamente", "DVAUT025", nil)
			return
		}

		r = cmm.SetInRequestContext(r, "authSession", &session)
		next.ServeHTTP(w, r)
	})
}

// PrivilegesDataValidate deja pasar solo peticiones cuyo usuario de la sesion tenga un privilegio (tag del catalogo
// USER_PRIVILEGES) de allowedTags, ej: PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag). Va despues de AuthDataValidate
func PrivilegesDataValidate(allowedTags ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			res := cmm.NewResponseHandler(w)

			authSession, err := cmm.GetFromRequestContext[*models.SessionsModelType](r, "authSession")
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, "DVAUT022", nil, err)
				return
			}

			if !models.SessionHasPrivilege(*authSession, allowedTags) {
				res.Send(http.StatusForbidden, "FAIL", "No tiene permisos para esta operacion", "DVAUT006", nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// LoginDataValidate valida las credenciales. Si la contraseña es incorrecta suma un intento fallido y al llegar al
// maximo el usuario queda bloqueado un tiempo. Deja el usuario en el contexto (user)
func LoginDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)
		now := time.Now().UTC()

		payload, err := cmm.GetFromRequestContext[*fv.LoginFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVAUT007", nil, err)
			return
		}

		user, err := services.GetUserByUsernameService(strings.ToLower(strings.TrimSpace(payload.Username)))
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		if user.ID.IsZero() {
			// Se verifica un hash de relleno para que la respuesta tarde lo mismo que con un usuario existente
			_, _ = services.VerifyPasswordService(payload.Password, dummyPasswordHash)
			res.Send(http.StatusUnauthorized, "FAIL", invalidCredentialsMessage, "DVAUT008", nil)
			return
		}

		if models.UserIsLocked(user, now) {
			res.Send(http.StatusUnauthorized, "FAIL", "Usuario bloqueado temporalmente por intentos fallidos, intente mas tarde", "DVAUT009", nil)
			return
		}

		match, err := services.VerifyPasswordService(payload.Password, user.PasswordHash)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		if !match {
			err = services.RegisterFailedLoginService(user.ID, now)
			if !err.IsEmtpy() {
				res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
				return
			}
			res.Send(http.StatusUnauthorized, "FAIL", invalidCredentialsMessage, "DVAUT010", nil)
			return
		}

		if !user.IsActive {
			res.Send(http.StatusUnauthorized, "FAIL", invalidCredentialsMessage, "DVAUT011", nil)
			return
		}

		r = cmm.SetInRequestContext(r, "user", &user)
		next.ServeHTTP(w, r)
	})
}

// CreateUserDataValidate valida que el privilegio exista y no sea el de super usuario (solo hay uno y se crea desde la
// base de datos) y que el username y el email no esten en uso. Deja el privilegio en el contexto (privilege)
func CreateUserDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.CreateUserFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVAUT012", nil, err)
			return
		}

		privilege, ok := findCatalog(res, payload.IdTypePrivilege, cmm.CatalogTypeUserPrivileges)
		if !ok {
			return
		}
		if privilege.ID.IsZero() {
			res.Error("FAIL", "El privilegio enviado no existe", "DVAUT013", payload.IdTypePrivilege, cmm.NewEmptyErrorHandler())
			return
		}
		if privilege.ID == cmm.UserPrivilegeSuperUser.ID {
			res.Error("FAIL", "Solo puede existir un super usuario", "DVAUT014", payload.IdTypePrivilege, cmm.NewEmptyErrorHandler())
			return
		}

		if !validateUserIsUnique(res, payload.Username, payload.Email) {
			return
		}

		r = cmm.SetInRequestContext(r, "privilege", &privilege)
		next.ServeHTTP(w, r)
	})
}

// UpdatePasswordDataValidate valida la contraseña actual del usuario de la sesion. Deja el usuario en el contexto (authUser)
func UpdatePasswordDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.UpdatePasswordFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVAUT015", nil, err)
			return
		}

		authSession, err := cmm.GetFromRequestContext[*models.SessionsModelType](r, "authSession")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVAUT016", nil, err)
			return
		}

		// El hash de la contraseña no esta en la sesion
		authUser, err := services.GetUserService(authSession.IDUser)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if authUser.ID.IsZero() || !authUser.IsActive {
			res.Send(http.StatusUnauthorized, "FAIL", "Usuario no disponible", "DVAUT003", nil)
			return
		}

		match, err := services.VerifyPasswordService(payload.CurrentPassword, authUser.PasswordHash)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if !match {
			res.Error("FAIL", "La contraseña actual es incorrecta", "DVAUT017", nil, cmm.NewEmptyErrorHandler())
			return
		}

		r = cmm.SetInRequestContext(r, "authUser", &authUser)
		next.ServeHTTP(w, r)
	})
}

// ResetUserPasswordDataValidate valida que el usuario exista. Deja el usuario en el contexto (targetUser)
func ResetUserPasswordDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.ResetUserPasswordFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVAUT018", nil, err)
			return
		}

		idUser, err := cmm.ParseHexToObjectId(payload.IdUser)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		user, err := services.GetUserService(idUser)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}
		if user.ID.IsZero() {
			res.Error("FAIL", "El usuario enviado no existe", "DVAUT019", payload.IdUser, cmm.NewEmptyErrorHandler())
			return
		}

		r = cmm.SetInRequestContext(r, "targetUser", &user)
		next.ServeHTTP(w, r)
	})
}
