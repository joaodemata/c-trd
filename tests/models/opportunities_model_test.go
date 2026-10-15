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
		{"6 velas de 4 horas", 6, "4H", 24 * time.Hour},
		{"10 velas de 15 minutos", 10, "15m", 150 * time.Minute},
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

	opp := newTestOpportunity(t, 6, "4H", nil)

	if opp.ID.IsZero() {
		t.Error("el ID debe generarse en el constructor")
	}
	if opp.IDStatus != cmm.OpportunityStatusOngoing.ID || opp.TagStatus != cmm.OpportunityStatusOngoing.Tag {
		t.Errorf("status = %s, toda oportunidad nace en %s", opp.TagStatus, cmm.OpportunityStatusOngoing.Tag)
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
		conditions := []models.Condition{{IDCondition: primitive.NewObjectID(), TagStatus: cmm.GenericStatusInactive.Tag}}

		opp := newTestOpportunity(t, 6, "4H", &models.OpportunityOptionalData{ID: id, Conditions: conditions})

		if opp.ID != id {
			t.Errorf("ID = %v, se esperaba %v", opp.ID, id)
		}
		if len(opp.Conditions) != 1 || opp.Conditions[0].IDCondition != conditions[0].IDCondition {
			t.Errorf("conditions = %+v, se esperaba %+v", opp.Conditions, conditions)
		}
	})

	t.Run("genera el ID si no se envia", func(t *testing.T) {
		t.Parallel()

		opp := newTestOpportunity(t, 6, "4H", &models.OpportunityOptionalData{})

		if opp.ID.IsZero() {
			t.Error("el ID debe generarse cuando opts no trae uno")
		}
		if opp.Conditions == nil {
			t.Error("sin condiciones en opts debe quedar un slice vacio")
		}
	})
}
