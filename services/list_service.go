package services

import (
	"c_trd/models"
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetTriggerService finds a document by hex ID and returns it as a generic map
func GetTriggerService(idTrigger string) (models.TriggersModelType, error) {
	var result models.TriggersModelType

	// 1. Convert string ID to MongoDB ObjectID
	idTriggerParsed, err := primitive.ObjectIDFromHex(idTrigger)

	if err != nil {
		return result, errors.New("invalid mongodb id format")
	}

	// 2. Set timeout context for the query
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 4. Create filter and variable to hold the result
	//TODO: agregar el idStatus al query
	filter := bson.M{"_id": idTriggerParsed,  "logical_delete": false}

	// Extraemos la data que nos importa
	opts := options.FindOne().SetProjection(bson.M{
		"pine_code": 0, 
		"last_updated_at": 0, 
		"created_date": 0,
		"logical_delete": 0,
		"core_db": 0,
		"core_version": 0,
		"user_db": 0,
		"schema_db": 0,
	})
	
	// 5. Execute query
	err = models.TriggersModel.FindOne(ctx, filter, opts).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, nil
		} 

		return result, err
	}

	return result, nil
}
