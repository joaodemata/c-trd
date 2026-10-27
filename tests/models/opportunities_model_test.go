package models_test

import (
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// newTestOpportunity crea una oportunidad con datos fijos, solo varian las velas y los datos opcionales
func newTestOpportunity(t *testing.T, maxCandles int, tagTimeframe string, opts *models.OpportunityOptionalData) models.OpportunitiesModelType {
	t.Helper()

	opp, err := models.NewOpportunityModel(
		primitive.NewObjectID(), "TRIGGER-RSI", "RSI", "RSI en sobreventa",
		primitive.NewObjectID(), "ASSET-BTC", "Bitcoin",
		primitive.NewObjectID(), "ACTION-BUY", "Compra",
		maxCandles, primitive.NewObjectID(), tagTimeframe, tagTimeframe,
		opts,
	)
	testutil.RequireEmptyError(t, err)
	return opp
}

func TestNewOpportunityModel_ExpirationDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		maxCandles   int
		tagTimeframe string
		want         time.Duration
	}{
		{"6 velas de 4 horas", 6, "4-HOURS", 24 * time.Hour},
		{"10 velas de 15 minutos", 10, "15-MINUTES", 150 * time.Minute},
		{"timeframe desconocido usa 1 hora", 3, "??", 3 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			before := time.Now()
			opp := newTestOpportunity(t, tt.maxCandles, tt.tagTimeframe, nil)
			after := time.Now()

			// La fecha se calcula con time.Now() dentro del constructor, se valida que caiga en la ventana
			if opp.ExpirationDate.Before(before.Add(tt.want)) || opp.ExpirationDate.After(after.Add(tt.want)) {
				t.Errorf("expiration = %v, se esperaba ahora + %v", opp.ExpirationDate, tt.want)
			}
		})
	}
}

func TestNewOpportunityModel_Defaults(t *testing.T) {
	t.Parallel()

	opp := newTestOpportunity(t, 6, "4-HOURS", nil)

	if opp.ID.IsZero() {
		t.Error("el ID debe generarse en el constructor")
	}
	// Los slices vacios se guardan como [] y no como null en Mongo
	if opp.Conditions == nil || opp.StopLoss == nil || opp.TakeProfit == nil {
		t.Error("conditions, stop_loss y take_profit deben ser slices vacios y no nil")
	}
	if opp.LogicalDelete {
		t.Error("una oportunidad nueva no puede estar borrada")
	}
}

func TestNewOpportunityModel_OptionalData(t *testing.T) {
	t.Parallel()

	t.Run("usa el ID y las condiciones enviadas", func(t *testing.T) {
		t.Parallel()

		id := primitive.NewObjectID()
		conditions := []models.ConditionsModelType{{
			ID:        primitive.NewObjectID(),
			Name:      "Put/Call AAPL",
			IDStatus:  cmm.GenericStatusActive.ID,
			TagStatus: cmm.GenericStatusActive.Tag,
			Status:    cmm.GenericStatusActive.Name,
		}}
		metadata := map[string]any{"price": 100.5}

		opp := newTestOpportunity(t, 6, "4-HOURS", &models.OpportunityOptionalData{ID: id, Conditions: conditions, Metadata: metadata})

		if opp.ID != id {
			t.Errorf("ID = %v, se esperaba %v", opp.ID, id)
		}
		// Se copia el status actual de la condicion, no nace inactiva
		if len(opp.Conditions) != 1 || opp.Conditions[0].IDCondition != conditions[0].ID || opp.Conditions[0].TagStatus != cmm.GenericStatusActive.Tag || opp.Conditions[0].NameCondition != conditions[0].Name {
			t.Errorf("conditions = %+v, se esperaba la copia de %+v", opp.Conditions, conditions)
		}
		if got, ok := opp.Metadata.(map[string]any); !ok || got["price"] != 100.5 {
			t.Errorf("metadata = %v, se esperaba %v", opp.Metadata, metadata)
		}
		// Sin stop loss ni take profit en opts se mantienen como [] y no null
		if opp.StopLoss == nil || opp.TakeProfit == nil {
			t.Error("stop_loss y take_profit deben ser slices vacios y no nil")
		}
	})

	t.Run("genera el ID si no se envia", func(t *testing.T) {
		t.Parallel()

		opp := newTestOpportunity(t, 6, "4-HOURS", &models.OpportunityOptionalData{})

		if opp.ID.IsZero() {
			t.Error("el ID debe generarse cuando opts no trae uno")
		}
		if opp.Conditions == nil {
			t.Error("sin condiciones en opts debe quedar un slice vacio")
		}
	})
}

// TestNewOpportunityModel_Status con condiciones nace en curso hasta que se cumplan; sin condiciones nace activa
func TestNewOpportunityModel_Status(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts *models.OpportunityOptionalData
		want cmm.CatalogStatus
	}{
		{"sin opts", nil, cmm.OpportunityStatusActive},
		{"opts sin condiciones", &models.OpportunityOptionalData{}, cmm.OpportunityStatusActive},
		{"con condiciones", &models.OpportunityOptionalData{Conditions: []models.ConditionsModelType{{ID: primitive.NewObjectID()}}}, cmm.OpportunityStatusOngoing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opp := newTestOpportunity(t, 6, "4-HOURS", tt.opts)
			if opp.IDStatus != tt.want.ID || opp.TagStatus != tt.want.Tag || opp.Status != tt.want.Name {
				t.Errorf("status = %s, se esperaba %s", opp.TagStatus, tt.want.Tag)
			}
		})
	}
}

// TestNewOpportunityModel_Prices el precio de TradingView siempre se guarda; el precio de activacion solo si nace activa
func TestNewOpportunityModel_Prices(t *testing.T) {
	t.Parallel()

	priceTriggered := 100.5
	priceActive := 101.25
	conditions := []models.ConditionsModelType{{ID: primitive.NewObjectID()}}

	tests := []struct {
		name            string
		opts            *models.OpportunityOptionalData
		wantTriggered   *float64
		wantActivePrice *float64
	}{
		{"sin opts", nil, nil, nil},
		{"nace activa", &models.OpportunityOptionalData{PriceTriggered: &priceTriggered, PriceActiveOpportunity: &priceActive}, &priceTriggered, &priceActive},
		{"nace activa sin precio de activacion", &models.OpportunityOptionalData{PriceTriggered: &priceTriggered}, &priceTriggered, nil},
		{"en curso ignora el precio de activacion", &models.OpportunityOptionalData{Conditions: conditions, PriceTriggered: &priceTriggered, PriceActiveOpportunity: &priceActive}, &priceTriggered, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opp := newTestOpportunity(t, 6, "4-HOURS", tt.opts)

			if !equalPrice(opp.PriceTriggered, tt.wantTriggered) {
				t.Errorf("priceTriggered = %v, se esperaba %v", opp.PriceTriggered, tt.wantTriggered)
			}
			if !equalPrice(opp.PriceActiveOportunity, tt.wantActivePrice) {
				t.Errorf("priceActiveOpportunity = %v, se esperaba %v", opp.PriceActiveOportunity, tt.wantActivePrice)
			}
			if opp.PriceAchieved != nil {
				t.Errorf("priceAchieved = %v, una oportunidad nueva no tiene precio alcanzado", *opp.PriceAchieved)
			}
		})
	}
}

// equalPrice compara dos precios opcionales por valor
func equalPrice(got, want *float64) bool {
	if got == nil || want == nil {
		return got == want
	}
	return *got == *want
}
