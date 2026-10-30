package models_test

import (
	"slices"
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// genericPrivilege catalogo USER_PRIVILEGES GENERIC_USER como lo retorna la base de datos
func genericPrivilege() models.CatalogsModelType {
	return models.CatalogsModelType{
		ID:          cmm.UserPrivilegeGenericUser.ID,
		TypeCatalog: cmm.CatalogTypeUserPrivileges,
		Tag:         cmm.UserPrivilegeGenericUser.Tag,
		Name:        cmm.UserPrivilegeGenericUser.Name,
		Description: "Privilegios de un usuario genérico.",
	}
}

func TestNewUserModel_Defaults(t *testing.T) {
	t.Parallel()

	user, err := models.NewUserModel("jmata", genericPrivilege(), "hash", nil)

	testutil.RequireEmptyError(t, err)
	if user.ID.IsZero() {
		t.Error("el ID no se genero")
	}
	if user.Username != "jmata" || user.PasswordHash != "hash" {
		t.Errorf("usuario = %+v, se esperaba jmata con hash", user)
	}
	if user.IDTypePrivilege != cmm.UserPrivilegeGenericUser.ID || user.TagPrivilege != "GENERIC_USER" ||
		user.NamePrivilege != "Usuario genérico" || user.DescriptionPrivilege != "Privilegios de un usuario genérico." {
		t.Errorf("privilegio = %s %s %s %s, se esperaba el catalogo GENERIC_USER",
			user.IDTypePrivilege.Hex(), user.TagPrivilege, user.NamePrivilege, user.DescriptionPrivilege)
	}
	if !user.IsActive {
		t.Error("el usuario debe nacer activo")
	}
	if user.Email != nil || user.Name != nil || user.LastName != nil || user.IDCreatedBy != nil {
		t.Error("los campos opcionales deben quedar null")
	}
	if user.MaxTriggersQuantity != nil || user.MaxConditionsQuantity != nil {
		t.Error("los limites deben quedar null (sin limite)")
	}
	if user.AvailableApis == nil || len(user.AvailableApis) != 0 {
		t.Errorf("availableApis = %v, se esperaba []", user.AvailableApis)
	}
	if user.Config == nil || len(user.Config) != 0 {
		t.Errorf("config = %v, se esperaba {}", user.Config)
	}
	if user.LockedUntil != nil || user.LastLoginAt != nil || user.FailedLoginAttempts != 0 {
		t.Error("el usuario no debe nacer bloqueado ni con logins")
	}
	if user.CreatedDate.Location() != time.UTC {
		t.Errorf("createdDate = %v, se esperaba UTC", user.CreatedDate)
	}
}

func TestNewUserModel_OptionalData(t *testing.T) {
	t.Parallel()

	email := "jmata@example.com"
	name := "Joao"
	lastName := "Mata"
	idCreatedBy := primitive.NewObjectID()

	user, err := models.NewUserModel("jmata", genericPrivilege(), "hash", &models.UserOptionalData{
		Email:       &email,
		Name:        &name,
		LastName:    &lastName,
		IDCreatedBy: &idCreatedBy,
	})

	testutil.RequireEmptyError(t, err)
	if user.Email == nil || *user.Email != email {
		t.Errorf("email = %v, se esperaba %s", user.Email, email)
	}
	if user.Name == nil || *user.Name != name || user.LastName == nil || *user.LastName != lastName {
		t.Errorf("name = %v, lastName = %v", user.Name, user.LastName)
	}
	if user.IDCreatedBy == nil || *user.IDCreatedBy != idCreatedBy {
		t.Errorf("idCreatedBy = %v, se esperaba %s", user.IDCreatedBy, idCreatedBy.Hex())
	}
}

func TestSessionHasPrivilege(t *testing.T) {
	t.Parallel()

	superUser := cmm.UserPrivilegeSuperUser.Tag
	genericUser := cmm.UserPrivilegeGenericUser.Tag

	tests := []struct {
		name         string
		tagPrivilege string
		allowedTags  []string
		want         bool
	}{
		{"super usuario en ruta de super usuario", superUser, []string{superUser}, true},
		{"generico en ruta de super usuario", genericUser, []string{superUser}, false},
		{"generico en ruta de ambos", genericUser, []string{genericUser, superUser}, true},
		{"super usuario en ruta de ambos", superUser, []string{genericUser, superUser}, true},
		{"super usuario en ruta de genericos", superUser, []string{genericUser}, false},
		{"usuario sin privilegio", "", []string{genericUser, superUser}, false},
		{"tag en minusculas", "super_user", []string{superUser}, false},
		{"ruta sin tags permitidos", superUser, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			session := models.SessionsModelType{RecordOwnerData: models.RecordOwnerData{TagTypePrivilege: tt.tagPrivilege}}
			if got := models.SessionHasPrivilege(session, tt.allowedTags); got != tt.want {
				t.Errorf("SessionHasPrivilege = %v, se esperaba %v", got, tt.want)
			}
		})
	}
}

func TestUserIsLocked(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Minute)
	past := now.Add(-time.Minute)

	tests := []struct {
		name        string
		lockedUntil *time.Time
		want        bool
	}{
		{"sin bloqueo", nil, false},
		{"bloqueo vigente", &future, true},
		{"bloqueo vencido", &past, false},
		{"bloqueo que vence justo ahora", &now, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			user := models.UsersModelType{LockedUntil: tt.lockedUntil}
			if got := models.UserIsLocked(user, now); got != tt.want {
				t.Errorf("UserIsLocked = %v, se esperaba %v", got, tt.want)
			}
		})
	}
}

func TestNewSessionModel(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.FixedZone("UTC-5", -5*3600))
	user, err := models.NewUserModel("jmata", genericPrivilege(), "hash", nil)
	testutil.RequireEmptyError(t, err)

	session, err := models.NewSessionModel(user, "hash-del-token", now)

	testutil.RequireEmptyError(t, err)
	if session.ID.IsZero() || session.IDUser != user.ID || session.TokenHash != "hash-del-token" {
		t.Errorf("sesion = %+v, se esperaba el usuario %s con su hash", session, user.ID.Hex())
	}
	// La sesion guarda la copia del usuario y su privilegio para autenticar con una sola consulta
	if session.Username != user.Username || session.RecordOwnerData != models.NewRecordOwnerData(user) {
		t.Errorf("usuario de la sesion = %s %+v, se esperaba la copia de %s", session.Username, session.RecordOwnerData, user.Username)
	}
	if want := now.UTC().Add(cmm.SessionTTL); !session.ExpiresAt.Equal(want) || session.ExpiresAt.Location() != time.UTC {
		t.Errorf("expiresAt = %v, se esperaba %v en UTC", session.ExpiresAt, want)
	}
	if session.LogicalDelete {
		t.Error("la sesion no debe nacer borrada")
	}
}

// TestNewRecordOwnerData se copia el usuario con su privilegio al registro que crea
func TestNewRecordOwnerData(t *testing.T) {
	t.Parallel()

	user, err := models.NewUserModel("jmata", genericPrivilege(), "hash", nil)
	testutil.RequireEmptyError(t, err)

	owner := models.NewRecordOwnerData(user)

	if owner.IDUser != user.ID {
		t.Errorf("id_user = %s, se esperaba %s", owner.IDUser.Hex(), user.ID.Hex())
	}
	if owner.IDTypePrivilege != user.IDTypePrivilege || owner.TagTypePrivilege != user.TagPrivilege ||
		owner.NameTypePrivilege != user.NamePrivilege || owner.DescriptionTypePrivilege != user.DescriptionPrivilege {
		t.Errorf("privilegio = %+v, se esperaba la copia del privilegio de %+v", owner, user)
	}
}

func TestMissingUsers(t *testing.T) {
	t.Parallel()

	first, second, third := primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()

	tests := []struct {
		name     string
		expected []primitive.ObjectID
		found    []models.UsersModelType
		want     []string
	}{
		{"todos encontrados", []primitive.ObjectID{first, second}, []models.UsersModelType{{ID: second}, {ID: first}}, []string{}},
		{"sin usuarios", nil, nil, []string{}},
		{"faltan en el orden enviado", []primitive.ObjectID{third, first, second}, []models.UsersModelType{{ID: first}}, []string{third.Hex(), second.Hex()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := models.MissingUsers(tt.expected, tt.found)
			if got == nil || !slices.Equal(got, tt.want) {
				t.Errorf("MissingUsers = %v, se esperaba %v", got, tt.want)
			}
		})
	}
}
