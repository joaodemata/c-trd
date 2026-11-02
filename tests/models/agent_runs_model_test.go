package models_test

import (
	"testing"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/tests/testutil"
)

func TestNewAgentRunModel(t *testing.T) {
	t.Parallel()

	thesis := newTestThesis(t)
	run, err := models.NewAgentRunModel(thesis, agentOwner)
	testutil.RequireEmptyError(t, err)

	if run.ID.IsZero() || run.IDThesis != thesis.ID {
		t.Errorf("corrida = %s de la tesis %s, se esperaba la tesis %s", run.ID.Hex(), run.IDThesis.Hex(), thesis.ID.Hex())
	}
	if run.Thesis != thesis.Thesis || run.TagAsset != thesis.TagAsset || run.TagAction != thesis.TagAction {
		t.Errorf("la corrida debe copiar la tesis: %+v", run)
	}
	if run.TagStatus != cmm.AgentRunStatusRunning.Tag || run.FinishedAt != nil {
		t.Errorf("status = %s, toda corrida nace en ejecucion y sin fecha de fin", run.TagStatus)
	}
	// Las listas se guardan como [] y los opcionales como null
	if run.Steps == nil || run.IDConditions == nil || run.IDTrigger != nil || run.Error != nil {
		t.Errorf("corrida = %+v, listas vacias y opcionales nulos", run)
	}
	if run.IDUser != agentOwner.IDUser || run.TagTypePrivilege != cmm.UserPrivilegeAgent.Tag {
		t.Errorf("la corrida debe ser del usuario agente: %+v", run.RecordOwnerData)
	}
}
