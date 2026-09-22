package services

import (
	cmm "c_trd/common"
	"c_trd/models"
	"context"
	"time"

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
	result, err := models.OpportunityModel.InsertOne(ctx, opportunity)
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
