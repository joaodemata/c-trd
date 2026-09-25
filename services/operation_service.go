package services

import (
	cmm "c_trd/common"
	"c_trd/models"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CreateOpportunityService inserts a new document into the opportunities collection
func CreateOpportunityService(opportunity models.OpportunitiesModelType) (string, *cmm.ErrorHandler) {
	// 1. Set default metadata values for the new document
	if opportunity.ID == primitive.NilObjectID {
		opportunity.ID = primitive.NewObjectID()
	}

	// 2. Set timeout context for the query //TODO: crear un ctx generico para las operaciones de bd
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 3. Execute query
	result, err := models.OpportunitiesModel.InsertOne(ctx, opportunity)
	if err != nil {
		return "", cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SOPRE001")
	}

	// 4. Extract and parse the inserted ID
	insertedID, ok := result.InsertedID.(primitive.ObjectID)
	if !ok {
		return "", cmm.NewErrorHandler(nil, "Error parseando el object ID.", cmm.LevelFatal, "SOPRE002")
	}

	return insertedID.Hex(), cmm.NewEmptyErrorHandler()
}

// UpdateOpportunitiesStatus actualiza las oportunidades
func UpdateOpportunitiesStatus(ctx context.Context, idCondition primitive.ObjectID, newIDStatus primitive.ObjectID, newTagStatus string, newStatus string,
) *cmm.ErrorHandler {
	// filter
	filter := bson.M{"conditions.id_condition": idCondition}
	// Update
	update := bson.M{
		"$set": bson.M{
			"conditions.$.id_status":  newIDStatus,
			"conditions.$.tag_status": newTagStatus,
			"conditions.$.status":     newStatus,
			"last_updated_at":         time.Now().UTC(), // Actualiza la fecha a nivel raíz del documento
		},
	}

	// Usa el ctx (que será el SessionContext durante la transacción)
	result, err := models.OpportunitiesModel.UpdateMany(ctx, filter, update)
	// Check error
	if err != nil {
		return cmm.NewErrorHandler(err, "Error actualizando las oportunidades.", cmm.LevelDatabase, "SOPRE003")
	}
	// Chequeamos que algo se haya actualizado
	if result.ModifiedCount < 1 {
		return cmm.NewErrorHandler(err, "Ningun registro actualizado.", cmm.LevelDatabase, "SOPRE004")
	}

	return cmm.NewEmptyErrorHandler()
}

// UpdateCondition actualiza la condición
func UpdateCondition(ctx context.Context, idCondition primitive.ObjectID, newIDStatus primitive.ObjectID, newTagStatus string, newStatus string) *cmm.ErrorHandler {
	filter := bson.M{"_id": idCondition}

	// Update
	update := bson.M{
		"$set": bson.M{
			"status":          newStatus,
			"last_updated_at": time.Now().UTC(),
		},
	}

	// Usa el ctx
	_, err := models.ConditionsModel.UpdateOne(ctx, filter, update)
	// Check error
	if err != nil {
		return cmm.NewErrorHandler(err, "Error actualizando las oportunidades.", cmm.LevelDatabase, "SOPRE005")
	}

	return cmm.NewEmptyErrorHandler()
}
