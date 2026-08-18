package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// RiskManagement define los objetivos de precio y su probabilidad estimada
type RiskManagement struct {
	TargetPrice float64  `bson:"target_price" json:"target_price" description:"Precio objetivo de salida o toma de ganancias"`
	Probability *float64 `bson:"probability,omitempty" json:"probability,omitempty" description:"Probabilidad estimada de alcanzar el precio (opcional)"`
}

// Condiciones
type condition struct {
	IDCondition        primitive.ObjectID `bson:"id_condition" json:"id_condition" description:"Id de la condición"`
	IDProvider         primitive.ObjectID `bson:"id_provider" json:"id_provider" description:"Id del proveedor de informacion del servicio"`
	NameProvider       string             `bson:"name_provider" json:"name_provider" description:" Proveedor de informacion del servicio"`
	NameCondition      string             `bson:"name_condition" json:"name_condition"`
	DescriptionCondition string           `bson:"description_condition" json:"description_condition"`

	
	// Status
	IDStatus             primitive.ObjectID `bson:"id_status" json:"id_status"`
	TagStatus            string             `bson:"tag_status" json:"tag_status" description:"Tag estatus de la condicion"`
	Status               string             `bson:"status" json:"status"`
}

type OpportunitiesModelType struct {
	ID                     primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	IDAsset                primitive.ObjectID `bson:"id_asset" json:"id_asset"`
	Asset                  string             `bson:"asset" json:"asset"`
	IDAction               primitive.ObjectID `bson:"id_action" json:"id_action"`
	Action                 string             `bson:"action" json:"action"`
	IDTrigger              primitive.ObjectID `bson:"id_trigger" json:"id_trigger"`
	Trigger                string             `bson:"trigger" json:"trigger"`
	IDStatus               primitive.ObjectID `bson:"id_status" json:"id_status"`
	Status                 string             `bson:"status" json:"status"`
	Conditions             []condition        `bson:"conditions" json:"conditions"`
	MaxCandlestickQty      int                `bson:"max_candlestick_qty" json:"max_candlestick_qty" description:"Maxima Cantidad de velas que puede haber para la oportunidad pasar a descartada"`
	IDCandlestickTimeframe primitive.ObjectID `bson:"id_candlestick_timeframe" json:"id_candlestick_timeframe" description:"Tiempo que representa cada vela"`
	CandlestickTimeframe   string             `bson:"candlestick_timeframe" json:"candlestick_timeframe"`
	PriceTriggered         float64            `bson:"price_triggered" json:"price_triggered" description:"Precio en el cual se disparo la oportunidad"`
	PriceActiveOportunity  float64            `bson:"price_active_opportunity" json:"price_active_opportunity" description:"Precio en el cual la oportunidad paso a estar activa"`
	PriceAchieved          float64            `bson:"price_achieved" json:"price_achieved" description:"Precio en el cual la oportunidad paso de status activa a completada o cancelada"`
	IDCancel               primitive.ObjectID `bson:"id_cancel" json:"id_cancel" description:"id de la razon por la cual la oportunidad fue descartada"`
	CancelReason           string             `bson:"cancel_reason" json:"cancel_reason" description:"Razon por la cual la oportunidad fue descartada"`
	
	// Campos de gestión de riesgo
	StopLoss               *RiskManagement    `bson:"stop_loss,omitempty" json:"stop_loss,omitempty" description:"Gestión de riesgo para límite de pérdidas"`
	TakeProfit             *RiskManagement    `bson:"take_profit,omitempty" json:"take_profit,omitempty" description:"Gestión de riesgo para toma de ganancias"`

	Metadata               interface{}        `bson:"metadata,omitempty" json:"metadata,omitempty" description:"Objeto opcional para guardar data"`

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
var OpportunityModel  *mongo.Collection
