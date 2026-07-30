package models

import (
	"c_trd/common"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OportunitiesModelType struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Timestamp    time.Time          `bson:"timestamp" json:"timestamp"`
}

// Expoprt model to be used directly
var OportunitiesModel, err = common.GetCollection("oportunities")
// Check if there is no error

func init() {
 if err != nil {
        panic(err)
    }
}