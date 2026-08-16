package services

import (
	"c_trd/models"
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// GetTriggerService finds a document by hex ID and returns it as a generic map
func GetTriggerService(idTrigger string) (bson.M, error) {
	// 1. Convert string ID to MongoDB ObjectID
	idTriggerParsed, err := primitive.ObjectIDFromHex(idTrigger)
	if err != nil {
		return nil, errors.New("invalid mongodb id format")
	}

	// 2. Set timeout context for the query
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 4. Create filter and variable to hold the result
	filter := bson.M{"_id": idTriggerParsed}
	var result bson.M

	// 5. Execute query
	err = models.TriggersModel.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("document not found")
		}
		return nil, err
	}

	return result, nil
}
