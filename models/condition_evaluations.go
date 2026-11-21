package models

import (
	"os"
	"time"

	cmm "c_trd/common"
	"c_trd/providers"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// ConditionEvaluationsModelType evaluacion de una condicion por el cronjob. Se guarda una por cada ejecucion, tambien
// las fallidas, para poder reconstruir el historico (backtesting) con el valor medido y la fecha propia del dato
type ConditionEvaluationsModelType struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	IDCondition primitive.ObjectID `bson:"id_condition" json:"idCondition"`
	// Copia del callback y sus parametros al momento de la evaluacion, la condicion puede cambiar despues
	CallbackFunction   string         `bson:"callback_function" json:"callbackFunction"`
	ParametersCallback map[string]any `bson:"parameters_callback" json:"parametersCallback"`
	// Resultado
	Evaluated         bool       `bson:"evaluated" json:"evaluated" description:"true si la API y el callback respondieron correctamente"`
	Accepted          bool       `bson:"accepted" json:"accepted" description:"true si la condicion se cumplio, false si no se cumplio o no se evaluo"`
	Value             *float64   `bson:"value" json:"value" description:"Valor medido por el callback, null si no se evaluo"`
	AsOf              *time.Time `bson:"as_of" json:"asOf" description:"Fecha del dato: la del proveedor o, si no la trae, executed_at. null si no se evaluo"`
	AsOfSource        *string    `bson:"as_of_source" json:"asOfSource" description:"PROVIDER o SERVER segun de donde salio as_of, null si no se evaluo"`
	ExecutedAt        time.Time  `bson:"executed_at" json:"executedAt" description:"Fecha en la que el cronjob ejecuto la evaluacion"`
	ErrorTrackingCode *string    `bson:"error_tracking_code" json:"errorTrackingCode" description:"Tracking code del error, null si se evaluo"`

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
var ConditionEvaluationsModel *mongo.Collection

// CONSTRUCTORS

// ConditionEvaluationData resultado de evaluar una condicion. Result solo se usa si Evaluated es true y
// ErrorTrackingCode solo si es false
type ConditionEvaluationData struct {
	Evaluated         bool
	Result            providers.CallbackResult
	ExecutedAt        time.Time
	ErrorTrackingCode string
}

// NewConditionEvaluationModel crea la evaluacion de la condicion. Si se evaluo, as_of es la fecha del proveedor
// (as_of_source PROVIDER) o, si la respuesta no la traia, executed_at (as_of_source SERVER) y el tracking code queda null.
// Si no se evaluo value, as_of, as_of_source y accepted quedan vacios (null y false) y se guarda el tracking code del error
func NewConditionEvaluationModel(condition ConditionsModelType, evaluation ConditionEvaluationData) (ConditionEvaluationsModelType, *cmm.ErrorHandler) {
	now := time.Now().UTC()

	parameters := condition.ParametersCallback
	if parameters == nil {
		parameters = map[string]any{}
	}

	var accepted bool
	var value *float64
	var asOf *time.Time
	var asOfSource *string
	var errorTrackingCode *string
	if evaluation.Evaluated {
		measured := evaluation.Result.Value
		accepted = evaluation.Result.Accepted
		value = &measured

		dataDate := evaluation.ExecutedAt
		source := cmm.AsOfSourceServer
		if evaluation.Result.AsOf != nil {
			dataDate = *evaluation.Result.AsOf
			source = cmm.AsOfSourceProvider
		}
		asOf = &dataDate
		asOfSource = &source
	} else {
		trackingCode := evaluation.ErrorTrackingCode
		errorTrackingCode = &trackingCode
	}

	return ConditionEvaluationsModelType{
		ID:                 primitive.NewObjectID(),
		IDCondition:        condition.ID,
		CallbackFunction:   condition.CallbackFunction,
		ParametersCallback: parameters,
		Evaluated:          evaluation.Evaluated,
		Accepted:           accepted,
		Value:              value,
		AsOf:               asOf,
		AsOfSource:         asOfSource,
		ExecutedAt:         evaluation.ExecutedAt,
		ErrorTrackingCode:  errorTrackingCode,

		// Auditoría
		CoreDB:      os.Getenv("JM_CTRD_MDB_NAME"),
		CoreVersion: "1.0.0", // TODO: poner version
		UserDB:      os.Getenv("JM_CTRD_MDB_USER"),
		SchemaDB:    "1.0.0", // TODO: mejorar estructura

		CreatedDate:   now,
		LastUpdatedAt: now,
		LogicalDelete: false,
	}, cmm.NewEmptyErrorHandler()
}
