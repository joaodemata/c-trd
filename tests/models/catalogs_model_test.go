package models_test

import (
	"testing"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// testOwner usuario fijo que crea los triggers, condiciones y oportunidades de las pruebas
var testOwner = models.RecordOwnerData{
	IDUser:                   primitive.NewObjectID(),
	IDTypePrivilege:          cmm.UserPrivilegeSuperUser.ID,
	TagTypePrivilege:         cmm.UserPrivilegeSuperUser.Tag,
	NameTypePrivilege:        cmm.UserPrivilegeSuperUser.Name,
	DescriptionTypePrivilege: "Privilegios de super usuario.",
}

// testVisibility visibilidad publica fija de las pruebas. allowed_users es nil para validar que se guarde como []
var testVisibility = models.RecordVisibilityData{
	IDTypeVisibility:   primitive.NewObjectID(),
	TagTypeVisibility:  cmm.VisibilityTagPublic,
	NameTypeVisibility: "Publica",
}

// visibilityCatalog catalogo TYPE_VISIBILITY como lo retorna la base de datos
func visibilityCatalog(tag string) models.CatalogsModelType {
	return models.CatalogsModelType{
		ID:          primitive.NewObjectID(),
		TypeCatalog: cmm.CatalogTypeVisibility,
		Tag:         tag,
		Name:        "Visibilidad " + tag,
		Description: "Descripcion de " + tag,
	}
}

func TestNewRecordVisibilityData(t *testing.T) {
	t.Parallel()

	users := []primitive.ObjectID{primitive.NewObjectID(), primitive.NewObjectID()}

	tests := []struct {
		name         string
		tag          string
		allowedUsers []primitive.ObjectID
		wantTracking string
		wantUsers    int
	}{
		{name: "privada", tag: cmm.VisibilityTagPrivate},
		{name: "publica con usuarios vacios", tag: cmm.VisibilityTagPublic, allowedUsers: []primitive.ObjectID{}},
		{name: "lista blanca con usuarios", tag: cmm.VisibilityTagWhitelist, allowedUsers: users, wantUsers: 2},
		{name: "tag no soportado", tag: "VISIBILITY-TEAM", wantTracking: "MCATE001"},
		{name: "lista blanca sin usuarios", tag: cmm.VisibilityTagWhitelist, wantTracking: "MCATE002"},
		{name: "privada con usuarios", tag: cmm.VisibilityTagPrivate, allowedUsers: users, wantTracking: "MCATE003"},
		{name: "publica con usuarios", tag: cmm.VisibilityTagPublic, allowedUsers: users, wantTracking: "MCATE003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			catalog := visibilityCatalog(tt.tag)
			visibility, err := models.NewRecordVisibilityData(catalog, tt.allowedUsers)

			if tt.wantTracking != "" {
				testutil.RequireTrackingCode(t, err, tt.wantTracking)
				return
			}

			testutil.RequireEmptyError(t, err)
			if visibility.IDTypeVisibility != catalog.ID || visibility.TagTypeVisibility != catalog.Tag ||
				visibility.NameTypeVisibility != catalog.Name || visibility.DescriptionTypeVisibility != catalog.Description {
				t.Errorf("visibilidad = %+v, se esperaba la copia de %+v", visibility, catalog)
			}
			// Sin usuarios se guarda [] y no null
			if visibility.AllowedUsers == nil || len(visibility.AllowedUsers) != tt.wantUsers {
				t.Errorf("allowed_users = %v, se esperaban %d usuarios", visibility.AllowedUsers, tt.wantUsers)
			}
		})
	}
}

// TestVisibleToUserFilter el usuario ve las publicas, las suyas y las de lista blanca donde esta
func TestVisibleToUserFilter(t *testing.T) {
	t.Parallel()

	idUser := primitive.NewObjectID()
	filter := models.VisibleToUserFilter(idUser)

	or, ok := filter["$or"].([]bson.M)
	if !ok || len(or) != 3 {
		t.Fatalf("filtro = %v, se esperaba un $or con 3 opciones", filter)
	}
	if or[0]["tag_type_visibility"] != cmm.VisibilityTagPublic || len(or[0]) != 1 {
		t.Errorf("$or[0] = %v, se esperaban las publicas", or[0])
	}
	if or[1]["id_user"] != idUser || len(or[1]) != 1 {
		t.Errorf("$or[1] = %v, se esperaban las del usuario", or[1])
	}
	if or[2]["tag_type_visibility"] != cmm.VisibilityTagWhitelist || or[2]["allowed_users"] != idUser || len(or[2]) != 2 {
		t.Errorf("$or[2] = %v, se esperaban las de lista blanca con el usuario", or[2])
	}
}

// TestModels_OwnerAndVisibility los triggers, condiciones y oportunidades guardan el usuario y la visibilidad en el
// primer nivel del documento y allowed_users como [] aunque llegue nil
func TestModels_OwnerAndVisibility(t *testing.T) {
	t.Parallel()

	condition, err := newTestCondition(3600, nil)
	testutil.RequireEmptyError(t, err)

	tests := []struct {
		name   string
		record any
	}{
		{"triggers", newTestTrigger(t, nil)},
		{"conditions", condition},
		{"opportunities", newTestOpportunity(t, 6, "4-HOURS", nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			doc, errMarshal := bson.Marshal(tt.record)
			if errMarshal != nil {
				t.Fatalf("bson.Marshal: %v", errMarshal)
			}
			raw := bson.Raw(doc)

			if got, ok := raw.Lookup("id_user").ObjectIDOK(); !ok || got != testOwner.IDUser {
				t.Errorf("id_user = %v, se esperaba %s", got, testOwner.IDUser.Hex())
			}
			if got, _ := raw.Lookup("tag_type_privilege").StringValueOK(); got != testOwner.TagTypePrivilege {
				t.Errorf("tag_type_privilege = %q, se esperaba %q", got, testOwner.TagTypePrivilege)
			}
			if got, _ := raw.Lookup("tag_type_visibility").StringValueOK(); got != testVisibility.TagTypeVisibility {
				t.Errorf("tag_type_visibility = %q, se esperaba %q", got, testVisibility.TagTypeVisibility)
			}
			for _, key := range []string{"id_type_privilege", "name_type_privilege", "description_type_privilege", "id_type_visibility", "name_type_visibility", "description_type_visibility"} {
				if raw.Lookup(key).Type == 0 {
					t.Errorf("falta %s en el documento", key)
				}
			}

			allowedUsers := raw.Lookup("allowed_users")
			if allowedUsers.Type != bsontype.Array {
				t.Fatalf("allowed_users es %v, se esperaba un arreglo", allowedUsers.Type)
			}
			if values, _ := allowedUsers.Array().Values(); len(values) != 0 {
				t.Errorf("allowed_users tiene %d elementos, se esperaba []", len(values))
			}

			// Al frontend tambien van en el primer nivel
			requireHiddenKeys(t, jsonKeys(t, tt.record), nil,
				[]string{"idUser", "idTypePrivilege", "tagTypePrivilege", "nameTypePrivilege", "descriptionTypePrivilege",
					"idTypeVisibility", "tagTypeVisibility", "nameTypeVisibility", "descriptionTypeVisibility", "allowedUsers"},
			)
		})
	}
}
