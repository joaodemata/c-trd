package common_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/tests/testutil"
)

// sendRateLimited hace una peticion por el middleware y devuelve el recorder y si paso al siguiente handler
func sendRateLimited(handler func(http.Handler) http.Handler, remoteAddr string, forwardedFor string) (*httptest.ResponseRecorder, bool) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	rec := httptest.NewRecorder()
	spy := &testutil.NextSpy{}

	handler(spy).ServeHTTP(rec, req)
	return rec, spy.Called
}

func TestRateLimitMiddlewareBlocksAfterLimit(t *testing.T) {
	t.Parallel()

	limit := cmm.RateLimitMiddleware(2, time.Minute)

	for i := 1; i <= 2; i++ {
		if _, called := sendRateLimited(limit, "203.0.113.5:40000", ""); !called {
			t.Fatalf("peticion %d bloqueada, se esperaba que pasara", i)
		}
	}

	rec, called := sendRateLimited(limit, "203.0.113.5:40001", "")
	if called {
		t.Fatal("la tercera peticion paso, se esperaba bloqueada")
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, se esperaba %d", rec.Code, http.StatusTooManyRequests)
	}
	if res := testutil.DecodeResponse(t, rec); res.Tracking != "CMMRLE001" {
		t.Errorf("tracking = %s, se esperaba CMMRLE001", res.Tracking)
	}
	// 2 peticiones por minuto recargan un token cada 30 segundos
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || retryAfter < 1 || retryAfter > 30 {
		t.Errorf("Retry-After = %q, se esperaba entre 1 y 30", rec.Header().Get("Retry-After"))
	}

	// Otra IP tiene su propia cubeta
	if _, called := sendRateLimited(limit, "198.51.100.7:40000", ""); !called {
		t.Error("otra IP bloqueada, se esperaba que pasara")
	}
}

func TestRateLimitMiddlewareBehindProxy(t *testing.T) {
	t.Parallel()

	limit := cmm.RateLimitMiddleware(1, time.Minute)
	caddyAddr := "172.18.0.2:50000"

	if _, called := sendRateLimited(limit, caddyAddr, "203.0.113.5"); !called {
		t.Fatal("primer cliente bloqueado, se esperaba que pasara")
	}
	// Mismo proxy pero otro cliente: no debe compartir la cubeta del primero
	if _, called := sendRateLimited(limit, caddyAddr, "198.51.100.7"); !called {
		t.Fatal("segundo cliente bloqueado por la cubeta del primero")
	}
	if _, called := sendRateLimited(limit, caddyAddr, "203.0.113.5"); called {
		t.Error("el primer cliente supero su limite y paso")
	}
}

func TestRateLimitMiddlewareInvalidConfig(t *testing.T) {
	t.Parallel()

	// Valores invalidos se corrigen a 1 peticion por minuto
	limit := cmm.RateLimitMiddleware(0, 0)

	if _, called := sendRateLimited(limit, "203.0.113.5:40000", ""); !called {
		t.Fatal("primera peticion bloqueada, se esperaba que pasara")
	}
	if _, called := sendRateLimited(limit, "203.0.113.5:40000", ""); called {
		t.Error("segunda peticion paso, se esperaba bloqueada")
	}
}
