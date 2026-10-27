package models

import (
	"os"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Condiciones
type conditionTriggers struct {
	IDCondition          primitive.ObjectID `bson:"id_condition" json:"idCondition" description:"Id de la condicion"`
	IDProvider           primitive.ObjectID `bson:"id_provider" json:"idProvider" description:"Id del proveedor de informacion del servicio"`
	NameProvider         string             `bson:"name_provider" json:"nameProvider" description:" Proveedor de informacion del servicio"`
	NameCondition        string             `bson:"name_condition" json:"nameCondition" `
	DescriptionCondition string             `bson:"description_condition" json:"descriptionCondition"`
}

type TriggersModelType struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	IDWebhook   primitive.ObjectID `bson:"id_webhook" json:"idWebhook" description:"Id que recibe el hook, se mantiene entre las versiones del trigger"`
	Tag         string             `bson:"tag" json:"tag" description:"tag del trigger"`
	Name        string             `bson:"name" json:"name" description:"Nombre del trigger"`
	Description string             `bson:"description" json:"description" description:"Que principio utiliza este trigger"`
	// Asset
	IDAsset  primitive.ObjectID `bson:"id_asset" json:"idAsset"`
	TagAsset string             `bson:"tag_asset" json:"tagAsset"`
	Asset    string             `bson:"asset" json:"asset"`
	// Action
	IDAction  primitive.ObjectID `bson:"id_action" json:"idAction"`
	TagAction string             `bson:"tag_action" json:"tagAction"`
	Action    string             `bson:"action" json:"action"`
	// Status
	IDStatus  primitive.ObjectID `bson:"id_status" json:"idStatus"`
	TagStatus string             `bson:"tag_status" json:"tagStatus"`
	Status    string             `bson:"status" json:"status"`
	// Conditions
	Conditions []conditionTriggers `bson:"conditions" json:"conditions"`
	// Candles
	MaxCandlestickQty       int                `bson:"max_candlestick_qty" json:"maxCandlestickQty" description:"Maxima Cantidad de velas que puede haber para la oportunidad pasar a descartada"`
	IDCandlestickTimeframe  primitive.ObjectID `bson:"id_candlestick_timeframe" json:"idCandlestickTimeframe" description:"Tiempo que representa cada vela"`
	TagCandlestickTimeframe string             `bson:"tag_candlestick_timeframe" json:"tagCandlestickTimeframe"`
	CandlestickTimeframe    string             `bson:"candlestick_timeframe" json:"candlestickTimeframe"`
	// Extra
	// Opcionales: se guardan como null si no vienen
	IdReference     *primitive.ObjectID `bson:"id_reference" json:"idReference" description:"Trigger generado a partir de otro trigger"`
	IdReferenceRoot *primitive.ObjectID `bson:"id_reference_root" json:"idReferenceRoot" description:"Raiz inicial del trigger generado a partir de otro trigger"`
	PineCode        *string             `bson:"pine_code" json:"pineCode" description:"Codigo de pine utilizado en trading view para el trigger"`

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
var TriggersModel *mongo.Collection

// Helpers

// TriggerReferenceRoot retorna la raiz de la cadena de versiones para el trigger que reemplaza a previous:
// si previous ya viene de otro trigger se reutiliza su raiz, si no previous es la raiz
func TriggerReferenceRoot(previous TriggersModelType) primitive.ObjectID {
	if previous.IdReferenceRoot != nil {
		return *previous.IdReferenceRoot
	}

	return previous.ID
}

// TriggerWebhookID retorna el id del webhook para el trigger que reemplaza a previous: se reutiliza el de previous
// para que las alertas sigan apuntando al mismo id. Si previous no tiene (triggers anteriores al campo) se usa su ID,
// que es el id que ya envian sus alertas
func TriggerWebhookID(previous TriggersModelType) primitive.ObjectID {
	if !previous.IDWebhook.IsZero() {
		return previous.IDWebhook
	}

	return previous.ID
}

// CONSTRUCTORS

type TriggerOptionalData struct {
	Conditions      []ConditionsModelType // Se copian los datos de cada condicion al trigger
	IdWebhook       *primitive.ObjectID   // Si no viene se genera uno nuevo
	IdReference     *primitive.ObjectID
	IdReferenceRoot *primitive.ObjectID
	PineCode        *string
}

// NewTriggerModel crea un trigger nuevo con status activo. El id del webhook se genera si no viene en opts. Los arreglos opcionales se guardan como []
// y los demas campos opcionales como null para mantener la integridad de la data
func NewTriggerModel(
	// --- Trigger ---
	tag, name, description string,
	// --- Asset ---
	idAsset primitive.ObjectID, tagAsset, asset string,
	// --- Action ---
	idAction primitive.ObjectID, tagAction, action string,
	// --- Candles ---
	maxCandles int, idTimeframe primitive.ObjectID, tagTimeframe, timeframe string,
	// --- Opcionales ---
	opts *TriggerOptionalData,
) (TriggersModelType, *cmm.ErrorHandler) {
	now := time.Now().UTC()

	trigger := TriggersModelType{
		ID:                      primitive.NewObjectID(),
		IDWebhook:               primitive.NewObjectID(),
		Tag:                     tag,
		Name:                    name,
		Description:             description,
		IDAsset:                 idAsset,
		TagAsset:                tagAsset,
		Asset:                   asset,
		IDAction:                idAction,
		TagAction:               tagAction,
		Action:                  action,
		IDStatus:                cmm.GenericStatusActive.ID,
		TagStatus:               cmm.GenericStatusActive.Tag,
		Status:                  cmm.GenericStatusActive.Name,
		Conditions:              []conditionTriggers{},
		MaxCandlestickQty:       maxCandles,
		IDCandlestickTimeframe:  idTimeframe,
		TagCandlestickTimeframe: tagTimeframe,
		CandlestickTimeframe:    timeframe,

		// Auditoría
		CoreDB:      os.Getenv("JM_CTRD_MDB_NAME"),
		CoreVersion: "1.0.0", // TODO: poner version
		UserDB:      os.Getenv("JM_CTRD_MDB_USER"),
		SchemaDB:    "1.0.0", // TODO: mejorar estructura

		CreatedDate:   now,
		LastUpdatedAt: now,
		LogicalDelete: false,
	}

	if opts != nil {
		for _, condition := range opts.Conditions {
			trigger.Conditions = append(trigger.Conditions, conditionTriggers{
				IDCondition:          condition.ID,
				IDProvider:           condition.IDProvider,
				NameProvider:         condition.NameProvider,
				NameCondition:        condition.Name,
				DescriptionCondition: condition.Description,
			})
		}
		if opts.IdWebhook != nil {
			trigger.IDWebhook = *opts.IdWebhook
		}
		trigger.IdReference = opts.IdReference
		trigger.IdReferenceRoot = opts.IdReferenceRoot
		trigger.PineCode = opts.PineCode
	}

	return trigger, cmm.NewEmptyErrorHandler()
}
