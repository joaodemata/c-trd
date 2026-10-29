package controllers

import (
	"net/http"
	"time"

	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/models"
	"c_trd/services"
)

// LoginController registra el login y abre una sesion para el usuario validado en LoginDataValidate
func LoginController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)
		now := time.Now().UTC()

		user, err := cmm.GetFromRequestContext[*models.UsersModelType](r, "user")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		err = services.RegisterSuccessfulLoginService(user.ID, now)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		token, tokenHash, err := services.GenerateSessionTokenService()
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		session, err := models.NewSessionModel(user.ID, tokenHash, now)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		err = services.CreateSessionService(session)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		user.LastLoginAt = &now
		data := map[string]any{
			"token":     token,
			"expiresAt": session.ExpiresAt,
			"user":      user,
		}

		res.Send(http.StatusOK, "SUCCESS", "Sesion iniciada", "CAUT001", data)
	}
}

// LogoutController cierra la sesion actual
func LogoutController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		authSession, err := cmm.GetFromRequestContext[*models.SessionsModelType](r, "authSession")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		err = services.DeleteSessionService(authSession.ID)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusOK, "SUCCESS", "Sesion cerrada", "CAUT002", nil)
	}
}

// MeController retorna el usuario de la sesion
func MeController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		authUser, err := cmm.GetFromRequestContext[*models.UsersModelType](r, "authUser")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusOK, "SUCCESS", "Usuario de la sesion", "CAUT003", authUser)
	}
}

// CreateUserController crea un usuario con el privilegio validado en CreateUserDataValidate, queda registrado el
// super usuario que lo creo
func CreateUserController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.CreateUserFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		authUser, err := cmm.GetFromRequestContext[*models.UsersModelType](r, "authUser")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		privilege, err := cmm.GetFromRequestContext[*models.CatalogsModelType](r, "privilege")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		passwordHash, err := services.HashPasswordService(payload.Password)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		user, err := models.NewUserModel(payload.Username, *privilege, passwordHash, &models.UserOptionalData{
			Email:       payload.Email,
			Name:        payload.Name,
			LastName:    payload.LastName,
			IDCreatedBy: &authUser.ID,
		})
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		idUser, err := services.CreateUserService(user)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusCreated, "SUCCESS", "Usuario creado exitosamente", "CAUT004", map[string]any{"idUser": idUser})
	}
}

// UpdatePasswordController cambia la contraseña del super usuario de la sesion. Se cierran todas sus sesiones y se
// abre una nueva para que el dispositivo actual siga conectado
func UpdatePasswordController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)
		now := time.Now().UTC()

		payload, err := cmm.GetFromRequestContext[*fv.UpdatePasswordFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		authUser, err := cmm.GetFromRequestContext[*models.UsersModelType](r, "authUser")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		passwordHash, err := services.HashPasswordService(payload.NewPassword)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		err = services.UpdateUserPasswordService(authUser.ID, passwordHash, now)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		err = services.DeleteUserSessionsService(authUser.ID)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		token, tokenHash, err := services.GenerateSessionTokenService()
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		session, err := models.NewSessionModel(authUser.ID, tokenHash, now)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		err = services.CreateSessionService(session)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		data := map[string]any{
			"token":     token,
			"expiresAt": session.ExpiresAt,
		}

		res.Send(http.StatusOK, "SUCCESS", "Contraseña actualizada", "CAUT005", data)
	}
}

// ResetUserPasswordController el super usuario asigna una contraseña nueva a otro usuario y cierra todas sus sesiones
func ResetUserPasswordController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res := cmm.NewResponseHandler(w)

		payload, err := cmm.GetFromRequestContext[*fv.ResetUserPasswordFormat](r, "payload")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		targetUser, err := cmm.GetFromRequestContext[*models.UsersModelType](r, "targetUser")
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		passwordHash, err := services.HashPasswordService(payload.NewPassword)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		err = services.UpdateUserPasswordService(targetUser.ID, passwordHash, time.Now().UTC())
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		err = services.DeleteUserSessionsService(targetUser.ID)
		if !err.IsEmtpy() {
			res.Error("FAIL", err.Message, err.TrackingCode, nil, err)
			return
		}

		res.Send(http.StatusOK, "SUCCESS", "Contraseña del usuario actualizada", "CAUT006", nil)
	}
}
