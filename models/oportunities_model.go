package models

import (
	"c_trd/common"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Condiciones
type Condition struct {
	Status             string             `bson:"status" json:"status" description:"Id del Estado actual de la condición"`
	IDStatus           primitive.ObjectID `bson:"id_status" json:"id_status" description:"Estado actual de la condición"`
	IDProvider         primitive.ObjectID `bson:"id_provider" json:"id_provider" description:"Id del proveedor de informacion del servicio"`
	NameProvider       string             `bson:"name_provider" json:"name_provider" description:" Proveedor de informacion del servicio"`
	NameCondition      string             `bson:"name_condition" json:"name_condition" `
	DescriptionCondition string           `bson:"description_condition" json:"description_condition"`
}

type OportunitiesModelType struct {
	ID                     primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	IDOpportunity          primitive.ObjectID `bson:"id_opportunity" json:"id_opportunity"`
	IDAsset                primitive.ObjectID `bson:"id_asset" json:"id_asset"`
	Asset                  string             `bson:"asset" json:"asset"`
	IDAction               primitive.ObjectID `bson:"id_action" json:"id_action"`
	Action                 string             `bson:"action" json:"action"`
	IDTrigger              primitive.ObjectID `bson:"id_trigger" json:"id_trigger"`
	Trigger                string             `bson:"trigger" json:"trigger"`
	IDStatus               primitive.ObjectID `bson:"id_status" json:"id_status"`
	Status                 string             `bson:"status" json:"status"`
	Conditions             []Condition        `bson:"conditions" json:"conditions"`
	MaxCandlestickQty      int                `bson:"max_candlestick_qty" json:"max_candlestick_qty" description:"Maxima Cantidad de velas que puede haber para la oportunidad pasar a descartada"`
	IDCandlestickTimeframe primitive.ObjectID `bson:"id_candlestick_timeframe" json:"id_candlestick_timeframe" description:"Tiempo que representa cada vela"`
	CandlestickTimeframe   string             `bson:"candlestick_timeframe" json:"candlestick_timeframe"`
	// Common
	CreatedDate            time.Time          `bson:"created_date" json:"created_date"`
	LastUpdate             time.Time          `bson:"last_update" json:"last_update"`
	LogicalDelete          bool               `bson:"logical_delete" json:"logical_delete"`
	Core                   string             `bson:"core" json:"core"`
}


// Expoprt model to be used directly
var OportunitiesModel = common.GetCollection("oportunities")

