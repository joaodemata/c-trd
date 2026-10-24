package common_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/tests/testutil"
)

// TestPaginationFormatValidate formato general de los listados paginados (oportunidades, APIs, assets y condiciones)
func TestPaginationFormatValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		query        string
		wantTracking string // vacio si es valida
		want         cmm.PaginationFormat
	}{
		{name: "sin filtros", query: ""},
		{name: "todos los filtros", query: "?limit=20&page=2&search=trends&startDate=2026-01-01&endDate=2026-02-01T00:00:00Z",
			want: cmm.PaginationFormat{Limit: 20, Page: 2, Search: "trends",
				StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}},
		{name: "limite maximo", query: "?limit=100", want: cmm.PaginationFormat{Limit: 100}},
		{name: "limite mayor a 100", query: "?limit=101", wantTracking: "CC003"},
		{name: "limite negativo", query: "?limit=-1", wantTracking: "CC003"},
		{name: "pagina cero", query: "?page=0", want: cmm.PaginationFormat{}},
		{name: "pagina negativa", query: "?page=-2", wantTracking: "CC003"},
		{name: "busqueda demasiado larga", query: "?search=" + strings.Repeat("a", 101), wantTracking: "CC003"},
		{name: "limite como texto", query: "?limit=diez", wantTracking: "CC004"},
		{name: "fecha invalida", query: "?startDate=ayer", wantTracking: "CC004"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Los filtros vienen por query, el body se ignora
			req := httptest.NewRequest(http.MethodGet, "/list/apis"+tt.query, strings.NewReader(`{"limit":5}`))
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			cmm.PaginationFormatValidate(spy).ServeHTTP(rec, req)

			if tt.wantTracking != "" {
				res := testutil.DecodeResponse(t, rec)
				if spy.Called || res.Tracking != tt.wantTracking {
					t.Fatalf("tracking = %s, se esperaba %s (body: %s)", res.Tracking, tt.wantTracking, rec.Body.String())
				}
				return
			}

			if !spy.Called {
				t.Fatalf("next no fue llamado (body: %s)", rec.Body.String())
			}
			payload, errCtx := cmm.GetFromRequestContext[*cmm.PaginationFormat](spy.Request, "payload")
			testutil.RequireEmptyError(t, errCtx)
			if *payload != tt.want {
				t.Errorf("payload = %+v, want %+v", *payload, tt.want)
			}
		})
	}
}
