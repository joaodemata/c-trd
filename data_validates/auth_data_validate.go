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

// authenticateRequest busca la sesion del token del header Authorization (Bearer) y valida que el usuario exista
// y este activo. Si falla responde al cliente y retorna false
func authenticateRequest(res *cmm.ResponseHandler, r *http.Request) (models.UsersModelType, models.SessionsModelType, bool) {
	var emptyUser models.UsersModelType
	var emptySession models.SessionsModelType

	token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	token = strings.TrimSpace(token)
	if !found || token == "" {
		res.Send(http.StatusUnauthorized, "FAIL", "Debe iniciar sesion", "DVAUT001", nil)
		return emptyUser, emptySession, false
	}

	session, err := services.GetActiveSessionService(services.HashSessionTokenService(token), time.Now().UTC())
	if !err.IsEmtpy() {
		res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
		return emptyUser, emptySession, false
	}
	if session.ID.IsZero() {
		res.Send(http.StatusUnauthorized, "FAIL", "La sesion no existe o expiro, inicie sesion nuevamente", "DVAUT002", nil)
		return emptyUser, emptySession, false
	}

	user, err := services.GetUserService(session.IDUser)
	if !err.IsEmtpy() {
		res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
		return emptyUser, emptySession, false
	}
	if user.ID.IsZero() || !user.IsActive {
		res.Send(http.StatusUnauthorized, "FAIL", "Usuario no disponible", "DVAUT003", nil)
		return emptyUser, emptySession, false
	}

	return user, session, true
}

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

// AuthDataValidate deja pasar solo peticiones con una sesion valida. Deja en el contexto el usuario (authUser) y la sesion (authSession)
func AuthDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		user, session, ok := authenticateRequest(res, r)
		if !ok {
			return
		}

		r = cmm.SetInRequestContext(r, "authUser", &user)
		r = cmm.SetInRequestContext(r, "authSession", &session)
		next.ServeHTTP(w, r)
	})
}

// SuperUserDataValidate deja pasar solo peticiones del super usuario con sesion valida. Deja en el contexto el
// usuario (authUser) y la sesion (authSession)
func SuperUserDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		user, session, ok := authenticateRequest(res, r)
		if !ok {
			return
		}

		if !models.UserIsSuperUser(user) {
			res.Send(http.StatusForbidden, "FAIL", "No tiene permisos para esta operacion", "DVAUT006", nil)
			return
		}

		r = cmm.SetInRequestContext(r, "authUser", &user)
		r = cmm.SetInRequestContext(r, "authSession", &session)
		next.ServeHTTP(w, r)
	})
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

// UpdatePasswordDataValidate valida la contraseña actual del usuario de la sesion
func UpdatePasswordDataValidate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.UpdatePasswordFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVAUT015", nil, err)
			return
		}

		authUser, err := cmm.GetFromRequestContext[*models.UsersModelType](r, "authUser")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, "DVAUT016", nil, err)
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
