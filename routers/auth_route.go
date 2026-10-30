package routers

import (
	"net/http"

	cmm "c_trd/common"
	"c_trd/controllers"
	dv "c_trd/data_validates"
	fv "c_trd/format_validates"
)

// Usuarios y sesiones. No hay registro publico ni recuperacion por email: el super usuario (solo hay uno) se crea
// con scripts/create_super_user.js y es el unico que crea usuarios y cambia contraseñas
func authRoutes(server *http.ServeMux) *http.ServeMux {
	// Router Decorator pointing to server
	api := cmm.NewRouter(server)
	// Routes
	api.Post("/auth/login", controllers.LoginController(), fv.LoginFormatValidate, dv.LoginDataValidate)
	api.Post("/auth/logout", controllers.LogoutController(), dv.AuthDataValidate)
	api.Get("/auth/me", controllers.MeController(), dv.AuthDataValidate)

	// Solo los privilegios enviados a PrivilegesDataValidate (tags del catalogo USER_PRIVILEGES)
	api.Post("/auth/create/user", controllers.CreateUserController(), dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.CreateUserFormatValidate, dv.CreateUserDataValidate)
	api.Put("/auth/update/password", controllers.UpdatePasswordController(), dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.UpdatePasswordFormatValidate, dv.UpdatePasswordDataValidate)
	api.Put("/auth/update/user/password", controllers.ResetUserPasswordController(), dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.ResetUserPasswordFormatValidate, dv.ResetUserPasswordDataValidate)

	return server
}
