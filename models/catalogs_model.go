package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CatalogsModelType struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	TypeCatalog   string             `bson:"type_catalog" json:"type_catalog" description:"tipo de catalogo"`
	Tag           string             `bson:"tag" json:"tag" description:"Tag o identificador"`
	Name          string             `bson:"name" json:"name" description:"Nombre"`
	Description   string             `bson:"description" json:"description" description:"Nombre"`
	Configs       interface{}        `bson:"configs" json:"configs" description:"Objeto con configuraciones varias"`
	Comment       string             `bson:"comment,omitempty" json:"comment,omitempty" description:"Comentario"`

	// Common
	LastUpdatedAt time.Time          `bson:"last_updated_at" json:"last_updated_at" description:"Ultima Actualizacion"`
	CreatedDate   time.Time          `bson:"created_date" json:"created_date" description:"Fecha de Creacion"`
	LogicalDelete bool               `bson:"logical_delete" json:"logical_delete" description:"Borrado Logico"`
	CoreDB        string             `bson:"core_db" json:"core_db" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string             `bson:"core_version" json:"core_version" description:"Version del Core"`
	UserDB        string             `bson:"user_db" json:"user_db" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string             `bson:"schema_db" json:"schema_db" description:"Versionado de la Base de datos"`
}

// Model pointer
var CatalogsModel  *mongo.Collection