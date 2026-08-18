package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Condiciones
type conditionTriggers struct {
	IDCondition          primitive.ObjectID `bson:"id_condition" json:"id_condition" description:"Id de la condicion"`
	IDProvider           primitive.ObjectID `bson:"id_provider" json:"id_provider" description:"Id del proveedor de informacion del servicio"`
	NameProvider         string             `bson:"name_provider" json:"name_provider" description:" Proveedor de informacion del servicio"`
	NameCondition        string             `bson:"name_condition" json:"name_condition" `
	DescriptionCondition string           `bson:"description_condition" json:"description_condition"`	
}

type TriggersModelType struct {
	ID                      primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name                    string             `bson:"name" json:"name" description:"Nombre del trigger"`
	Description             string             `bson:"description" json:"description" description:"Que principio utiliza este trigger"`
	//Asset
	IDAsset                 primitive.ObjectID `bson:"id_asset" json:"idAsset"`
	TagAsset                string             `bson:"tag_asset" json:"tagAsset"` 
	Asset                   string             `bson:"asset" json:"asset"`
	// Action
	IDAction                primitive.ObjectID `bson:"id_action" json:"idAction"`
	TagAction               string             `bson:"tag_action" json:"tagAction"`
	Action                  string             `bson:"action" json:"action"`
	//Status
	IDStatus                primitive.ObjectID `bson:"id_status" json:"idStatus"`
	TagStatus               string             `bson:"tag_status" json:"tagStatus"`
	Status                  string             `bson:"status" json:"status"`
	// Conditions
	Conditions              []conditionTriggers`bson:"conditions" json:"conditions"`
	// Candles
	MaxCandlestickQty       int                `bson:"max_candlestick_qty" json:"maxCandlestickQty" description:"Maxima Cantidad de velas que puede haber para la oportunidad pasar a descartada"`
	IDCandlestickTimeframe  primitive.ObjectID `bson:"id_candlestick_timeframe" json:"idCandlestickTimeframe" description:"Tiempo que representa cada vela"`
    TagCandlestickTimeframe string             `bson:"tag_candlestick_timeframe" json:"tagCandlestickTimeframe"`
	CandlestickTimeframe    string             `bson:"candlestick_timeframe" json:"candlestickTimeframe"`
	// Extra
	IdReference             primitive.ObjectID `bson:"id_reference" json:"idReference" description:"Trigger generado a partir de otro trigger"`
	PineCode			    string             `bson:"pine_code" json:"pineCode" description:"Codigo de pine utilizado en trading view para el trigger"`



	// Common
	LastUpdatedAt time.Time          `bson:"last_updated_at" json:"lastUpdatedAt" description:"Ultima Actualizacion"`
	CreatedDate   time.Time          `bson:"created_date" json:"createdDate" description:"Fecha de Creacion"`
	LogicalDelete bool               `bson:"logical_delete" json:"logicalDelete" description:"Borrado Logico"`
	CoreDB        string             `bson:"core_db" json:"coreDB" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string             `bson:"core_version" json:"coreVersion" description:"Version del Core"`
	UserDB        string             `bson:"user_db" json:"userDB" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string             `bson:"schema_db" json:"schemaDB" description:"Versionado de la Base de datos"`
}


// Model pointer
var TriggersModel *mongo.Collection


// Helpers 





