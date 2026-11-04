package models_test

import (
	"testing"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func newTestThesis(t *testing.T) models.AgentThesesModelType {
	t.Helper()

	thesis, err := models.NewAgentThesisModel(
		"BTC en miedo extremo con soporte semanal, horizonte 3 meses",
		primitive.NewObjectID(), "ASSET-BTC", "Bitcoin",
		primitive.NewObjectID(), "ACTION-BUY", "Compra",
		testOwner,
	)
	testutil.RequireEmptyError(t, err)
	return thesis
}

func TestNewAgentThesisModel(t *testing.T) {
	t.Parallel()

	thesis := newTestThesis(t)
	if thesis.ID.IsZero() || thesis.LogicalDelete {
		t.Errorf("tesis = %+v, debe tener ID y no estar borrada", thesis)
	}
	if thesis.IDStatus != cmm.AgentThesisStatusPending.ID || thesis.TagStatus != cmm.AgentThesisStatusPending.Tag {
		t.Errorf("status = %s, toda tesis nace pendiente", thesis.TagStatus)
	}
	if thesis.ClaimedAt != nil {
		t.Error("una tesis nueva no fue tomada por ninguna corrida")
	}
	if thesis.IDUser != testOwner.IDUser || thesis.CreatedDate.IsZero() {
		t.Errorf("dueño = %s, fecha = %v", thesis.IDUser.Hex(), thesis.CreatedDate)
	}
}
