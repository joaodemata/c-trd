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

	idTrigger := primitive.NewObjectID().Hex()

	tests := []struct {
		name         string
		body         string
		wantNext     bool
		wantTracking string
		wantPrice    float64
	}{
		{"precio decimal", `{"idTrigger":"` + idTrigger + `","price":67012.34}`, true, "", 67012.34},
		{"precio con metadatos", `{"idTrigger":"` + idTrigger + `","price":1.0842,"metadatos":{"rsi":28}}`, true, "", 1.0842},
		{"sin precio", `{"idTrigger":"` + idTrigger + `"}`, false, "CC003", 0},
		{"precio cero", `{"idTrigger":"` + idTrigger + `","price":0}`, false, "CC003", 0},
		{"precio negativo", `{"idTrigger":"` + idTrigger + `","price":-5}`, false, "CC003", 0},
		{"precio como texto", `{"idTrigger":"` + idTrigger + `","price":"67012.34"}`, false, "CC001", 0},
		{"sin trigger", `{"price":67012.34}`, false, "CC003", 0},
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
		})
	}
}
