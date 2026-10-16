package services

import (
	"c_trd/models"
	"context"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetTriggerByTagService busca un trigger no borrado por su tag. Si no existe retorna el trigger vacio sin error
func GetTriggerByTagService(tag string) (models.TriggersModelType, *cmm.ErrorHandler) {
	var result models.TriggersModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"tag": tag, "logical_delete": false}
	opts := options.FindOne().SetProjection(bson.M{"_id": 1, "tag": 1})

	err := models.TriggersModel.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "STRIE001")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// CreateTriggerService inserta un trigger nuevo y retorna su id
func CreateTriggerService(trigger models.TriggersModelType) (string, *cmm.ErrorHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	result, err := models.TriggersModel.InsertOne(ctx, trigger)
	if err != nil {
		// Si existe un indice unico sobre el tag, dos peticiones simultaneas con el mismo tag terminan aqui
		if mongo.IsDuplicateKeyError(err) {
			return "", cmm.NewErrorHandler(err, "Ya existe un trigger con el tag enviado.", cmm.LevelError, "STRIE002")
		}

		return "", cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "STRIE003")
	}

	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", cmm.NewErrorHandler(nil, "Error parseando el object ID.", cmm.LevelFatal, "STRIE004")
	}

	return insertedID.Hex(), cmm.NewEmptyErrorHandler()
}
