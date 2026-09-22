package models

import (
	"os"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// RiskManagement define los objetivos de precio y su probabilidad estimada
type RiskManagement struct {
	TargetPrice float64  `bson:"target_price" json:"targetPrice" description:"Precio objetivo de salida o toma de ganancias"`
	Probability *float64 `bson:"probability,omitempty" json:"probability,omitempty" description:"Probabilidad estimada de alcanzar el precio (opcional)"`
}

// Condiciones en oportunidad, guardamos status para tener un rastreo de porque la operacion fue ejecutada o no
type condition struct {
	IDCondition          primitive.ObjectID `bson:"id_condition" json:"idCondition" description:"Id de la condición"`
	IDProvider           primitive.ObjectID `bson:"id_provider" json:"idProvider" description:"Id del proveedor de informacion del servicio"`
	NameProvider         string             `bson:"name_provider" json:"nameProvider" description:" Proveedor de informacion del servicio"`
	NameCondition        string             `bson:"name_condition" json:"nameCondition"`
	DescriptionCondition string             `bson:"description_condition" json:"descriptionCondition"`

	// Status
	IDStatus  primitive.ObjectID `bson:"id_status" json:"idStatus"`
	TagStatus string             `bson:"tag_status" json:"tagStatus" description:"Tag estatus de la condicion"`
	Status    string             `bson:"status" json:"status"`
}

type OpportunitiesModelType struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	// Trigger
	IDTrigger  primitive.ObjectID `bson:"id_trigger" json:"idTrigger"`
	TagTrigger string             `bson:"tag_trigger" json:"tagTrigger"`
	Trigger    string             `bson:"trigger" json:"trigger"`
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
	Conditions []condition `bson:"conditions" json:"conditions"`
	// Candles
	MaxCandlestickQty       int                `bson:"max_candlestick_qty" json:"maxCandlestickQty" description:"Maxima Cantidad de velas que puede haber para la oportunidad pasar a descartada"`
	IDCandlestickTimeframe  primitive.ObjectID `bson:"id_candlestick_timeframe" json:"idCandlestickTimeframe" description:"Tiempo que representa cada vela"`
	TagCandlestickTimeframe string             `bson:"tag_candlestick_timeframe" json:"tagCandlestickTimeframe"`
	CandlestickTimeframe    string             `bson:"candlestick_timeframe" json:"candlestickTimeframe"`
	// Expiration
	ExpirationDate time.Time `bson:"expiration_date" json:"expirationDate" description:"Tiempo de expiracion de la oportunidad dependiendo del maximo cantidad de velas"`

	// Prices executed
	PriceTriggered        float64 `bson:"price_triggered" json:"priceTriggered" description:"Precio en el cual se disparo la oportunidad"`
	PriceActiveOportunity float64 `bson:"price_active_opportunity" json:"priceActiveOpportunity" description:"Precio en el cual la oportunidad paso a estar activa"`
	PriceAchieved         float64 `bson:"price_achieved" json:"priceAchieved" description:"Precio en el cual la oportunidad paso de status activa a completada o cancelada"`
	// Cancels
	IDCancel        primitive.ObjectID `bson:"id_cancel" json:"idCancel" description:"id de la razon por la cual la oportunidad fue descartada"`
	TagCancelReason string             `bson:"tag_cancel_reason" json:"tagCancelReason" description:"Tag de la Razon por la cual la oportunidad fue descartada"`
	CancelReason    string             `bson:"cancel_reason" json:"cancelReason" description:"Razon por la cual la oportunidad fue descartada"`

	// Campos de gestión de riesgo
	StopLoss   *RiskManagement `bson:"stop_loss,omitempty" json:"stopLoss,omitempty" description:"Gestión de riesgo para límite de pérdidas"`
	TakeProfit *RiskManagement `bson:"take_profit,omitempty" json:"takeProfit,omitempty" description:"Gestión de riesgo para toma de ganancias"`

	Metadata interface{} `bson:"metadata,omitempty" json:"metadata,omitempty" description:"Objeto opcional para guardar data"`

	// Common
	LastUpdatedAt time.Time `bson:"last_updated_at" json:"lastUpdatedAt" description:"Ultima Actualizacion"`
	CreatedDate   time.Time `bson:"created_date" json:"createdDate" description:"Fecha de Creacion"`
	LogicalDelete bool      `bson:"logical_delete" json:"logicalDelete" description:"Borrado Logico"`
	CoreDB        string    `bson:"core_db" json:"core_db" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string    `bson:"core_version" json:"core_version" description:"Version del Core"`
	UserDB        string    `bson:"user_db" json:"user_db" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string    `bson:"schema_db" json:"schema_db" description:"Versionado de la Base de datos"`
}

// Model pointer
var OpportunityModel *mongo.Collection

// CONSTRUCTORS

type OpportunityOptionalData struct {
	ID         primitive.ObjectID // Si viene vacío, lo generamos dentro
	StopLoss   *RiskManagement
	TakeProfit *RiskManagement
	Metadata   interface{}
	Conditions []condition // Incluido aquí porque a veces la oportunidad nace sin condiciones evaluadas
}

// NewOpportunity es el método/constructor que crea una nueva instancia de OpportunitiesModelType.
func NewOpportunityModel(
	// --- Trigger ---
	idTrigger primitive.ObjectID, tagTrigger, nameTrigger string,
	// --- Asset ---
	idAsset primitive.ObjectID, tagAsset, asset string,
	// --- Action ---
	idAction primitive.ObjectID, tagAction, action string,
	// --- Status ---
	idStatus primitive.ObjectID, tagStatus, status string,
	// --- Candles & Expiration ---
	maxCandles int, idTimeframe primitive.ObjectID, tagTimeframe, timeframe string,
	// --- Opcionales (omitempty) ---
	opts *OpportunityOptionalData,
) OpportunitiesModelType {
	// Tiempo actual
	now := time.Now()

	// 1. Calculamos dinámicamente el Expiration Date
	// Multiplicamos la cantidad máxima de velas por lo que dura una sola vela
	candleDuration := cmm.ParseTimeframeTag(tagTimeframe)
	totalDuration := time.Duration(maxCandles) * candleDuration
	calculatedExpiration := now.Add(totalDuration)

	// 1. Instanciamos la oportunidad con todos los campos obligatorios
	opp := OpportunitiesModelType{
		IDTrigger:               idTrigger,
		TagTrigger:              tagTrigger,
		Trigger:                 nameTrigger,
		IDAsset:                 idAsset,
		TagAsset:                tagAsset,
		Asset:                   asset,
		IDAction:                idAction,
		TagAction:               tagAction,
		Action:                  action,
		IDStatus:                idStatus,
		TagStatus:               tagStatus,
		Status:                  status,
		MaxCandlestickQty:       maxCandles,
		IDCandlestickTimeframe:  idTimeframe,
		TagCandlestickTimeframe: tagTimeframe,
		CandlestickTimeframe:    timeframe,
		ExpirationDate:          calculatedExpiration,

		// Auditoría
		CoreDB:      os.Getenv("JM_CTRD_MDB_NAME"),
		CoreVersion: "1.0.0", // TODO: poner version
		UserDB:      os.Getenv("JM_CTRD_MDB_USER"),
		SchemaDB:    "1.0.0", // TODO: mejorar estructura

		// Campos autogenerados por defecto
		CreatedDate:   time.Now(),
		LastUpdatedAt: time.Now(),
		LogicalDelete: false,
	}

	// 2. Evaluamos los datos opcionales extraídos del último parámetro
	if opts != nil {
		// Validamos el ID: Si nos pasaron uno lo usamos, si no, lo generamos aquí dentro
		if opts.ID != primitive.NilObjectID {
			opp.ID = opts.ID
		} else {
			opp.ID = primitive.NewObjectID()
		}

		// Asignamos los campos omitempty directamente (pueden ser nil y mongo los ignorará gracias al omitempty)
		opp.StopLoss = opts.StopLoss
		opp.TakeProfit = opts.TakeProfit
		opp.Metadata = opts.Metadata

		if opts.Conditions != nil {
			opp.Conditions = opts.Conditions
		}
	} else {
		// Si opts es nil (no mandaron datos opcionales), solo aseguramos de generar el ID
		opp.ID = primitive.NewObjectID()
	}

	return opp
}
