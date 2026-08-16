package models

import (
	cmm "c_trd/common"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Condiciones
type conditionTriggers struct {
	Status             string             `bson:"status" json:"status" description:"Id del Estado actual de la condición"`
	IDStatus           primitive.ObjectID `bson:"id_status" json:"id_status" description:"Estado actual de la condición"`
	IDProvider         primitive.ObjectID `bson:"id_provider" json:"id_provider" description:"Id del proveedor de informacion del servicio"`
	NameProvider       string             `bson:"name_provider" json:"name_provider" description:" Proveedor de informacion del servicio"`
	NameCondition      string             `bson:"name_condition" json:"name_condition" `
	DescriptionCondition string           `bson:"description_condition" json:"description_condition"`
}

type TriggersModelType struct {
	ID                     primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name                   string             `bson:"name" json:"name" description:"Nombre del trigger"`
	Description            string             `bson:"description" json:"description" description:"Que principio utiliza este trigger"`
	IDAsset                primitive.ObjectID `bson:"id_asset" json:"id_asset"`
	Asset                  string             `bson:"asset" json:"asset"`
	IDAction               primitive.ObjectID `bson:"id_action" json:"id_action"`
	Action                 string             `bson:"action" json:"action"`
	IDStatus               primitive.ObjectID `bson:"id_status" json:"id_status"`
	Status                 string             `bson:"status" json:"status"`
	Conditions             []conditionTriggers`bson:"conditions" json:"conditions"`
	MaxCandlestickQty      int                `bson:"max_candlestick_qty" json:"max_candlestick_qty" description:"Maxima Cantidad de velas que puede haber para la oportunidad pasar a descartada"`
	IDCandlestickTimeframe primitive.ObjectID `bson:"id_candlestick_timeframe" json:"id_candlestick_timeframe" description:"Tiempo que representa cada vela"`
	CandlestickTimeframe   string             `bson:"candlestick_timeframe" json:"candlestick_timeframe"`
	IdReference            primitive.ObjectID `bson:"id_reference" json:"id_reference" description:"Trigger generado a partir de otro trigger"`
	PineCode			   string             `bson:"pine_code" json:"pine_code" description:"Codigo de pine utilizado en trading view para el trigger"`



	// Common
	LastUpdatedAt time.Time          `bson:"last_updated_at" json:"last_updated_at" description:"Ultima Actualizacion"`
	CreatedDate   time.Time          `bson:"created_date" json:"created_date" description:"Fecha de Creacion"`
	LogicalDelete bool               `bson:"logical_delete" json:"logical_delete" description:"Borrado Logico"`
	CoreDB        string             `bson:"core_db" json:"core_db" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string             `bson:"core_version" json:"core_version" description:"Version del Core"`
	UserDB        string             `bson:"user_db" json:"user_db" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string             `bson:"schema_db" json:"schema_db" description:"Versionado de la Base de datos"`
}


// Expoprt model to be used directly
var TriggersModel = cmm.GetCollection("oportunities")

