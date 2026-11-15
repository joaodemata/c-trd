package models_test

import (
	"testing"

	"c_trd/models"
	"c_trd/tests/testutil"
)

func TestNewPortfolioModel(t *testing.T) {
	t.Parallel()

	portfolio, err := models.NewPortfolioModel("PORTFOLIO-LARGO-PLAZO", "Largo plazo", "Inversiones a mas de un año", testOwner)

	testutil.RequireEmptyError(t, err)
	if portfolio.ID.IsZero() || portfolio.LogicalDelete {
		t.Errorf("portfolio = %+v, debe tener ID y no estar borrado", portfolio)
	}
	if portfolio.IDUser != testOwner.IDUser || portfolio.TagTypePrivilege != testOwner.TagTypePrivilege {
		t.Errorf("dueño = %s %s, se esperaba el de la sesion", portfolio.IDUser.Hex(), portfolio.TagTypePrivilege)
	}
	if portfolio.Tag != "PORTFOLIO-LARGO-PLAZO" || portfolio.Name != "Largo plazo" {
		t.Errorf("tag = %q, nombre = %q", portfolio.Tag, portfolio.Name)
	}
	if portfolio.CreatedDate.IsZero() || portfolio.CreatedDate.Location().String() != "UTC" {
		t.Errorf("fecha de creacion = %v, se esperaba en UTC", portfolio.CreatedDate)
	}

	// La cartera se guarda como [] y no como null
	keys := jsonKeys(t, portfolio)
	if string(keys["assets"]) != "[]" {
		t.Errorf("assets = %s, se esperaba []", keys["assets"])
	}
	if _, ok := keys["idUser"]; !ok {
		t.Error("el dueño (idUser) debe enviarse en camelCase")
	}
}
