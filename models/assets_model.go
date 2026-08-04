package models

import (
	"c_trd/common"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AssetsModelType struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Tag           string             `bson:"tag" json:"tag" description:"Tag o identificador"`
	Name          string             `bson:"name" json:"name" description:"Nombre"`
	Description   string             `bson:"description" json:"description" description:"Nombre"`
	Configs       interface{}        `bson:"configs" json:"configs" description:"Objeto con configuraciones varias"`
	Comment       string             `bson:"comment,omitempty" json:"comment,omitempty" description:"Comentario"`
	TagStatus     string             `bson:"tag_status" json:"tag_status" description:"Tag estatus del catalogo"`
	Status        string             `bson:"status" json:"status" description:"Estatus del catalogo"`
	
	// Common
	LastUpdatedAt time.Time          `bson:"last_updated_at" json:"last_updated_at" description:"Ultima Actualizacion"`
	CreatedDate   time.Time          `bson:"created_date" json:"created_date" description:"Fecha de Creacion"`
	LogicalDelete bool               `bson:"logical_delete" json:"logical_delete" description:"Borrado Logico"`
	CoreDB        string             `bson:"core_db" json:"core_db" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string             `bson:"core_version" json:"core_version" description:"Version del Core"`
	UserDB        string             `bson:"user_db" json:"user_db" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string             `bson:"schema_db" json:"schema_db" description:"Versionado de la Base de datos"`
}

// Export model to be used directly
var AssetsModel = common.GetCollection("assets")