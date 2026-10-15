package services

import (
	"c_trd/models"
	"context"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetProvidersByIDsService obtiene los proveedores activos con los ids indicados, indexados por id
func GetProvidersByIDsService(ids []primitive.ObjectID) (map[primitive.ObjectID]models.ProvidersModelType, *cmm.ErrorHandler) {
	result := make(map[primitive.ObjectID]models.ProvidersModelType)
	if len(ids) == 0 {
		return result, cmm.NewEmptyErrorHandler()
	}

	// Muchas condiciones comparten proveedor, eliminamos los ids repetidos antes de consultar
	seen := make(map[primitive.ObjectID]struct{}, len(ids))
	uniqueIDs := make([]primitive.ObjectID, 0)
	for _, id := range ids {
		if _, exists := seen[id]; !exists {
			seen[id] = struct{}{}
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{
		"_id":            bson.M{"$in": uniqueIDs},
		"logical_delete": false,
	}

	// Solo necesitamos los datos de autenticacion del proveedor
	opts := options.Find().SetProjection(bson.M{
		"tag":           1,
		"name":          1,
		"secret_ref":    1,
		"allowed_hosts": 1,
	})

	cursor, err := models.ProvidersModel.Find(ctx, filter, opts)
	if err != nil {
		return result, cmm.NewErrorHandler(err, "Error consultando los proveedores", cmm.LevelDatabase, "SPROVE001")
	}
	defer cursor.Close(ctx)

	providers := make([]models.ProvidersModelType, 0)
	if err = cursor.All(ctx, &providers); err != nil {
		return result, cmm.NewErrorHandler(err, "Error decodificando los proveedores", cmm.LevelDatabase, "SPROVE002")
	}

	for _, provider := range providers {
		result[provider.ID] = provider
	}

	return result, cmm.NewEmptyErrorHandler()
}
