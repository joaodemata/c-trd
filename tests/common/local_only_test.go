package common_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	cmm "c_trd/common"
	"c_trd/tests/testutil"
)

func TestLocalOnlyMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		remoteAddr   string
		headers      map[string]string
		wantNext     bool
		wantTracking string
	}{
		{name: "loopback IPv4", remoteAddr: "127.0.0.1:50000", wantNext: true},
		{name: "loopback IPv6", remoteAddr: "[::1]:50000", wantNext: true},
		{name: "IP externa", remoteAddr: "192.0.2.10:50000", wantTracking: "CMMLOE001"},
		{name: "RemoteAddr invalido", remoteAddr: "no-es-una-ip", wantTracking: "CMMLOE001"},
		{
			name:         "loopback detras de un reverse proxy",
			remoteAddr:   "127.0.0.1:50000",
			headers:      map[string]string{"X-Forwarded-For": "203.0.113.5"},
			wantTracking: "CMMLOE002",
		},
		{
			name:         "loopback con header Forwarded",
			remoteAddr:   "127.0.0.1:50000",
			headers:      map[string]string{"Forwarded": "for=203.0.113.5"},
			wantTracking: "CMMLOE002",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			cmm.LocalOnlyMiddleware(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v", spy.Called, tt.wantNext)
			}
			if tt.wantNext {
				return
			}

			if rec.Code != http.StatusForbidden {
				t.Errorf("status = %d, se esperaba %d", rec.Code, http.StatusForbidden)
			}
			if res := testutil.DecodeResponse(t, rec); res.Tracking != tt.wantTracking {
				t.Errorf("tracking = %s, se esperaba %s", res.Tracking, tt.wantTracking)
			}
		})
	}
}
