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

// GetCatalogService busca un catalogo no borrado por su ID y su type_catalog. Si no existe o es de otro tipo
// retorna el catalogo vacio sin error
func GetCatalogService(idCatalog primitive.ObjectID, typeCatalog string) (models.CatalogsModelType, *cmm.ErrorHandler) {
	var result models.CatalogsModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": idCatalog, "type_catalog": typeCatalog, "logical_delete": false}
	opts := options.FindOne().SetProjection(bson.M{"_id": 1, "type_catalog": 1, "tag": 1, "name": 1})

	err := models.CatalogsModel.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SCATE001")
	}

	return result, cmm.NewEmptyErrorHandler()
}
