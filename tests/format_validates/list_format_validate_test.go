package format_validates_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/tests/testutil"
)

func TestListCallbacksFormatValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		query    string
		wantNext bool
	}{
		{"formato valido", "?responseFormat=" + cmm.ResponseFormatGoogleTrendsSeries, true},
		{"con numeros", "?responseFormat=FORMAT_2", true},
		{"sin query", "", false},
		{"vacio", "?responseFormat=", false},
		{"minusculas", "?responseFormat=google_trends_series", false},
		{"caracteres especiales", "?responseFormat=GOOGLE-TRENDS", false},
		{"demasiado largo", "?responseFormat=" + strings.Repeat("A", 65), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// El formato viene por query, el body se ignora
			req := httptest.NewRequest(http.MethodGet, "/list/callbacks"+tt.query, strings.NewReader(`{"responseFormat":"FROM_BODY"}`))
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			fv.ListCallbacksFormatValidate(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				if res := testutil.DecodeResponse(t, rec); res.Tracking != "CC003" {
					t.Errorf("tracking = %s, se esperaba CC003", res.Tracking)
				}
				return
			}

			payload, errCtx := cmm.GetFromRequestContext[*fv.ListCallbacksFormat](spy.Request, "payload")
			testutil.RequireEmptyError(t, errCtx)
			if want := strings.TrimPrefix(tt.query, "?responseFormat="); payload.ResponseFormat != want {
				t.Errorf("responseFormat = %q, want %q", payload.ResponseFormat, want)
			}
		})
	}
}

func TestListApisFormatValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		query        string
		wantTracking string // vacio si es valida
		want         fv.ListApisFormat
	}{
		{name: "sin filtros", query: ""},
		{name: "todos los filtros", query: "?limit=20&page=2&search=trends&startDate=2026-01-01&endDate=2026-02-01T00:00:00Z",
			want: fv.ListApisFormat{Limit: 20, Page: 2, Search: "trends",
				StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), EndDate: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}},
		{name: "limite maximo", query: "?limit=100", want: fv.ListApisFormat{Limit: 100}},
		{name: "limite mayor a 100", query: "?limit=101", wantTracking: "CC003"},
		{name: "limite negativo", query: "?limit=-1", wantTracking: "CC003"},
		{name: "pagina negativa", query: "?page=-2", wantTracking: "CC003"},
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

			fv.ListApisFormatValidate(spy).ServeHTTP(rec, req)

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
			payload, errCtx := cmm.GetFromRequestContext[*fv.ListApisFormat](spy.Request, "payload")
			testutil.RequireEmptyError(t, errCtx)
			if *payload != tt.want {
				t.Errorf("payload = %+v, want %+v", *payload, tt.want)
			}
		})
	}
}
