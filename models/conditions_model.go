package models

import (
	"c_trd/providers"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ConditionsModelType struct {
	ID                   primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	IDProvider           primitive.ObjectID `bson:"id_provider" json:"idProvider"`
	NameProvider         string             `bson:"name_provider" json:"nameProvider"`
	NameCondition        string             `bson:"name_condition" json:"nameCondition"`
	DescriptionCondition string             `bson:"description_condition" json:"descriptionCondition"`

	// Si Metadata es nil en Go, se guardará como null en la base de datos, siempre debe ser un objeto asi sea vacio
	Metadata interface{} `bson:"metadata" json:"metadata" description:"Objeto con datos a guardar para data analisis"`

	// --- CAMPOS DE EJECUCIÓN Y WEBHOOKS ---
	IsWebhookOnly bool `bson:"is_webhook_only" json:"is_webhook_only" description:"Indica si el status solo cambia por webhook"`

	// Solo si se valida por cronjob
	NextExecutionAt *time.Time `bson:"next_execution_at" json:"next_execution_at" description:"Momento en el que el cronjob debe evaluar esta condición"`
	IntervalSeconds *int       `bson:"interval_seconds" json:"interval_seconds" description:"Frecuencia de actualización en segundos"`
	LastExecutedAt  *time.Time `bson:"last_executed_at" json:"last_executed_at" description:"Última vez que se evaluó la condición"`

	IsProcessing bool `bson:"is_processing" json:"is_processing" description:"Lock optimista para evitar que dos workers procesen el mismo registro"`
	// Data que se pasa a la API (parametros dinamicos. Ej: symbol, pair, currency)
	IDApi            primitive.ObjectID          `bson:"id_api" json:"idApi" description:"Id de la api"`
	HttpRequest      providers.HTTPRequestConfig `bson:"http_request" json:"httpRequest" description:"Peticion."`
	CallbackFunction string                      `bson:"callback_function" json:"callbackFunction" description:"Nombre de la función que procesará la respuesta"`

	// Status
	IDStatus  primitive.ObjectID `bson:"id_status" json:"id_status"`
	TagStatus string             `bson:"tag_status" json:"tag_status" description:"Tag estatus de la condicion"`
	Status    string             `bson:"status" json:"status"`

	// Common
	LastUpdatedAt time.Time `bson:"last_updated_at" json:"last_updated_at" description:"Ultima Actualizacion"`
	CreatedDate   time.Time `bson:"created_date" json:"created_date" description:"Fecha de Creacion"`
	LogicalDelete bool      `bson:"logical_delete" json:"logical_delete" description:"Borrado Logico"`
	CoreDB        string    `bson:"core_db" json:"core_db" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string    `bson:"core_version" json:"core_version" description:"Version del Core"`
	UserDB        string    `bson:"user_db" json:"user_db" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string    `bson:"schema_db" json:"schema_db" description:"Versionado de la Base de datos"`
}

// Model pointer
var ConditionsModel *mongo.Collection
