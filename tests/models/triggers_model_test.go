package models_test

import (
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// newTestTrigger crea un trigger con datos fijos, solo varian los datos opcionales
func newTestTrigger(t *testing.T, opts *models.TriggerOptionalData) models.TriggersModelType {
	t.Helper()

	trigger, err := models.NewTriggerModel(
		"TRIGGER-RSI", "RSI", "RSI en sobreventa",
		primitive.NewObjectID(), "ASSET-BTC", "Bitcoin",
		primitive.NewObjectID(), "ACTION-BUY", "Compra",
		6, primitive.NewObjectID(), "4-HOURS", "4 Horas",
		opts,
	)
	testutil.RequireEmptyError(t, err)
	return trigger
}

func TestNewTriggerModel_Defaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts *models.TriggerOptionalData
	}{
		{"sin datos opcionales", nil},
		{"datos opcionales vacios", &models.TriggerOptionalData{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			trigger := newTestTrigger(t, tt.opts)

			if trigger.ID.IsZero() {
				t.Error("el ID debe generarse en el constructor")
			}
			if trigger.IDStatus != cmm.GenericStatusActive.ID || trigger.TagStatus != cmm.GenericStatusActive.Tag {
				t.Errorf("status = %s, todo trigger nace en %s", trigger.TagStatus, cmm.GenericStatusActive.Tag)
			}
			if trigger.LogicalDelete {
				t.Error("un trigger nuevo no puede estar borrado")
			}
			if trigger.CreatedDate.Location() != time.UTC {
				t.Errorf("created_date = %v, las fechas se guardan en UTC", trigger.CreatedDate)
			}

			// Integridad de la data: arreglos opcionales como [] y escalares opcionales como null
			doc, err := bson.Marshal(trigger)
			if err != nil {
				t.Fatalf("bson.Marshal: %v", err)
			}
			raw := bson.Raw(doc)

			conditions := raw.Lookup("conditions")
			if conditions.Type != bsontype.Array {
				t.Fatalf("conditions es %v, se esperaba un arreglo", conditions.Type)
			}
			if values, _ := conditions.Array().Values(); len(values) != 0 {
				t.Errorf("conditions tiene %d elementos, se esperaba []", len(values))
			}
			for _, key := range []string{"id_reference", "id_reference_root", "pine_code"} {
				if got := raw.Lookup(key).Type; got != bsontype.Null {
					t.Errorf("%s es %v, se esperaba null", key, got)
				}
			}
		})
	}
}

func TestNewTriggerModel_OptionalData(t *testing.T) {
	t.Parallel()

	first := models.ConditionsModelType{
		ID: primitive.NewObjectID(), IDProvider: primitive.NewObjectID(),
		NameProvider: "Alpha Vantage", NameCondition: "Put/Call", DescriptionCondition: "Ratio alto",
	}
	second := models.ConditionsModelType{ID: primitive.NewObjectID(), NameCondition: "Trends"}
	idReference := primitive.NewObjectID()
	idReferenceRoot := primitive.NewObjectID()
	pineCode := "//@version=5"

	trigger := newTestTrigger(t, &models.TriggerOptionalData{
		Conditions:      []models.ConditionsModelType{first, second},
		IdReference:     &idReference,
		IdReferenceRoot: &idReferenceRoot,
		PineCode:        &pineCode,
	})

	if len(trigger.Conditions) != 2 {
		t.Fatalf("conditions = %d, se esperaban 2", len(trigger.Conditions))
	}
	// Se conserva el orden y se copian los datos de la condicion
	got := trigger.Conditions[0]
	if got.IDCondition != first.ID || got.IDProvider != first.IDProvider || got.NameProvider != first.NameProvider ||
		got.NameCondition != first.NameCondition || got.DescriptionCondition != first.DescriptionCondition {
		t.Errorf("conditions[0] = %+v, se esperaba la copia de %+v", got, first)
	}
	if trigger.Conditions[1].IDCondition != second.ID {
		t.Errorf("conditions[1] = %s, se esperaba %s", trigger.Conditions[1].IDCondition.Hex(), second.ID.Hex())
	}
	if trigger.IdReference == nil || *trigger.IdReference != idReference {
		t.Errorf("idReference = %v, se esperaba %s", trigger.IdReference, idReference.Hex())
	}
	if trigger.IdReferenceRoot == nil || *trigger.IdReferenceRoot != idReferenceRoot {
		t.Errorf("idReferenceRoot = %v, se esperaba %s", trigger.IdReferenceRoot, idReferenceRoot.Hex())
	}
	if trigger.PineCode == nil || *trigger.PineCode != pineCode {
		t.Errorf("pineCode = %v, se esperaba %q", trigger.PineCode, pineCode)
	}
}

// TestTriggerReferenceRoot la raiz se asigna con el primer reemplazo y se reutiliza en los siguientes
func TestTriggerReferenceRoot(t *testing.T) {
	t.Parallel()

	original := newTestTrigger(t, nil)

	// Primer reemplazo: el original no tiene raiz, el original es la raiz
	firstRoot := models.TriggerReferenceRoot(original)
	if firstRoot != original.ID {
		t.Fatalf("root = %s, se esperaba el id del original %s", firstRoot.Hex(), original.ID.Hex())
	}

	idReference := original.ID
	second := newTestTrigger(t, &models.TriggerOptionalData{IdReference: &idReference, IdReferenceRoot: &firstRoot})

	// Segundo reemplazo: se reutiliza la raiz y no el id del trigger reemplazado
	if got := models.TriggerReferenceRoot(second); got != original.ID {
		t.Errorf("root = %s, se esperaba la raiz %s y no %s", got.Hex(), original.ID.Hex(), second.ID.Hex())
	}
}
