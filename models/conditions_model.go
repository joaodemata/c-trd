package models

import (
	"c_trd/common"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ConditionsModelType struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	
	IDProvider           primitive.ObjectID `bson:"id_provider" json:"idProvider"`
	NameProvider         string             `bson:"name_provider" json:"nameProvider"`
	NameCondition        string             `bson:"name_condition" json:"nameCondition"`
	DescriptionCondition string             `bson:"description_condition" json:"descriptionCondition"`
	Metadata             interface{}       `bson:"metadata" json:"metadata" description:"Objeto con datos a guardar para data analisis"`
	
	// Common
	LastUpdatedAt time.Time          `bson:"last_updated_at" json:"last_updated_at" description:"Ultima Actualizacion"`
	CreatedDate   time.Time          `bson:"created_date" json:"created_date" description:"Fecha de Creacion"`
	LogicalDelete bool               `bson:"logical_delete" json:"logical_delete" description:"Borrado Logico"`
	CoreDB        string             `bson:"core_db" json:"core_db" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string             `bson:"core_version" json:"core_version" description:"Version del Core"`
	UserDB        string             `bson:"user_db" json:"user_db" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string             `bson:"schema_db" json:"schema_db" description:"Versionado de la Base de datos"`
}

// Expoprt model to be used directly
var ConditionsModel = common.GetCollection("conditions")

