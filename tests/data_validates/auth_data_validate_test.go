package data_validates_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	cmm "c_trd/common"
	dv "c_trd/data_validates"
	"c_trd/tests/testutil"
)

// Solo se prueban los casos que fallan antes de consultar MongoDB (header Authorization sin token)
func TestAuthMiddlewares_RejectWithoutToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		authorization string
	}{
		{"sin header", ""},
		{"sin Bearer", "Token abc"},
		{"Bearer en minusculas", "bearer abc"},
		{"Bearer vacio", "Bearer   "},
	}

	middlewares := map[string]func(http.Handler) http.Handler{
		"AuthDataValidate":       dv.AuthDataValidate,
		"PrivilegesDataValidate": dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag),
	}

	for middlewareName, middleware := range middlewares {
		for _, tt := range tests {
			t.Run(middlewareName+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				req := httptest.NewRequest(http.MethodGet, "/", nil)
				if tt.authorization != "" {
					req.Header.Set("Authorization", tt.authorization)
				}
				rec := httptest.NewRecorder()
				spy := &testutil.NextSpy{}

				middleware(spy).ServeHTTP(rec, req)

				if spy.Called {
					t.Fatal("el middleware dejo pasar una peticion sin token")
				}
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("status = %d, se esperaba %d", rec.Code, http.StatusUnauthorized)
				}
				if res := testutil.DecodeResponse(t, rec); res.Tracking != "DVAUT001" {
					t.Errorf("tracking = %s, se esperaba DVAUT001", res.Tracking)
				}
			})
		}
	}
}
