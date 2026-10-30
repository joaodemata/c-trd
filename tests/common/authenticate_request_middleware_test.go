package common_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	cmm "c_trd/common"
	"c_trd/tests/testutil"
)

func TestAuthenticateRequestMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		authorization string
		wantNext      bool
		wantToken     string
	}{
		{name: "con token", authorization: "Bearer abc123", wantNext: true, wantToken: "abc123"},
		{name: "token con espacios", authorization: "Bearer   abc123  ", wantNext: true, wantToken: "abc123"},
		{name: "sin header", authorization: ""},
		{name: "sin Bearer", authorization: "Token abc"},
		{name: "Bearer en minusculas", authorization: "bearer abc"},
		{name: "Bearer vacio", authorization: "Bearer   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			cmm.AuthenticateRequestMiddleware(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("status = %d, se esperaba %d", rec.Code, http.StatusUnauthorized)
				}
				if res := testutil.DecodeResponse(t, rec); res.Tracking != "CMMARE001" {
					t.Errorf("tracking = %s, se esperaba CMMARE001", res.Tracking)
				}
				return
			}

			token, err := cmm.GetFromRequestContext[string](spy.Request, "sessionToken")
			testutil.RequireEmptyError(t, err)
			if token != tt.wantToken {
				t.Errorf("token = %q, se esperaba %q", token, tt.wantToken)
			}
		})
	}
}

func TestOptionalAuthenticateRequestMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		authorization string
		wantNext      bool
		wantToken     string
	}{
		{name: "sin header sigue como anonimo", authorization: "", wantNext: true, wantToken: ""},
		{name: "con token", authorization: "Bearer abc123", wantNext: true, wantToken: "abc123"},
		{name: "sin Bearer", authorization: "Token abc"},
		{name: "Bearer vacio", authorization: "Bearer   "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			cmm.OptionalAuthenticateRequestMiddleware(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				if rec.Code != http.StatusUnauthorized {
					t.Errorf("status = %d, se esperaba %d", rec.Code, http.StatusUnauthorized)
				}
				if res := testutil.DecodeResponse(t, rec); res.Tracking != "CMMARE002" {
					t.Errorf("tracking = %s, se esperaba CMMARE002", res.Tracking)
				}
				return
			}

			// El token siempre queda en el contexto, vacio si el usuario no tiene registro
			token, err := cmm.GetFromRequestContext[string](spy.Request, "sessionToken")
			testutil.RequireEmptyError(t, err)
			if token != tt.wantToken {
				t.Errorf("token = %q, se esperaba %q", token, tt.wantToken)
			}
		})
	}
}
