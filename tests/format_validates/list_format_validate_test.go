package format_validates_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestListCatalogsFormatValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		query    string
		wantNext bool
	}{
		{"acciones", "?typeCatalog=" + cmm.CatalogTypeAction, true},
		{"temporalidades", "?typeCatalog=" + cmm.CatalogTypeCandleStick, true},
		{"sin query", "", false},
		{"vacio", "?typeCatalog=", false},
		{"minusculas", "?typeCatalog=type_action", false},
		{"caracteres especiales", "?typeCatalog=TYPE-ACTION", false},
		{"demasiado largo", "?typeCatalog=" + strings.Repeat("A", 65), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/list/catalogs"+tt.query, nil)
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			fv.ListCatalogsFormatValidate(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				if res := testutil.DecodeResponse(t, rec); res.Tracking != "CC003" {
					t.Errorf("tracking = %s, se esperaba CC003", res.Tracking)
				}
				return
			}

			payload, errCtx := cmm.GetFromRequestContext[*fv.ListCatalogsFormat](spy.Request, "payload")
			testutil.RequireEmptyError(t, errCtx)
			if want := strings.TrimPrefix(tt.query, "?typeCatalog="); payload.TypeCatalog != want {
				t.Errorf("typeCatalog = %q, want %q", payload.TypeCatalog, want)
			}
		})
	}
}

func TestListTriggersFormatValidate(t *testing.T) {
	t.Parallel()

	const root = "6adcd1882c3b99e2480bc973"

	tests := []struct {
		name     string
		query    string
		wantNext bool
		want     fv.ListTriggersFormat
	}{
		{"sin filtros", "", true, fv.ListTriggersFormat{}},
		{"versiones de un trigger", "?showReferences=true&idReferenceRoot=" + root, true, fv.ListTriggersFormat{ShowReferences: true, IdReferenceRoot: root}},
		{"idReferenceRoot vacio", "?idReferenceRoot=", true, fv.ListTriggersFormat{}},
		{"idReferenceRoot invalido", "?idReferenceRoot=no-es-un-id", false, fv.ListTriggersFormat{}},
		{"idReferenceRoot corto", "?idReferenceRoot=6adcd188", false, fv.ListTriggersFormat{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/list/triggers"+tt.query, nil)
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			fv.ListTriggersFormatValidate(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				if res := testutil.DecodeResponse(t, rec); res.Tracking != "CC003" {
					t.Errorf("tracking = %s, se esperaba CC003", res.Tracking)
				}
				return
			}

			// Se inyecta en "filters" porque "payload" lo ocupa la paginacion
			filters, errCtx := cmm.GetFromRequestContext[*fv.ListTriggersFormat](spy.Request, "filters")
			testutil.RequireEmptyError(t, errCtx)
			if *filters != tt.want {
				t.Errorf("filters = %+v, want %+v", *filters, tt.want)
			}
		})
	}
}
