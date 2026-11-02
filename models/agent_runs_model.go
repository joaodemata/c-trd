package models

import (
	"os"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AgentRunStep un intento de un agente: su salida (JSON) y los problemas que encontraron las validaciones o el backend
type AgentRunStep struct {
	Agent     string         `bson:"agent" json:"agent" description:"CONDITION_DESIGNER, TRIGGER_COMPOSER, PINE_AUTHOR o FINANCIAL_JUDGE"`
	Attempt   int            `bson:"attempt" json:"attempt" description:"Numero de intento del agente, desde 1"`
	Output    map[string]any `bson:"output" json:"output" description:"Salida del agente, {} si no fue un JSON valido"`
	Problems  []string       `bson:"problems" json:"problems" description:"Problemas de la salida, [] si fue aceptada"`
	CreatedAt time.Time      `bson:"created_at" json:"createdAt"`
}

// AgentRunsModelType corrida de los agentes sobre una tesis. La crea el usuario agente al tomar la tesis y la cierra
// al terminar; el super usuario la revisa junto con las condiciones y el trigger que quedaron pendientes
type AgentRunsModelType struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	IDThesis primitive.ObjectID `bson:"id_thesis" json:"idThesis"`
	// Copia de la tesis al momento de la corrida
	Thesis    string             `bson:"thesis" json:"thesis"`
	IDAsset   primitive.ObjectID `bson:"id_asset" json:"idAsset"`
	TagAsset  string             `bson:"tag_asset" json:"tagAsset"`
	Asset     string             `bson:"asset" json:"asset"`
	IDAction  primitive.ObjectID `bson:"id_action" json:"idAction"`
	TagAction string             `bson:"tag_action" json:"tagAction"`
	Action    string             `bson:"action" json:"action"`
	// Status (catalogo AGENT_RUN_STATUS)
	IDStatus  primitive.ObjectID `bson:"id_status" json:"idStatus"`
	TagStatus string             `bson:"tag_status" json:"tagStatus"`
	Status    string             `bson:"status" json:"status"`
	// Resultado
	Steps        []AgentRunStep       `bson:"steps" json:"steps" description:"Intentos de cada agente, [] hasta que termine"`
	IDConditions []primitive.ObjectID `bson:"id_conditions" json:"idConditions" description:"Condiciones creadas en la corrida"`
	IDTrigger    *primitive.ObjectID  `bson:"id_trigger" json:"idTrigger" description:"Trigger creado, null si no se creo"`
	Error        *string              `bson:"error" json:"error" description:"Razon por la que fallo, null si no fallo"`
	StartedAt    time.Time            `bson:"started_at" json:"startedAt"`
	FinishedAt   *time.Time           `bson:"finished_at" json:"finishedAt" description:"null mientras esta en ejecucion"`
	// Usuario agente que la ejecuto
	RecordOwnerData `bson:",inline"`

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
var AgentRunsModel *mongo.Collection

// CONSTRUCTORS

// NewAgentRunModel crea la corrida en ejecucion de una tesis tomada por el usuario agente
func NewAgentRunModel(thesis AgentThesesModelType, owner RecordOwnerData) (AgentRunsModelType, *cmm.ErrorHandler) {
	now := time.Now().UTC()

	return AgentRunsModelType{
		ID:              primitive.NewObjectID(),
		IDThesis:        thesis.ID,
		Thesis:          thesis.Thesis,
		IDAsset:         thesis.IDAsset,
		TagAsset:        thesis.TagAsset,
		Asset:           thesis.Asset,
		IDAction:        thesis.IDAction,
		TagAction:       thesis.TagAction,
		Action:          thesis.Action,
		IDStatus:        cmm.AgentRunStatusRunning.ID,
		TagStatus:       cmm.AgentRunStatusRunning.Tag,
		Status:          cmm.AgentRunStatusRunning.Name,
		Steps:           []AgentRunStep{},
		IDConditions:    []primitive.ObjectID{},
		IDTrigger:       nil,
		Error:           nil,
		StartedAt:       now,
		FinishedAt:      nil,
		RecordOwnerData: owner,

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
