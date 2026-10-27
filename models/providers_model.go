package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProvidersModelType struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Tag         string             `bson:"tag" json:"tag" description:"Tag o identificador"`
	Name        string             `bson:"name" json:"name" description:"Nombre"`
	Description string             `bson:"description" json:"description" description:"Nombre"`
	Configs     interface{}        `bson:"configs" json:"configs" description:"Objeto con configuraciones varias (URLS, HEADERS, ETC)"`
	Comment     string             `bson:"comment,omitempty" json:"comment,omitempty" description:"Comentario"`
	TagStatus   string             `bson:"tag_status" json:"tagStatus" description:"Tag estatus del catalogo"`
	Status      string             `bson:"status" json:"status" description:"Estatus del catalogo"`

	// Autenticacion de las peticiones HTTP de las condiciones de este proveedor
	SecretRef    string   `bson:"secret_ref,omitempty" json:"secretRef,omitempty" description:"Nombre del secreto, se lee de la variable de entorno JM_CTRD_SECRET_<secret_ref>"`
	AllowedHosts []string `bson:"allowed_hosts,omitempty" json:"allowedHosts,omitempty" description:"Hosts a los que se pueden enviar peticiones de este proveedor"`

	// Common
	LastUpdatedAt time.Time `bson:"last_updated_at" json:"lastUpdatedAt" description:"Ultima Actualizacion"`
	CreatedDate   time.Time `bson:"created_date" json:"createdDate" description:"Fecha de Creacion"`
	LogicalDelete bool      `bson:"logical_delete" json:"-" description:"Borrado Logico"`
	CoreDB        string    `bson:"core_db" json:"-" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string    `bson:"core_version" json:"-" description:"Version del Core"`
	UserDB        string    `bson:"user_db" json:"-" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string    `bson:"schema_db" json:"-" description:"Versionado de la Base de datos"`
}

// Model pointer
var ProvidersModel *mongo.Collection
