package format_validates_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cmm "c_trd/common"
	fv "c_trd/format_validates"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestTradingViewTriggerFormatValidate(t *testing.T) {
	t.Parallel()

	idWebhook := primitive.NewObjectID().Hex()
	idCancel := primitive.NewObjectID().Hex()

	tests := []struct {
		name         string
		body         string
		wantNext     bool
		wantTracking string
		wantPrice    float64
		wantIdCancel string
	}{
		{"precio decimal", `{"idWebhook":"` + idWebhook + `","price":67012.34}`, true, "", 67012.34, ""},
		{"precio con metadatos", `{"idWebhook":"` + idWebhook + `","price":1.0842,"metadatos":{"rsi":28}}`, true, "", 1.0842, ""},
		{"sin precio", `{"idWebhook":"` + idWebhook + `"}`, false, "CC003", 0, ""},
		{"precio cero", `{"idWebhook":"` + idWebhook + `","price":0}`, false, "CC003", 0, ""},
		{"precio negativo", `{"idWebhook":"` + idWebhook + `","price":-5}`, false, "CC003", 0, ""},
		{"precio como texto numerico", `{"idWebhook":"` + idWebhook + `","price":"67012.34"}`, true, "", 67012.34, ""},
		{"precio como texto no numerico", `{"idWebhook":"` + idWebhook + `","price":"alto"}`, false, "CC001", 0, ""},
		{"sin webhook", `{"price":67012.34}`, false, "CC003", 0, ""},
		{"cancelacion", `{"idWebhook":"` + idWebhook + `","price":67012.34,"idCancel":"` + idCancel + `"}`, true, "", 67012.34, idCancel},
		{"cancelacion con id invalido", `{"idWebhook":"` + idWebhook + `","price":67012.34,"idCancel":"abc"}`, false, "CC003", 0, ""},
		{"cancelacion sin precio", `{"idWebhook":"` + idWebhook + `","idCancel":"` + idCancel + `"}`, false, "CC003", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/hook/trading_view", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			fv.TradingViewTriggerFormatValidate(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				if res := testutil.DecodeResponse(t, rec); res.Tracking != tt.wantTracking {
					t.Errorf("tracking = %s, se esperaba %s", res.Tracking, tt.wantTracking)
				}
				return
			}

			payload, err := cmm.GetFromRequestContext[*fv.TradingViewTriggerFormat](spy.Request, "payload")
			testutil.RequireEmptyError(t, err)
			if payload.Price != tt.wantPrice {
				t.Errorf("price = %v, se esperaba %v", payload.Price, tt.wantPrice)
			}
			if payload.IdCancel != tt.wantIdCancel {
				t.Errorf("idCancel = %q, se esperaba %q", payload.IdCancel, tt.wantIdCancel)
			}
		})
	}
}

func TestTradingViewTriggerFormatValidateRiskManagement(t *testing.T) {
	t.Parallel()

	base := `{"idWebhook":"` + primitive.NewObjectID().Hex() + `","price":100,`

	tests := []struct {
		name           string
		body           string
		wantNext       bool
		wantTracking   string
		wantStopLoss   int
		wantTakeProfit int
	}{
		{"sin gestion de riesgo", base + `"metadatos":{}}`, true, "", 0, 0},
		{"stop loss y take profit", base + `"stopLoss":[{"price":90,"probability":0.3}],"takeProfit":[{"price":110,"probability":0.5},{"price":120}]}`, true, "", 1, 2},
		{"precios como texto", base + `"stopLoss":[{"price":"90","probability":"0.3"}]}`, true, "", 1, 0},
		{"arreglos vacios", base + `"stopLoss":[],"takeProfit":[]}`, true, "", 0, 0},
		{"objetivo sin precio", base + `"stopLoss":[{"probability":0.3}]}`, false, "CC003", 0, 0},
		{"objetivo con precio cero", base + `"takeProfit":[{"price":0}]}`, false, "CC003", 0, 0},
		{"probabilidad negativa", base + `"takeProfit":[{"price":110,"probability":-0.1}]}`, false, "CC003", 0, 0},
		{"stop loss no es arreglo", base + `"stopLoss":{"price":90}}`, false, "CC001", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/hook/trading_view", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			fv.TradingViewTriggerFormatValidate(spy).ServeHTTP(rec, req)

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				if res := testutil.DecodeResponse(t, rec); res.Tracking != tt.wantTracking {
					t.Errorf("tracking = %s, se esperaba %s", res.Tracking, tt.wantTracking)
				}
				return
			}

			payload, err := cmm.GetFromRequestContext[*fv.TradingViewTriggerFormat](spy.Request, "payload")
			testutil.RequireEmptyError(t, err)
			if len(payload.StopLoss) != tt.wantStopLoss {
				t.Errorf("stopLoss = %d objetivos, se esperaban %d", len(payload.StopLoss), tt.wantStopLoss)
			}
			if len(payload.TakeProfit) != tt.wantTakeProfit {
				t.Errorf("takeProfit = %d objetivos, se esperaban %d", len(payload.TakeProfit), tt.wantTakeProfit)
			}
		})
	}
}
