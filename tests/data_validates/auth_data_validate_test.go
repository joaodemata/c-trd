package data_validates_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	cmm "c_trd/common"
	dv "c_trd/data_validates"
	"c_trd/models"
	"c_trd/tests/testutil"
)

// Solo se prueban los casos que no consultan MongoDB: sin token en el contexto (falta cmm.AuthenticateRequestMiddleware)
func TestAuthDataValidate_WithoutToken(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	spy := &testutil.NextSpy{}

	dv.AuthDataValidate(spy).ServeHTTP(rec, req)

	if spy.Called {
		t.Fatal("el middleware dejo pasar una peticion sin token")
	}
	if res := testutil.DecodeResponse(t, rec); res.Tracking != "DVAUT020" {
		t.Errorf("tracking = %s, se esperaba DVAUT020", res.Tracking)
	}
}

// El privilegio se valida con la copia del usuario que trae la sesion, sin consultar MongoDB
func TestPrivilegesDataValidate(t *testing.T) {
	t.Parallel()

	superUser := cmm.UserPrivilegeSuperUser.Tag
	genericUser := cmm.UserPrivilegeGenericUser.Tag

	tests := []struct {
		name         string
		tagPrivilege *string // nil: sin sesion en el contexto
		allowedTags  []string
		wantNext     bool
		wantStatus   int
		wantTracking string
	}{
		{name: "super usuario", tagPrivilege: &superUser, allowedTags: []string{superUser}, wantNext: true},
		{name: "generico en ruta de ambos", tagPrivilege: &genericUser, allowedTags: []string{genericUser, superUser}, wantNext: true},
		{name: "generico en ruta de super usuario", tagPrivilege: &genericUser, allowedTags: []string{superUser}, wantStatus: http.StatusForbidden, wantTracking: "DVAUT006"},
		{name: "sin sesion en el contexto", allowedTags: []string{superUser}, wantStatus: http.StatusBadRequest, wantTracking: "DVAUT022"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.tagPrivilege != nil {
				session := models.SessionsModelType{RecordOwnerData: models.RecordOwnerData{TagTypePrivilege: *tt.tagPrivilege}}
				req = cmm.SetInRequestContext(req, "authSession", &session)
			}
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			dv.PrivilegesDataValidate(tt.allowedTags...)(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}
			if tt.wantNext {
				return
			}
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, se esperaba %d", rec.Code, tt.wantStatus)
			}
			if res := testutil.DecodeResponse(t, rec); res.Tracking != tt.wantTracking {
				t.Errorf("tracking = %s, se esperaba %s", res.Tracking, tt.wantTracking)
			}
		})
	}
}

// Solo se prueban los casos que no consultan MongoDB
func TestOptionalAuthDataValidate(t *testing.T) {
	t.Parallel()

	t.Run("sin token sigue como anonimo", func(t *testing.T) {
		t.Parallel()

		req := cmm.SetInRequestContext(httptest.NewRequest(http.MethodGet, "/", nil), "sessionToken", "")
		rec := httptest.NewRecorder()
		spy := &testutil.NextSpy{}

		dv.OptionalAuthDataValidate(spy).ServeHTTP(rec, req)

		if !spy.Called {
			t.Fatalf("next no fue llamado (body: %s)", rec.Body.String())
		}
		session, err := cmm.GetFromRequestContext[*models.SessionsModelType](spy.Request, "authSession")
		testutil.RequireEmptyError(t, err)
		if session != nil {
			t.Errorf("authSession = %+v, se esperaba nil para el usuario sin registro", session)
		}
	})

	t.Run("sin cmm.OptionalAuthenticateRequestMiddleware", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		spy := &testutil.NextSpy{}

		dv.OptionalAuthDataValidate(spy).ServeHTTP(rec, req)

		if spy.Called {
			t.Fatal("el middleware dejo pasar una peticion sin sessionToken en el contexto")
		}
		if res := testutil.DecodeResponse(t, rec); res.Tracking != "DVAUT023" {
			t.Errorf("tracking = %s, se esperaba DVAUT023", res.Tracking)
		}
	})
}
