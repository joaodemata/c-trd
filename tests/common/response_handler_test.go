package common_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	cmm "c_trd/common"
	"c_trd/tests/testutil"
)

func TestResponseHandler_Send(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	cmm.NewResponseHandler(rec).Send(http.StatusCreated, "SUCCESS", "creado", "TEST001", map[string]any{"id": "1"})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, se esperaba %d", rec.Code, http.StatusCreated)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, se esperaba application/json", got)
	}

	res := testutil.DecodeResponse(t, rec)
	if res.Status != "SUCCESS" || res.Message != "creado" || res.Tracking != "TEST001" {
		t.Errorf("respuesta = %+v", res)
	}
	if data, ok := res.Data.(map[string]any); !ok || data["id"] != "1" {
		t.Errorf("data = %#v, se esperaba {id: 1}", res.Data)
	}
}

func TestResponseHandler_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         *cmm.ErrorHandler
		wantMessage string
	}{
		{
			name:        "error vacio usa el mensaje enviado",
			err:         cmm.NewEmptyErrorHandler(),
			wantMessage: "El trigger enviado no existe",
		},
		{
			name:        "error fatal oculta el detalle interno",
			err:         cmm.NewErrorHandler(errors.New("detalle interno"), "detalle interno", cmm.LevelFatal, "TESTE003"),
			wantMessage: "Servicio no disponible.",
		},
		{
			name:        "error de base de datos oculta el detalle interno",
			err:         cmm.NewErrorHandler(errors.New("timeout"), "timeout", cmm.LevelDatabase, "TESTE004"),
			wantMessage: "Servicio no disponible.",
		},
		{
			name:        "error de negocio muestra el mensaje enviado",
			err:         cmm.NewErrorHandler(nil, "El asset no tiene un CSV", cmm.LevelError, "TESTE007"),
			wantMessage: "El trigger enviado no existe",
		},
		{
			name:        "advertencia muestra el mensaje enviado",
			err:         cmm.NewErrorHandler(nil, "Archivo desactualizado", cmm.LevelWarning, "TESTE008"),
			wantMessage: "El trigger enviado no existe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			cmm.NewResponseHandler(rec).Error("FAIL", "El trigger enviado no existe", "TEST002", nil, tt.err)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, se esperaba %d", rec.Code, http.StatusBadRequest)
			}

			res := testutil.DecodeResponse(t, rec)
			if res.Message != tt.wantMessage {
				t.Errorf("message = %q, se esperaba %q", res.Message, tt.wantMessage)
			}
			if res.Tracking != "TEST002" {
				t.Errorf("tracking = %q, se esperaba TEST002", res.Tracking)
			}
		})
	}
}
