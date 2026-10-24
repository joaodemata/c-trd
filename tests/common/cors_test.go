package common_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	cmm "c_trd/common"
	"c_trd/tests/testutil"
)

func TestCorsMiddleware(t *testing.T) {
	t.Parallel()

	allowed := []string{"http://localhost:5173"}

	tests := []struct {
		name         string
		method       string
		origin       string
		preflight    bool
		wantNext     bool
		wantStatus   int
		wantAllow    string
		wantTracking string
	}{
		{name: "GET desde origen permitido", method: http.MethodGet, origin: "http://localhost:5173", wantNext: true, wantAllow: "http://localhost:5173"},
		{name: "GET desde origen no permitido", method: http.MethodGet, origin: "http://evil.test", wantNext: true},
		{name: "GET sin Origin", method: http.MethodGet, wantNext: true},
		{name: "preflight permitido", method: http.MethodOptions, origin: "http://localhost:5173", preflight: true, wantStatus: http.StatusNoContent, wantAllow: "http://localhost:5173"},
		{name: "preflight no permitido", method: http.MethodOptions, origin: "http://evil.test", preflight: true, wantStatus: http.StatusForbidden, wantTracking: "CMMCOE001"},
		{name: "OPTIONS sin preflight pasa al mux", method: http.MethodOptions, origin: "http://localhost:5173", wantNext: true, wantAllow: "http://localhost:5173"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tt.method, "/list/opportunities", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.preflight {
				req.Header.Set("Access-Control-Request-Method", http.MethodGet)
			}
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			cmm.CorsMiddleware(allowed)(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v", spy.Called, tt.wantNext)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllow {
				t.Errorf("Access-Control-Allow-Origin = %q, se esperaba %q", got, tt.wantAllow)
			}
			if tt.wantNext {
				return
			}

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, se esperaba %d", rec.Code, tt.wantStatus)
			}
			if tt.wantTracking != "" {
				if res := testutil.DecodeResponse(t, rec); res.Tracking != tt.wantTracking {
					t.Errorf("tracking = %s, se esperaba %s", res.Tracking, tt.wantTracking)
				}
			}
		})
	}
}

func TestCorsOriginsFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "sin variable usa el origen de Vite", value: "", want: []string{"http://localhost:5173"}},
		{name: "lista separada por coma", value: "http://a.test, http://b.test", want: []string{"http://a.test", "http://b.test"}},
		{name: "ignora entradas vacias", value: " , http://a.test,", want: []string{"http://a.test"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("JM_CTRD_CORS_ORIGINS", tt.value)

			if got := cmm.CorsOriginsFromEnv(); !slices.Equal(got, tt.want) {
				t.Errorf("origenes = %v, se esperaba %v", got, tt.want)
			}
		})
	}
}
