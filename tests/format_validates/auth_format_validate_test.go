package format_validates_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// runAuthFormatValidate envia body al middleware y valida si dejo pasar la peticion o el tracking code del error
func runAuthFormatValidate(t *testing.T, middleware func(http.Handler) http.Handler, body map[string]any, wantNext bool, wantTracking string) *testutil.NextSpy {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(encoded)))
	rec := httptest.NewRecorder()
	spy := &testutil.NextSpy{}

	middleware(spy).ServeHTTP(rec, req)

	if spy.Called != wantNext {
		t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, wantNext, rec.Body.String())
	}
	if !wantNext {
		if res := testutil.DecodeResponse(t, rec); res.Tracking != wantTracking {
			t.Errorf("tracking = %s, se esperaba %s", res.Tracking, wantTracking)
		}
	}

	return spy
}

func validCreateUserBody() map[string]any {
	return map[string]any{
		"username":        "jmata",
		"password":        "una-clave-segura",
		"idTypePrivilege": cmm.UserPrivilegeGenericUser.ID.Hex(),
	}
}

func TestCreateUserFormatValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		edit         func(body map[string]any)
		wantNext     bool
		wantTracking string
	}{
		{name: "solo los obligatorios", edit: func(map[string]any) {}, wantNext: true},
		{name: "con todos los opcionales", wantNext: true, edit: func(b map[string]any) {
			b["email"] = "jmata@example.com"
			b["name"] = "Joao"
			b["lastName"] = "Mata"
		}},
		{name: "opcionales en null", wantNext: true, edit: func(b map[string]any) { b["email"] = nil; b["name"] = nil }},
		{name: "sin username", edit: func(b map[string]any) { delete(b, "username") }, wantTracking: "CC003"},
		{name: "username corto", edit: func(b map[string]any) { b["username"] = "jm" }, wantTracking: "CC003"},
		{name: "username largo", edit: func(b map[string]any) { b["username"] = strings.Repeat("a", 33) }, wantTracking: "CC003"},
		{name: "username con mayusculas", edit: func(b map[string]any) { b["username"] = "JMata" }, wantTracking: "CC003"},
		{name: "username con simbolos", edit: func(b map[string]any) { b["username"] = "j.mata" }, wantTracking: "CC003"},
		{name: "sin contraseña", edit: func(b map[string]any) { delete(b, "password") }, wantTracking: "CC003"},
		{name: "contraseña corta", edit: func(b map[string]any) { b["password"] = "1234567" }, wantTracking: "CC003"},
		{name: "contraseña larga", edit: func(b map[string]any) { b["password"] = strings.Repeat("a", 129) }, wantTracking: "CC003"},
		{name: "sin privilegio", edit: func(b map[string]any) { delete(b, "idTypePrivilege") }, wantTracking: "CC003"},
		{name: "privilegio que no es ObjectID", edit: func(b map[string]any) { b["idTypePrivilege"] = "GENERIC_USER" }, wantTracking: "CC003"},
		{name: "email invalido", edit: func(b map[string]any) { b["email"] = "no-es-email" }, wantTracking: "CC003"},
		{name: "email vacio", edit: func(b map[string]any) { b["email"] = "" }, wantTracking: "CC003"},
		{name: "email con mayusculas", edit: func(b map[string]any) { b["email"] = "JMata@example.com" }, wantTracking: "CC003"},
		{name: "nombre vacio", edit: func(b map[string]any) { b["name"] = "" }, wantTracking: "CC003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := validCreateUserBody()
			tt.edit(body)

			spy := runAuthFormatValidate(t, fv.CreateUserFormatValidate, body, tt.wantNext, tt.wantTracking)
			if !tt.wantNext {
				return
			}

			payload, errCtx := cmm.GetFromRequestContext[*fv.CreateUserFormat](spy.Request, "payload")
			testutil.RequireEmptyError(t, errCtx)
			if payload.Username != "jmata" {
				t.Errorf("username = %s, se esperaba jmata", payload.Username)
			}
		})
	}
}

func TestUpdatePasswordFormatValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         map[string]any
		wantNext     bool
		wantTracking string
	}{
		{name: "valido", body: map[string]any{"currentPassword": "clave-actual", "newPassword": "clave-nueva-1"}, wantNext: true},
		{name: "sin contraseña actual", body: map[string]any{"newPassword": "clave-nueva-1"}, wantTracking: "CC003"},
		{name: "nueva contraseña corta", body: map[string]any{"currentPassword": "clave-actual", "newPassword": "corta"}, wantTracking: "CC003"},
		{name: "nueva igual a la actual", body: map[string]any{"currentPassword": "clave-actual", "newPassword": "clave-actual"}, wantTracking: "CC003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runAuthFormatValidate(t, fv.UpdatePasswordFormatValidate, tt.body, tt.wantNext, tt.wantTracking)
		})
	}
}

func TestResetUserPasswordFormatValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         map[string]any
		wantNext     bool
		wantTracking string
	}{
		{name: "valido", body: map[string]any{"idUser": primitive.NewObjectID().Hex(), "newPassword": "clave-nueva-1"}, wantNext: true},
		{name: "usuario que no es ObjectID", body: map[string]any{"idUser": "abc", "newPassword": "clave-nueva-1"}, wantTracking: "CC003"},
		{name: "sin contraseña", body: map[string]any{"idUser": primitive.NewObjectID().Hex()}, wantTracking: "CC003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runAuthFormatValidate(t, fv.ResetUserPasswordFormatValidate, tt.body, tt.wantNext, tt.wantTracking)
		})
	}
}

func TestLoginFormatValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         map[string]any
		wantNext     bool
		wantTracking string
	}{
		{name: "valido", body: map[string]any{"username": "jmata", "password": "x"}, wantNext: true},
		{name: "sin username", body: map[string]any{"password": "x"}, wantTracking: "CC003"},
		{name: "sin contraseña", body: map[string]any{"username": "jmata"}, wantTracking: "CC003"},
		{name: "contraseña larga", body: map[string]any{"username": "jmata", "password": strings.Repeat("a", 129)}, wantTracking: "CC003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runAuthFormatValidate(t, fv.LoginFormatValidate, tt.body, tt.wantNext, tt.wantTracking)
		})
	}
}
