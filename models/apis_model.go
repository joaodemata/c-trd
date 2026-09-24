package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ApisModelType struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	IDProvider   primitive.ObjectID `bson:"id_provider,omitempty" json:"idProvider,omitempty"`
	TagProvider  string             `bson:"tag" json:"tagProvider" description:"Tag del proveedor de la API."`
	NameProvider string             `bson:"tag" json:"nameProvider" description:"Nombre del proveedor de la API."`

	Tag          string      `bson:"tag" json:"tag" description:"Tag de la API."`
	Name         string      `bson:"name" json:"name" description:"Nombre de la API."`
	Description  string      `bson:"description" json:"description" description:"Descripcion de la API."`
	IsActive     bool        `bson:"is_active" json:"isActive" description:"Si esta activo la API."`
	Type         string      `bson:"type" json:"type" description:"Tipo de api (interna o externa)"`
	Method       string      `bson:"method" json:"method" description:"metodo de la peticion a Llamar"`
	URL          string      `bson:"url" json:"url" description:"url a Llamar"`
	Headers      interface{} `bson:"headers" json:"headers" description:"cabeceras de la peticion"`
	DataLocation string      `bson:"data_location" json:"dataLocation" description:"BODY, QUERY, ETC"`
	Timeout      string      `bson:"timeout" json:"timeout" description:"Timeout de la peticion"`

	// Comunes
	LastUpdatedAt time.Time `bson:"last_updated_at" json:"lastUpdatedAt" description:"Ultima Actualizacion"`
	CreatedDate   time.Time `bson:"created_date" json:"createdDate" description:"Fecha de Creacion"`
	LogicalDelete bool      `bson:"logical_delete" json:"logicalDelete" description:"Borrado Logico"`
	CoreDB        string    `bson:"core_db" json:"coreDb" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string    `bson:"core_version" json:"coreVersion" description:"Version del Core"`
	UserDB        string    `bson:"user_db" json:"userDb" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string    `bson:"schema_db" json:"schemaDb" description:"Versionado de la Base de datos."`
}

// Model pointer
var ApisModel *mongo.Collection
