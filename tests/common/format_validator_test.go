package common_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
