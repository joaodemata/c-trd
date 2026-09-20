package services

import (
	"c_trd/models"
	"context"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetTriggerService finds a document by hex ID
func GetTriggerService(idTrigger string) (models.TriggersModelType, *cmm.ErrorHandler) {
	var result models.TriggersModelType

	idTriggerParsed, err := primitive.ObjectIDFromHex(idTrigger)
	
	if err != nil {
		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelFatal, "SLISE001")
	}

	//Set timeout context for the query
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	//Create filter and variable to hold the result
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

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE002")
	}

	return result, cmm.NewEmptyErrorHandler()
}
