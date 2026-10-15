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

// ConditionStatusChange nuevo status de una condicion que debe replicarse en las oportunidades
type ConditionStatusChange struct {
	IDCondition primitive.ObjectID
	NewStatus   cmm.CatalogStatus
}

// UpdateOpportunitiesConditionsStatusService replica el status de las condiciones en las oportunidades en curso.
// Recibe el ctx de la transaccion (SessionContext)
func UpdateOpportunitiesConditionsStatusService(ctx context.Context, changes []ConditionStatusChange) *cmm.ErrorHandler {
	if len(changes) == 0 {
		return cmm.NewEmptyErrorHandler()
	}

	now := time.Now().UTC()
	writes := make([]mongo.WriteModel, 0, len(changes))

	for _, c := range changes {
		filter := bson.M{
			"tag_status":              cmm.OpportunityStatusOngoing.Tag,
			"logical_delete":          false,
			"conditions.id_condition": c.IDCondition,
		}
		update := bson.M{
			"$set": bson.M{
				"conditions.$[cond].id_status":  c.NewStatus.ID,
				"conditions.$[cond].tag_status": c.NewStatus.Tag,
				"conditions.$[cond].status":     c.NewStatus.Name,
				"last_updated_at":               now,
			},
		}
		arrayFilters := options.ArrayFilters{Filters: []interface{}{bson.M{"cond.id_condition": c.IDCondition}}}

		writes = append(writes, mongo.NewUpdateManyModel().
			SetFilter(filter).
			SetUpdate(update).
			SetArrayFilters(arrayFilters))
	}

	if _, err := models.OpportunitiesModel.BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false)); err != nil {
		return cmm.NewErrorHandler(err, "Error actualizando las condiciones de las oportunidades.", cmm.LevelDatabase, "SOPPE001")
	}

	return cmm.NewEmptyErrorHandler()
}

// ActivateCompletedOpportunitiesService cambia a ACTIVE las oportunidades en curso, no expiradas,
// que tengan al menos una condicion y todas sus condiciones activas
func ActivateCompletedOpportunitiesService() (int64, *cmm.ErrorHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	now := time.Now().UTC()

	filter := bson.M{
		"tag_status":      cmm.OpportunityStatusOngoing.Tag,
		"logical_delete":  false,
		"expiration_date": bson.M{"$gt": now},
		"conditions.0":    bson.M{"$exists": true},
		"conditions": bson.M{
			"$not": bson.M{
				"$elemMatch": bson.M{"tag_status": bson.M{"$ne": cmm.GenericStatusActive.Tag}},
			},
		},
	}
	update := bson.M{
		"$set": bson.M{
			"id_status":       cmm.OpportunityStatusActive.ID,
			"tag_status":      cmm.OpportunityStatusActive.Tag,
			"status":          cmm.OpportunityStatusActive.Name,
			"last_updated_at": now,
		},
	}

	result, err := models.OpportunitiesModel.UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, cmm.NewErrorHandler(err, "Error activando las oportunidades.", cmm.LevelDatabase, "SOPPE002")
	}

	return result.ModifiedCount, cmm.NewEmptyErrorHandler()
}
