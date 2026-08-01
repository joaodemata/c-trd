package models

import (
	"c_trd/common"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ConditionsModelType struct {
	IDCondition                   primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	
	
	IDProvider           primitive.ObjectID `bson:"id_provider" json:"idProvider"`
	NameProvider         string             `bson:"name_provider" json:"nameProvider"`
	NameCondition        string             `bson:"name_condition" json:"nameCondition"`
	DescriptionCondition string             `bson:"description_condition" json:"descriptionCondition"`
	
	// Common
	CreatedDate            time.Time          `bson:"created_date" json:"created_date"`
	LastUpdate             time.Time          `bson:"last_update" json:"last_update"`
	LogicalDelete          bool               `bson:"logical_delete" json:"logical_delete"`
	Core                   string             `bson:"core" json:"core"`
}

// Expoprt model to be used directly
var ConditionsModel = common.GetCollection("conditions")

