package models

import (
	"fmt"
	"slices"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CatalogsModelType struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	TypeCatalog string             `bson:"type_catalog" json:"typeCatalog" description:"tipo de catalogo"`
	Tag         string             `bson:"tag" json:"tag" description:"Tag o identificador"`
	Name        string             `bson:"name" json:"name" description:"Nombre"`
	Description string             `bson:"description" json:"description" description:"Nombre"`
	Configs     interface{}        `bson:"configs" json:"configs" description:"Objeto con configuraciones varias"`
	Comment     string             `bson:"comment,omitempty" json:"comment,omitempty" description:"Comentario"`

	// Common
	LastUpdatedAt time.Time `bson:"last_updated_at" json:"lastUpdatedAt" description:"Ultima Actualizacion"`
	CreatedDate   time.Time `bson:"created_date" json:"createdDate" description:"Fecha de Creacion"`
	LogicalDelete bool      `bson:"logical_delete" json:"-" description:"Borrado Logico"`
	CoreDB        string    `bson:"core_db" json:"-" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string    `bson:"core_version" json:"-" description:"Version del Core"`
	UserDB        string    `bson:"user_db" json:"-" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string    `bson:"schema_db" json:"-" description:"Versionado de la Base de datos"`
}

// RecordVisibilityData visibilidad (catalogo TYPE_VISIBILITY) de un trigger, condicion u oportunidad. Se embebe en
// los modelos (bson inline). allowed_users solo tiene usuarios si la visibilidad es lista blanca, si no se guarda []
type RecordVisibilityData struct {
	IDTypeVisibility          primitive.ObjectID   `bson:"id_type_visibility" json:"idTypeVisibility" description:"Id de la visibilidad del registro"`
	TagTypeVisibility         string               `bson:"tag_type_visibility" json:"tagTypeVisibility"`
	NameTypeVisibility        string               `bson:"name_type_visibility" json:"nameTypeVisibility"`
	DescriptionTypeVisibility string               `bson:"description_type_visibility" json:"descriptionTypeVisibility"`
	AllowedUsers              []primitive.ObjectID `bson:"allowed_users" json:"allowedUsers" description:"Usuarios que pueden ver el registro si es lista blanca"`
}

// Model pointer
var CatalogsModel *mongo.Collection

// Tags de TYPE_VISIBILITY que se saben filtrar
var knownVisibilityTags = []string{cmm.VisibilityTagPrivate, cmm.VisibilityTagPublic, cmm.VisibilityTagWhitelist}

// NewRecordVisibilityData copia la visibilidad del catalogo al registro. La lista blanca necesita al menos un usuario
// y las demas visibilidades no pueden tener usuarios. allowedUsers nil se guarda como []
func NewRecordVisibilityData(visibility CatalogsModelType, allowedUsers []primitive.ObjectID) (RecordVisibilityData, *cmm.ErrorHandler) {
	result := RecordVisibilityData{
		IDTypeVisibility:          visibility.ID,
		TagTypeVisibility:         visibility.Tag,
		NameTypeVisibility:        visibility.Name,
		DescriptionTypeVisibility: visibility.Description,
		AllowedUsers:              []primitive.ObjectID{},
	}

	if !slices.Contains(knownVisibilityTags, visibility.Tag) {
		return result, cmm.NewErrorHandler(fmt.Errorf("tag_type_visibility %s", visibility.Tag), "La visibilidad enviada no esta soportada", cmm.LevelError, "MCATE001")
	}

	if visibility.Tag == cmm.VisibilityTagWhitelist && len(allowedUsers) == 0 {
		return result, cmm.NewErrorHandler(fmt.Errorf("allowed_users vacio"), "La visibilidad de lista blanca necesita al menos un usuario en allowedUsers", cmm.LevelError, "MCATE002")
	}

	if visibility.Tag != cmm.VisibilityTagWhitelist && len(allowedUsers) > 0 {
		return result, cmm.NewErrorHandler(fmt.Errorf("allowed_users con %d usuarios", len(allowedUsers)), "Solo la visibilidad de lista blanca puede tener allowedUsers", cmm.LevelError, "MCATE003")
	}

	if allowedUsers != nil {
		result.AllowedUsers = allowedUsers
	}

	return result, cmm.NewEmptyErrorHandler()
}

// VisibleToUserFilter filtro de MongoDB de los registros que puede ver el usuario: los publicos, los suyos y los de
// lista blanca donde esta en allowed_users. Con idUser cero (usuario sin registro) solo los publicos
func VisibleToUserFilter(idUser primitive.ObjectID) bson.M {
	if idUser.IsZero() {
		return bson.M{"tag_type_visibility": cmm.VisibilityTagPublic}
	}

	return bson.M{"$or": []bson.M{
		{"tag_type_visibility": cmm.VisibilityTagPublic},
		{"id_user": idUser},
		{"tag_type_visibility": cmm.VisibilityTagWhitelist, "allowed_users": idUser},
	}}
}

// recordVisibilityWithAllowedUsers asegura que allowed_users se guarde como [] y no como null
func recordVisibilityWithAllowedUsers(visibility RecordVisibilityData) RecordVisibilityData {
	if visibility.AllowedUsers == nil {
		visibility.AllowedUsers = []primitive.ObjectID{}
	}

	return visibility
}
