package models

import (
	"os"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AgentThesesModelType idea de inversion que el super usuario deja en la cola de los agentes. Cada jueves en la noche la corrida
// toma las AgentThesesPerRun mas antiguas pendientes (FIFO) y el resto espera a la siguiente semana
type AgentThesesModelType struct {
	ID     primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Thesis string             `bson:"thesis" json:"thesis" description:"Idea de inversion, horizonte y contexto en texto libre"`
	// Asset
	IDAsset  primitive.ObjectID `bson:"id_asset" json:"idAsset"`
	TagAsset string             `bson:"tag_asset" json:"tagAsset"`
	Asset    string             `bson:"asset" json:"asset"`
	// Action
	IDAction  primitive.ObjectID `bson:"id_action" json:"idAction"`
	TagAction string             `bson:"tag_action" json:"tagAction"`
	Action    string             `bson:"action" json:"action"`
	// Status (catalogo AGENT_THESIS_STATUS)
	IDStatus  primitive.ObjectID `bson:"id_status" json:"idStatus"`
	TagStatus string             `bson:"tag_status" json:"tagStatus"`
	Status    string             `bson:"status" json:"status"`
	// Momento en que una corrida la tomo, null si nunca se tomo. Sirve para recuperar tesis de corridas que fallaron
	ClaimedAt *time.Time `bson:"claimed_at" json:"claimedAt" description:"Momento en que una corrida la tomo"`
	// Usuario que la creo (super usuario)
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
var AgentThesesModel *mongo.Collection

// CONSTRUCTORS

// NewAgentThesisModel crea una tesis pendiente en la cola de los agentes
func NewAgentThesisModel(
	thesis string,
	// --- Asset ---
	idAsset primitive.ObjectID, tagAsset, asset string,
	// --- Action ---
	idAction primitive.ObjectID, tagAction, action string,
	// --- Usuario ---
	owner RecordOwnerData,
) (AgentThesesModelType, *cmm.ErrorHandler) {
	now := time.Now().UTC()

	return AgentThesesModelType{
		ID:              primitive.NewObjectID(),
		Thesis:          thesis,
		IDAsset:         idAsset,
		TagAsset:        tagAsset,
		Asset:           asset,
		IDAction:        idAction,
		TagAction:       tagAction,
		Action:          action,
		IDStatus:        cmm.AgentThesisStatusPending.ID,
		TagStatus:       cmm.AgentThesisStatusPending.Tag,
		Status:          cmm.AgentThesisStatusPending.Name,
		ClaimedAt:       nil,
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
