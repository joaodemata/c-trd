package models_test

import (
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

func TestUserIsSuperUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		idPrivilege primitive.ObjectID
		want        bool
	}{
		{"super usuario", cmm.UserPrivilegeSuperUser.ID, true},
		{"usuario generico", cmm.UserPrivilegeGenericUser.ID, false},
		{"sin privilegio", primitive.NilObjectID, false},
		{"privilegio desconocido", primitive.NewObjectID(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			user := models.UsersModelType{IDTypePrivilege: tt.idPrivilege}
			if got := models.UserIsSuperUser(user); got != tt.want {
				t.Errorf("UserIsSuperUser = %v, se esperaba %v", got, tt.want)
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
	idUser := primitive.NewObjectID()

	session, err := models.NewSessionModel(idUser, "hash-del-token", now)

	testutil.RequireEmptyError(t, err)
	if session.ID.IsZero() || session.IDUser != idUser || session.TokenHash != "hash-del-token" {
		t.Errorf("sesion = %+v, se esperaba el usuario %s con su hash", session, idUser.Hex())
	}
	if want := now.UTC().Add(cmm.SessionTTL); !session.ExpiresAt.Equal(want) || session.ExpiresAt.Location() != time.UTC {
		t.Errorf("expiresAt = %v, se esperaba %v en UTC", session.ExpiresAt, want)
	}
	if session.LogicalDelete {
		t.Error("la sesion no debe nacer borrada")
	}
}
