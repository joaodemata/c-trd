package common_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// samplePayload formato de prueba que usa las mismas reglas que los format validates reales
type samplePayload struct {
	IDTrigger string `json:"idTrigger" validate:"required,mongodb"`
	Qty       int    `json:"qty"`
}

func TestFormatValidateMiddleware(t *testing.T) {
	t.Parallel()

	validID := primitive.NewObjectID().Hex()

	tests := []struct {
		name         string
		body         string
		wantNext     bool
		wantTracking string
	}{
		{name: "payload valido", body: `{"idTrigger":"` + validID + `","qty":3}`, wantNext: true},
		{name: "body vacio falla en required", body: ``, wantTracking: "CC003"},
		{name: "id que no es ObjectID", body: `{"idTrigger":"abc"}`, wantTracking: "CC003"},
		{name: "tipo de dato incorrecto", body: `{"idTrigger":"` + validID + `","qty":"tres"}`, wantTracking: "CC001"},
		{name: "JSON mal formado", body: `{"idTrigger":`, wantTracking: "CC002"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			cmm.FormatValidateMiddleware[samplePayload](spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				if rec.Code != http.StatusBadRequest {
					t.Errorf("status = %d, se esperaba %d", rec.Code, http.StatusBadRequest)
				}
				if res := testutil.DecodeResponse(t, rec); res.Tracking != tt.wantTracking {
					t.Errorf("tracking = %s, se esperaba %s", res.Tracking, tt.wantTracking)
				}
				return
			}

			// El payload validado debe quedar en el contexto para el data validate
			payload, err := cmm.GetFromRequestContext[*samplePayload](spy.Request, "payload")
			testutil.RequireEmptyError(t, err)
			if payload.IDTrigger != validID || payload.Qty != 3 {
				t.Errorf("payload = %+v, se esperaba idTrigger %s y qty 3", payload, validID)
			}
		})
	}
}

// rulesPayload un campo por cada regla con mensaje propio
type rulesPayload struct {
	IDTrigger string            `json:"idTrigger" validate:"required,mongodb"`
	Qty       int               `json:"qty" validate:"omitempty,min=1,max=10"`
	Name      string            `json:"name" validate:"omitempty,max=3"`
	Tags      []string          `json:"tags" validate:"omitempty,min=2,unique"`
	Format    string            `json:"format" validate:"omitempty,uppersnake"`
	Kind      string            `json:"kind" validate:"omitempty,oneof=QUERY BODY"`
	Flag      bool              `json:"flag" validate:"isdefault"`
	Rate      float64           `json:"rate" validate:"gte=0"`
	Values    map[string]string `json:"values" validate:"omitempty,max=1"`
	Items     []string          `json:"items" validate:"dive,mongodb"`
}

// requireFieldErrors valida la estructura del data de un error de formato: un objeto por campo con su mensaje,
// la lista "details" con todos los mensajes y el mensaje general "error"
func requireFieldErrors(t *testing.T, data any, want map[string]string) {
	t.Helper()

	object, ok := data.(map[string]any)
	if !ok {
		t.Fatalf("data = %v, se esperaba un objeto", data)
	}
	if object["error"] != "Datos de entrada inválidos" {
		t.Errorf("error = %v, se esperaba el mensaje general", object["error"])
	}

	details, _ := object["details"].([]any)
	if len(details) != len(want) {
		t.Errorf("details = %v, se esperaban %d mensajes", details, len(want))
	}

	for field, message := range want {
		entry, _ := object[field].(map[string]any)
		if entry == nil || entry["details"] != message {
			t.Errorf("%s = %v, se esperaba {\"details\": %q}", field, object[field], message)
		}
		if !slices.Contains(details, any(message)) {
			t.Errorf("details no contiene %q", message)
		}
	}
}

func TestFormatValidateMiddleware_ErrorMessages(t *testing.T) {
	t.Parallel()

	validID := primitive.NewObjectID().Hex()

	tests := []struct {
		name         string
		body         string
		wantTracking string
		want         map[string]string
	}{
		{name: "obligatorio", body: `{}`, wantTracking: "CC003", want: map[string]string{
			"idTrigger": "El campo 'idTrigger' es obligatorio",
		}},
		{name: "reglas con mensaje propio", wantTracking: "CC003",
			body: `{"idTrigger":"abc","qty":11,"name":"abcd","tags":["a"],"format":"a-b","kind":"PATH","flag":true,"rate":-1,"values":{"a":"1","b":"2"},"items":["` + validID + `","x"]}`,
			want: map[string]string{
				"idTrigger": "El campo 'idTrigger' debe ser un ObjectID válido (24 caracteres hexadecimales)",
				"qty":       "El campo 'qty' debe ser menor o igual a 10",
				"name":      "El campo 'name' debe tener una cantidad de caracteres menor o igual a 3",
				"tags":      "El campo 'tags' debe tener una cantidad de elementos mayor o igual a 2",
				"format":    "El campo 'format' solo puede tener letras mayúsculas, números y guion bajo",
				"kind":      "El campo 'kind' debe ser uno de: QUERY, BODY",
				"flag":      "El campo 'flag' debe ser false",
				"rate":      "El campo 'rate' debe ser mayor o igual a 0",
				"values":    "El campo 'values' debe tener una cantidad de elementos menor o igual a 1",
				"items[1]":  "El campo 'items[1]' debe ser un ObjectID válido (24 caracteres hexadecimales)",
			}},
		{name: "valores repetidos", wantTracking: "CC003", body: `{"idTrigger":"` + validID + `","tags":["a","a"]}`, want: map[string]string{
			"tags": "El campo 'tags' no puede tener valores repetidos",
		}},
		{name: "minimo numerico", wantTracking: "CC003", body: `{"idTrigger":"` + validID + `","qty":-5}`, want: map[string]string{
			"qty": "El campo 'qty' debe ser mayor o igual a 1",
		}},
		{name: "texto en vez de entero", wantTracking: "CC001", body: `{"qty":"tres"}`, want: map[string]string{
			"qty": "Error de tipo en el campo 'qty': se esperaba un número entero pero se recibió un texto",
		}},
		{name: "decimal en vez de entero", wantTracking: "CC001", body: `{"qty":1.5}`, want: map[string]string{
			"qty": "Error de tipo en el campo 'qty': se esperaba un número entero pero se recibió el número 1.5",
		}},
		{name: "objeto en vez de arreglo", wantTracking: "CC001", body: `{"tags":{"a":1}}`, want: map[string]string{
			"tags": "Error de tipo en el campo 'tags': se esperaba un arreglo pero se recibió un objeto",
		}},
		{name: "numero en vez de booleano", wantTracking: "CC001", body: `{"flag":1}`, want: map[string]string{
			"flag": "Error de tipo en el campo 'flag': se esperaba un booleano (true o false) pero se recibió un número",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			cmm.FormatValidateMiddleware[rulesPayload](&testutil.NextSpy{}).ServeHTTP(rec, req)

			res := testutil.DecodeResponse(t, rec)
			if res.Tracking != tt.wantTracking {
				t.Fatalf("tracking = %s, se esperaba %s (body: %s)", res.Tracking, tt.wantTracking, rec.Body.String())
			}
			requireFieldErrors(t, res.Data, tt.want)

			// En un error de tipo el mensaje de la respuesta es el del campo
			if tt.wantTracking == "CC001" {
				for _, message := range tt.want {
					if res.Message != message {
						t.Errorf("message = %q, se esperaba %q", res.Message, message)
					}
				}
			}
		})
	}
}

// mixedPayload campos de query de cada tipo soportado y un campo del body
type mixedPayload struct {
	Limit  int       `json:"limit" in:"query" validate:"omitempty,min=1,max=100"`
	Ratio  float64   `json:"ratio" in:"query"`
	Active bool      `json:"active" in:"query"`
	From   time.Time `json:"from" in:"query"`
	Tags   []string  `json:"tags" in:"query" validate:"omitempty,unique"`
	Name   string    `json:"name" validate:"required"`
}

// queryOnlyPayload sin campos del body: el body no se lee
type queryOnlyPayload struct {
	Search string `json:"search" in:"query" validate:"required"`
}

func TestFormatValidateMiddleware_QueryAndBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		query        string
		body         string
		wantTracking string // vacio si es valida
		want         mixedPayload
		wantFields   map[string]string
	}{
		{name: "query y body", query: "?limit=5&ratio=1.5&active=true&from=2026-01-31&tags=a&tags=b", body: `{"name":"x"}`,
			want: mixedPayload{Limit: 5, Ratio: 1.5, Active: true, From: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC), Tags: []string{"a", "b"}, Name: "x"}},
		{name: "fecha RFC3339", query: "?from=2026-01-31T10:30:00Z", body: `{"name":"x"}`,
			want: mixedPayload{From: time.Date(2026, 1, 31, 10, 30, 0, 0, time.UTC), Name: "x"}},
		{name: "sin query params", body: `{"name":"x"}`, want: mixedPayload{Name: "x"}},
		{name: "el body no llena campos de query", query: "?limit=7", body: `{"name":"x","limit":50,"tags":["z"]}`,
			want: mixedPayload{Limit: 7, Name: "x"}},
		{name: "entero invalido", query: "?limit=diez", body: `{"name":"x"}`, wantTracking: "CC004", wantFields: map[string]string{
			"limit": "Error de tipo en el parámetro 'limit': se esperaba un número entero pero se recibió 'diez'",
		}},
		{name: "booleano invalido", query: "?active=si", body: `{"name":"x"}`, wantTracking: "CC004", wantFields: map[string]string{
			"active": "Error de tipo en el parámetro 'active': se esperaba un booleano (true o false) pero se recibió 'si'",
		}},
		{name: "fecha invalida", query: "?from=31/01/2026", body: `{"name":"x"}`, wantTracking: "CC004", wantFields: map[string]string{
			"from": "Error de tipo en el parámetro 'from': se esperaba una fecha (YYYY-MM-DD o RFC3339) pero se recibió '31/01/2026'",
		}},
		{name: "regla de un campo de query", query: "?limit=500&tags=a&tags=a", body: `{"name":"x"}`, wantTracking: "CC003", wantFields: map[string]string{
			"limit": "El campo 'limit' debe ser menor o igual a 100",
			"tags":  "El campo 'tags' no puede tener valores repetidos",
		}},
		{name: "regla de un campo del body", query: "?limit=5", body: `{}`, wantTracking: "CC003", wantFields: map[string]string{
			"name": "El campo 'name' es obligatorio",
		}},
		{name: "body mal formado", query: "?limit=5", body: `{"name":`, wantTracking: "CC002"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/"+tt.query, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			cmm.FormatValidateMiddleware[mixedPayload](spy).ServeHTTP(rec, req)

			if tt.wantTracking != "" {
				res := testutil.DecodeResponse(t, rec)
				if spy.Called || res.Tracking != tt.wantTracking {
					t.Fatalf("tracking = %s, se esperaba %s (body: %s)", res.Tracking, tt.wantTracking, rec.Body.String())
				}
				if tt.wantFields != nil {
					requireFieldErrors(t, res.Data, tt.wantFields)
				}
				return
			}

			if !spy.Called {
				t.Fatalf("next no fue llamado (body: %s)", rec.Body.String())
			}
			payload, err := cmm.GetFromRequestContext[*mixedPayload](spy.Request, "payload")
			testutil.RequireEmptyError(t, err)
			if !reflect.DeepEqual(*payload, tt.want) {
				t.Errorf("payload = %+v, want %+v", *payload, tt.want)
			}
		})
	}
}

func TestFormatValidateMiddleware_QueryOnlyIgnoresBody(t *testing.T) {
	t.Parallel()

	// Sin campos del body, un body invalido no se lee
	req := httptest.NewRequest(http.MethodGet, "/?search=trends", strings.NewReader(`{"search":`))
	rec := httptest.NewRecorder()
	spy := &testutil.NextSpy{}

	cmm.FormatValidateMiddleware[queryOnlyPayload](spy).ServeHTTP(rec, req)

	if !spy.Called {
		t.Fatalf("next no fue llamado (body: %s)", rec.Body.String())
	}
	payload, err := cmm.GetFromRequestContext[*queryOnlyPayload](spy.Request, "payload")
	testutil.RequireEmptyError(t, err)
	if payload.Search != "trends" {
		t.Errorf("search = %q, se esperaba trends", payload.Search)
	}
}

func TestFormatValidateMiddleware_PanicsOnInvalidFields(t *testing.T) {
	t.Parallel()

	type unsupportedIn struct {
		ID string `json:"id" in:"path"`
	}
	type unsupportedQueryType struct {
		Filters map[string]string `json:"filters" in:"query"`
	}

	tests := []struct {
		name     string
		register func()
	}{
		{"origen no soportado", func() { cmm.FormatValidateMiddleware[unsupportedIn](&testutil.NextSpy{}) }},
		{"tipo no soportado en query", func() { cmm.FormatValidateMiddleware[unsupportedQueryType](&testutil.NextSpy{}) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Un format validate mal definido debe detener la app al registrar la ruta
			defer func() {
				if recover() == nil {
					t.Fatal("se esperaba un panic al registrar la ruta")
				}
			}()
			tt.register()
		})
	}
}
