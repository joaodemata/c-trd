package format_validates

import (
	"c_trd/common"
)

// El username solo acepta minusculas y numeros para que no se repita con otra capitalizacion.
// Las contraseñas tienen un maximo para que el hash no se use para saturar el servidor

// LoginFormat credenciales del usuario
type LoginFormat struct {
	Username string `json:"username" validate:"required,max=32"`
	Password string `json:"password" validate:"required,max=128"`
}

var LoginFormatValidate = common.FormatValidateMiddleware[LoginFormat]

// CreateUserFormat datos del usuario que crea el super usuario. idTypePrivilege es un catalogo USER_PRIVILEGES.
// Email, name y lastName son opcionales
type CreateUserFormat struct {
	Username        string  `json:"username" validate:"required,min=3,max=32,alphanum,lowercase"`
	Password        string  `json:"password" validate:"required,min=8,max=128"`
	IdTypePrivilege string  `json:"idTypePrivilege" validate:"required,mongodb"`
	Email           *string `json:"email" validate:"omitempty,email,lowercase,max=254"`
	Name            *string `json:"name" validate:"omitempty,min=1,max=100"`
	LastName        *string `json:"lastName" validate:"omitempty,min=1,max=100"`
}

var CreateUserFormatValidate = common.FormatValidateMiddleware[CreateUserFormat]

// UpdatePasswordFormat cambio de contraseña del super usuario de la sesion, requiere la contraseña actual
type UpdatePasswordFormat struct {
	CurrentPassword string `json:"currentPassword" validate:"required,max=128"`
	NewPassword     string `json:"newPassword" validate:"required,min=8,max=128,nefield=CurrentPassword"`
}

var UpdatePasswordFormatValidate = common.FormatValidateMiddleware[UpdatePasswordFormat]

// ResetUserPasswordFormat el super usuario asigna una contraseña nueva a otro usuario (no hay recuperacion por email)
type ResetUserPasswordFormat struct {
	IdUser      string `json:"idUser" validate:"required,mongodb"`
	NewPassword string `json:"newPassword" validate:"required,min=8,max=128"`
}

var ResetUserPasswordFormatValidate = common.FormatValidateMiddleware[ResetUserPasswordFormat]
