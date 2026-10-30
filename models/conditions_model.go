package models

import (
	cmm "c_trd/common"
	"c_trd/providers"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ConditionsModelType struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Tag          string             `bson:"tag" json:"tag"`
	Name         string             `bson:"name" json:"name"`
	Description  string             `bson:"description" json:"description"`
	IDProvider   primitive.ObjectID `bson:"id_provider" json:"idProvider"`
	NameProvider string             `bson:"name_provider" json:"nameProvider"`

	// Type condition
	IDTypeCondition   primitive.ObjectID `bson:"id_type_condition" json:"idTypeCondition"`
	NameTypeCondition string             `bson:"name_type_condition" json:"nameTypeCondition"`
	TagTypeCondition  string             `bson:"tag_type_condition" json:"tagTypeCondition"`

	// Metadata y http_request no se envian al frontend (json:"-")
	// Si Metadata es nil en Go, se guardará como null en la base de datos, siempre debe ser un objeto asi sea vacio
	Metadata interface{} `bson:"metadata" json:"-" description:"Objeto con datos a guardar para data analisis"`

	// --- CAMPOS DE EJECUCIÓN Y WEBHOOKS ---
	IsWebhookOnly bool `bson:"is_webhook_only" json:"isWebhookOnly" description:"Indica si el status solo cambia por webhook"`

	// Solo si se valida por cronjob
	NextExecutionAt *time.Time `bson:"next_execution_at" json:"nextExecutionAt" description:"Momento en el que el cronjob debe evaluar esta condición"`
	IntervalSeconds *int       `bson:"interval_seconds" json:"intervalSeconds" description:"Frecuencia de actualización en segundos"`
	LastExecutedAt  *time.Time `bson:"last_executed_at" json:"lastExecutedAt" description:"Última vez que se evaluó la condición"`

	IsProcessing bool `bson:"is_processing" json:"isProcessing" description:"Lock optimista para evitar que dos workers procesen el mismo registro"`
	// Data que se pasa a la API (parametros dinamicos. Ej: symbol, pair, currency)
	IDApi            primitive.ObjectID          `bson:"id_api" json:"idApi" description:"Id de la api"`
	HttpRequest      providers.HTTPRequestConfig `bson:"http_request" json:"-" description:"Peticion."`
	CallbackFunction string                      `bson:"callback_function" json:"callbackFunction" description:"Nombre de la función que procesará la respuesta"`
	// Valores de los parametros del callback ya tipados (ej: umbrales). Se guarda {} si no tiene
	ParametersCallback map[string]any `bson:"parameters_callback" json:"parametersCallback" description:"Parametros que recibe el callback"`

	// Status
	IDStatus  primitive.ObjectID `bson:"id_status" json:"idStatus"`
	TagStatus string             `bson:"tag_status" json:"tagStatus" description:"Tag estatus de la condicion"`
	Status    string             `bson:"status" json:"status"`

	// Usuario que la creo y quienes pueden verla
	RecordOwnerData      `bson:",inline"`
	RecordVisibilityData `bson:",inline"`

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
var ConditionsModel *mongo.Collection

// CONSTRUCTORS

// ConditionCronjobData datos con los que el cronjob evalua la condicion
type ConditionCronjobData struct {
	HttpRequest        providers.HTTPRequestConfig
	CallbackFunction   string
	ParametersCallback map[string]any
	IntervalSeconds    int
}

// ConditionTypeData catalogo TYPE_CONDITION que clasifica la condicion
type ConditionTypeData struct {
	ID   primitive.ObjectID
	Tag  string
	Name string
}

// NewConditionModel crea una condicion de cronjob (is_webhook_only false) con status inactivo hasta que se cumpla.
// Se programa para evaluarse en la siguiente ejecucion del cronjob. Metadata y parameters_callback se guardan como objeto vacio
func NewConditionModel(
	// --- Condicion ---
	tag, name, description string,
	// --- Proveedor ---
	idProvider primitive.ObjectID, nameProvider string,
	// --- Tipo de condicion ---
	typeCondition ConditionTypeData,
	idApi primitive.ObjectID,
	// --- Cronjob ---
	cronjob ConditionCronjobData,
	// --- Usuario y visibilidad ---
	owner RecordOwnerData, visibility RecordVisibilityData,
) (ConditionsModelType, *cmm.ErrorHandler) {
	now := time.Now().UTC()

	condition := ConditionsModelType{
		ID:                   primitive.NewObjectID(),
		Tag:                  tag,
		Name:                 name,
		Description:          description,
		IDProvider:           idProvider,
		NameProvider:         nameProvider,
		IDTypeCondition:      typeCondition.ID,
		NameTypeCondition:    typeCondition.Name,
		TagTypeCondition:     typeCondition.Tag,
		Metadata:             map[string]any{},
		ParametersCallback:   map[string]any{},
		IsWebhookOnly:        false,
		IsProcessing:         false,
		IDApi:                idApi,
		IDStatus:             cmm.GenericStatusInactive.ID,
		TagStatus:            cmm.GenericStatusInactive.Tag,
		Status:               cmm.GenericStatusInactive.Name,
		RecordOwnerData:      owner,
		RecordVisibilityData: recordVisibilityWithAllowedUsers(visibility),

		// Auditoría
		CoreDB:      os.Getenv("JM_CTRD_MDB_NAME"),
		CoreVersion: "1.0.0", // TODO: poner version
		UserDB:      os.Getenv("JM_CTRD_MDB_USER"),
		SchemaDB:    "1.0.0", // TODO: mejorar estructura

		CreatedDate:   now,
		LastUpdatedAt: now,
		LogicalDelete: false,
	}

	if cronjob.IntervalSeconds < cmm.ConditionMinIntervalSeconds {
		return condition, cmm.NewErrorHandler(fmt.Errorf("interval_seconds %d", cronjob.IntervalSeconds), fmt.Sprintf("El intervalo minimo es de %d segundos", cmm.ConditionMinIntervalSeconds), cmm.LevelError, "MCONE001")
	}

	interval := cronjob.IntervalSeconds
	condition.HttpRequest = cronjob.HttpRequest
	condition.CallbackFunction = cronjob.CallbackFunction
	if cronjob.ParametersCallback != nil {
		condition.ParametersCallback = cronjob.ParametersCallback
	}
	condition.IntervalSeconds = &interval
	condition.NextExecutionAt = &now

	return condition, cmm.NewEmptyErrorHandler()
}
