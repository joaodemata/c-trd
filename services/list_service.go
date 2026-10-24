package services

import (
	"context"
	"regexp"
	"time"

	cmm "c_trd/common"
	"c_trd/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetTriggerService busca un trigger no borrado por su ID. Si no existe retorna el trigger vacio sin error.
// Retorna el pine_code para copiarlo al trigger que lo reemplaza
func GetTriggerService(idTrigger primitive.ObjectID) (models.TriggersModelType, *cmm.ErrorHandler) {
	var result models.TriggersModelType

	// Set timeout context for the query
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create filter and variable to hold the result
	// TODO: agregar el idStatus al query
	filter := bson.M{"_id": idTrigger, "logical_delete": false}

	// Extraemos la data que nos importa
	opts := options.FindOne().SetProjection(bson.M{
		"last_updated_at": 0,
		"created_date":    0,
		"logical_delete":  0,
		"core_db":         0,
		"core_version":    0,
		"user_db":         0,
		"schema_db":       0,
	})

	// 5. Execute query
	err := models.TriggersModel.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE002")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// Obtener listado de oportunidades
// GetOpportunitiesService obtiene un listado paginado de oportunidades basado en filtros

// Retorna: el arreglo de datos, el total de documentos encontrados (para paginación) y el error handler
func GetOpportunitiesService(Limit int, Page int, Search string, StartDate time.Time, EndDate time.Time) ([]models.OpportunitiesModelType, int64, *cmm.ErrorHandler) {
	// Inicializamos el slice donde guardaremos los resultados (vacío por defecto, no nil)
	result := make([]models.OpportunitiesModelType, 0)

	// Validaciones de seguridad para la paginación
	if Limit <= 0 {
		Limit = 10 // Valor por defecto
	}
	if Page <= 0 {
		Page = 1
	}
	skip := int64((Page - 1) * Limit)
	limit64 := int64(Limit)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Subimos un poco el timeout para listados
	defer cancel()

	// 1. Construir el Filtro (Query)
	filter := bson.M{"logical_delete": false}

	// Filtro por Fechas (ej: sobre created_date)
	if !StartDate.IsZero() && !EndDate.IsZero() {
		filter["created_date"] = bson.M{
			"$gte": StartDate,
			"$lte": EndDate,
		}
	} else if !StartDate.IsZero() {
		filter["created_date"] = bson.M{"$gte": StartDate}
	} else if !EndDate.IsZero() {
		filter["created_date"] = bson.M{"$lte": EndDate}
	}

	// Filtro por Búsqueda de Texto (Search)
	if Search != "" {
		// Creamos un regex para que la búsqueda sea case-insensitive (opción "i")
		regexPattern := primitive.Regex{Pattern: Search, Options: "i"}

		// Buscamos en varios campos al mismo tiempo usando $or
		filter["$or"] = []bson.M{
			{"asset": regexPattern},
			{"action": regexPattern},
			{"status": regexPattern},
			{"tag_trigger": regexPattern},
		}
	}

	// 2. Contar el total de documentos reales (sin limit ni skip) para el paginador del frontend
	totalDocs, err := models.OpportunitiesModel.CountDocuments(ctx, filter)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error contando documentos", cmm.LevelDatabase, "SLISE003")
	}

	// Si no hay documentos, retornamos de inmediato un arreglo vacío
	if totalDocs == 0 {
		return result, 0, cmm.NewEmptyErrorHandler()
	}

	// 3. Opciones del Query (Skip, Limit, Sort y Projection)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(limit64).
		SetSort(bson.M{"created_date": -1}). // Ordenar por los más recientes primero
		SetProjection(bson.M{
			// "metadata": 0, // Descomenta si quieres excluir metadatos pesados en el listado
			"logical_delete": 0,
			"core_db":        0,
			"core_version":   0,
			"user_db":        0,
			"schema_db":      0,
		})

	// 4. Ejecutar el Query (Usar OpportunityModel en vez de TriggersModel)
	cursor, err := models.OpportunitiesModel.Find(ctx, filter, opts)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error consultando listado", cmm.LevelDatabase, "SLISE004")
	}
	// Cerramos el cursor al terminar de procesar
	defer cursor.Close(ctx)

	// 5. Decodificar todos los resultados en el Slice
	if err = cursor.All(ctx, &result); err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error decodificando cursor", cmm.LevelDatabase, "SLISE005")
	}

	return result, totalDocs, cmm.NewEmptyErrorHandler()
}

// GetActiveOpportunity busca si hay una oportunidad en curso o activa de algun idTrigger
func GetActiveOpportunity(idTrigger primitive.ObjectID) (models.OpportunitiesModelType, *cmm.ErrorHandler) {
	var result models.OpportunitiesModelType

	// 2. Crear contexto con timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 3. Crear filtro: idTrigger, status en curso o activo y que no esté borrado lógicamente
	filter := bson.M{
		"id_trigger": idTrigger,
		"tag_status": bson.M{"$in": []string{
			cmm.OpportunityStatusOngoing.Tag,
			cmm.OpportunityStatusActive.Tag,
		}},
		"logical_delete": false,
	}

	// 4. Ejecutar consulta FindOne y decodificar directamente en result
	err := models.OpportunitiesModel.FindOne(ctx, filter).Decode(&result)
	// Check error
	if err != nil {
		// Manejar el caso donde no existe ninguna oportunidad activa para ese trigger
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler() // Retorna el struct vacío y ningún error
		}

		// Manejar cualquier otro error de base de datos
		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE006")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetAssetService busca un asset no borrado por su ID. Si no existe retorna el asset vacio sin error
func GetAssetService(idAsset primitive.ObjectID) (models.AssetsModelType, *cmm.ErrorHandler) {
	var result models.AssetsModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": idAsset, "logical_delete": false}
	opts := options.FindOne().SetProjection(bson.M{"_id": 1, "tag": 1, "name": 1})

	err := models.AssetsModel.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SASSE001")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetActiveApisService lista paginadas las APIs activas con lo que el frontend necesita para crear una condicion:
// sus parametros y su formato de respuesta. No retorna la configuracion de la peticion (url, auth, data fija).
// La busqueda es literal (sin regex) sobre el nombre, tag, descripcion y proveedor; las fechas filtran created_date.
// Retorna: las APIs de la pagina, el total de documentos encontrados (para paginación) y el error handler
func GetActiveApisService(Limit int, Page int, Search string, StartDate time.Time, EndDate time.Time) ([]models.ApisModelType, int64, *cmm.ErrorHandler) {
	result := make([]models.ApisModelType, 0)

	// Validaciones de seguridad para la paginación
	if Limit <= 0 {
		Limit = 10 // Valor por defecto
	}
	if Page <= 0 {
		Page = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"is_active": true, "logical_delete": false}

	if !StartDate.IsZero() && !EndDate.IsZero() {
		filter["created_date"] = bson.M{"$gte": StartDate, "$lte": EndDate}
	} else if !StartDate.IsZero() {
		filter["created_date"] = bson.M{"$gte": StartDate}
	} else if !EndDate.IsZero() {
		filter["created_date"] = bson.M{"$lte": EndDate}
	}

	if Search != "" {
		// Se escapa el texto para que la busqueda no se interprete como regex
		regexPattern := primitive.Regex{Pattern: regexp.QuoteMeta(Search), Options: "i"}
		filter["$or"] = []bson.M{
			{"name": regexPattern},
			{"tag": regexPattern},
			{"description": regexPattern},
			{"name_provider": regexPattern},
		}
	}

	// Total de documentos sin limit ni skip para el paginador del frontend
	totalDocs, err := models.ApisModel.CountDocuments(ctx, filter)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error contando las APIs", cmm.LevelDatabase, "SLISE014")
	}
	if totalDocs == 0 {
		return result, 0, cmm.NewEmptyErrorHandler()
	}

	opts := options.Find().
		SetSkip(int64((Page - 1) * Limit)).
		SetLimit(int64(Limit)).
		SetSort(bson.M{"name": 1}).
		SetProjection(bson.M{
			"_id":             1,
			"id_provider":     1,
			"tag_provider":    1,
			"name_provider":   1,
			"tag":             1,
			"name":            1,
			"description":     1,
			"is_active":       1,
			"response_format": 1,
			"parameters":      1,
		})

	cursor, err := models.ApisModel.Find(ctx, filter, opts)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error consultando las APIs", cmm.LevelDatabase, "SLISE007")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error decodificando las APIs", cmm.LevelDatabase, "SLISE008")
	}

	// Una API sin parametros en base de datos se retorna con [] para el frontend
	for i := range result {
		if result[i].Parameters == nil {
			result[i].Parameters = []models.ApiParameter{}
		}
	}

	return result, totalDocs, cmm.NewEmptyErrorHandler()
}

// GetActiveApiService busca una API activa y no borrada por su ID. Si no existe retorna la API vacia sin error
func GetActiveApiService(idApi primitive.ObjectID) (models.ApisModelType, *cmm.ErrorHandler) {
	var result models.ApisModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": idApi, "is_active": true, "logical_delete": false}

	err := models.ApisModel.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE009")
	}

	return result, cmm.NewEmptyErrorHandler()
}

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

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE010")
	}

	return result, cmm.NewEmptyErrorHandler()
}

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

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE011")
	}

	return result, cmm.NewEmptyErrorHandler()
}

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
		return result, cmm.NewErrorHandler(err, "Error consultando los proveedores", cmm.LevelDatabase, "SLISE012")
	}
	defer cursor.Close(ctx)

	found := make([]models.ProvidersModelType, 0)
	if err = cursor.All(ctx, &found); err != nil {
		return result, cmm.NewErrorHandler(err, "Error decodificando los proveedores", cmm.LevelDatabase, "SLISE013")
	}

	for _, provider := range found {
		result[provider.ID] = provider
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetAssetsService lista paginados los assets no borrados para los selectores del frontend. La busqueda es literal
// (sin regex) sobre el nombre, el tag y el tipo de asset.
// Retorna: los assets de la pagina, el total de documentos encontrados (para paginación) y el error handler
func GetAssetsService(Limit int, Page int, Search string) ([]models.AssetsModelType, int64, *cmm.ErrorHandler) {
	result := make([]models.AssetsModelType, 0)

	// Validaciones de seguridad para la paginación
	if Limit <= 0 {
		Limit = 10 // Valor por defecto
	}
	if Page <= 0 {
		Page = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"logical_delete": false}
	if Search != "" {
		regexPattern := primitive.Regex{Pattern: regexp.QuoteMeta(Search), Options: "i"}
		filter["$or"] = []bson.M{
			{"name": regexPattern},
			{"tag": regexPattern},
			{"type_name": regexPattern},
		}
	}

	totalDocs, err := models.AssetsModel.CountDocuments(ctx, filter)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error contando los assets", cmm.LevelDatabase, "SLISE015")
	}
	if totalDocs == 0 {
		return result, 0, cmm.NewEmptyErrorHandler()
	}

	opts := options.Find().
		SetSkip(int64((Page - 1) * Limit)).
		SetLimit(int64(Limit)).
		SetSort(bson.M{"name": 1}).
		SetProjection(bson.M{
			"_id":           1,
			"id_type_asset": 1,
			"type_tag":      1,
			"type_name":     1,
			"tag":           1,
			"name":          1,
			"description":   1,
			"tag_status":    1,
			"status":        1,
		})

	cursor, err := models.AssetsModel.Find(ctx, filter, opts)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error consultando los assets", cmm.LevelDatabase, "SLISE016")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error decodificando los assets", cmm.LevelDatabase, "SLISE017")
	}

	return result, totalDocs, cmm.NewEmptyErrorHandler()
}

// GetCatalogsByTypeService lista los catalogos no borrados de un type_catalog ordenados por nombre. Si el tipo no
// existe retorna un arreglo vacio sin error
func GetCatalogsByTypeService(typeCatalog string) ([]models.CatalogsModelType, *cmm.ErrorHandler) {
	result := make([]models.CatalogsModelType, 0)

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"type_catalog": typeCatalog, "logical_delete": false}
	opts := options.Find().
		SetSort(bson.M{"name": 1}).
		SetProjection(bson.M{"_id": 1, "type_catalog": 1, "tag": 1, "name": 1, "description": 1})

	cursor, err := models.CatalogsModel.Find(ctx, filter, opts)
	if err != nil {
		return result, cmm.NewErrorHandler(err, "Error consultando los catalogos", cmm.LevelDatabase, "SLISE018")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, cmm.NewErrorHandler(err, "Error decodificando los catalogos", cmm.LevelDatabase, "SLISE019")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetConditionsService lista paginadas las condiciones no borradas, las mas recientes primero. No retorna la peticion
// (http_request) ni los campos internos del cronjob. La busqueda es literal (sin regex) sobre el nombre, la descripcion,
// el proveedor y el callback.
// Retorna: las condiciones de la pagina, el total de documentos encontrados (para paginación) y el error handler
func GetConditionsService(Limit int, Page int, Search string) ([]models.ConditionsModelType, int64, *cmm.ErrorHandler) {
	result := make([]models.ConditionsModelType, 0)

	// Validaciones de seguridad para la paginación
	if Limit <= 0 {
		Limit = 10 // Valor por defecto
	}
	if Page <= 0 {
		Page = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"logical_delete": false}
	if Search != "" {
		regexPattern := primitive.Regex{Pattern: regexp.QuoteMeta(Search), Options: "i"}
		filter["$or"] = []bson.M{
			{"name_condition": regexPattern},
			{"description_condition": regexPattern},
			{"name_provider": regexPattern},
			{"callback_function": regexPattern},
		}
	}

	totalDocs, err := models.ConditionsModel.CountDocuments(ctx, filter)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error contando las condiciones", cmm.LevelDatabase, "SLISE020")
	}
	if totalDocs == 0 {
		return result, 0, cmm.NewEmptyErrorHandler()
	}

	opts := options.Find().
		SetSkip(int64((Page - 1) * Limit)).
		SetLimit(int64(Limit)).
		SetSort(bson.M{"created_date": -1}).
		SetProjection(bson.M{
			"_id":                   1,
			"id_provider":           1,
			"name_provider":         1,
			"name_condition":        1,
			"description_condition": 1,
			"is_webhook_only":       1,
			"interval_seconds":      1,
			"next_execution_at":     1,
			"last_executed_at":      1,
			"id_api":                1,
			"callback_function":     1,
			"id_status":             1,
			"tag_status":            1,
			"status":                1,
			"created_date":          1,
		})

	cursor, err := models.ConditionsModel.Find(ctx, filter, opts)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error consultando las condiciones", cmm.LevelDatabase, "SLISE021")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error decodificando las condiciones", cmm.LevelDatabase, "SLISE022")
	}

	return result, totalDocs, cmm.NewEmptyErrorHandler()
}

// GetOngoingOpportunitiesConditionsService lista las oportunidades en curso, no expiradas y con condiciones. Solo
// retorna el id y las condiciones para comparar sus status con los de la coleccion de condiciones
func GetOngoingOpportunitiesConditionsService() ([]models.OpportunitiesModelType, *cmm.ErrorHandler) {
	result := make([]models.OpportunitiesModelType, 0)

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{
		"tag_status":      cmm.OpportunityStatusOngoing.Tag,
		"logical_delete":  false,
		"expiration_date": bson.M{"$gt": time.Now().UTC()},
		"conditions.0":    bson.M{"$exists": true},
	}
	opts := options.Find().SetProjection(bson.M{"_id": 1, "conditions": 1})

	cursor, err := models.OpportunitiesModel.Find(ctx, filter, opts)
	if err != nil {
		return result, cmm.NewErrorHandler(err, "Error consultando las oportunidades en curso", cmm.LevelDatabase, "SLISE023")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, cmm.NewErrorHandler(err, "Error decodificando las oportunidades en curso", cmm.LevelDatabase, "SLISE024")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetAssetPriceConfigService retorna la configuracion del precio de mercado del asset no borrado (configs.price_provider
// y configs.price_symbol). Si el asset no existe o no la tiene retorna la configuracion vacia sin error
func GetAssetPriceConfigService(idAsset primitive.ObjectID) (models.AssetPriceConfig, *cmm.ErrorHandler) {
	var result struct {
		Configs models.AssetPriceConfig `bson:"configs"`
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": idAsset, "logical_delete": false}
	opts := options.FindOne().SetProjection(bson.M{"configs.price_provider": 1, "configs.price_symbol": 1})

	err := models.AssetsModel.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result.Configs, cmm.NewEmptyErrorHandler()
		}

		return result.Configs, cmm.NewErrorHandler(err, "Error consultando la configuracion de precio del asset", cmm.LevelDatabase, "SLISE025")
	}

	return result.Configs, cmm.NewEmptyErrorHandler()
}

// GetProviderByTagService busca un proveedor no borrado por su tag con sus datos de autenticacion.
// Si no existe retorna el proveedor vacio sin error
func GetProviderByTagService(tag string) (models.ProvidersModelType, *cmm.ErrorHandler) {
	var result models.ProvidersModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"tag": tag, "logical_delete": false}
	opts := options.FindOne().SetProjection(bson.M{"tag": 1, "name": 1, "secret_ref": 1, "allowed_hosts": 1})

	err := models.ProvidersModel.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, "Error consultando el proveedor", cmm.LevelDatabase, "SLISE026")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetCompletedOpportunitiesService lista las oportunidades listas para pasar a ACTIVE (ver completedOpportunitiesFilter).
// Solo retorna el id y el asset para consultar el precio de mercado antes de activarlas
func GetCompletedOpportunitiesService(now time.Time) ([]models.OpportunitiesModelType, *cmm.ErrorHandler) {
	result := make([]models.OpportunitiesModelType, 0)

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	opts := options.Find().SetProjection(bson.M{"_id": 1, "id_asset": 1})

	cursor, err := models.OpportunitiesModel.Find(ctx, completedOpportunitiesFilter(now), opts)
	if err != nil {
		return result, cmm.NewErrorHandler(err, "Error consultando las oportunidades por activar", cmm.LevelDatabase, "SLISE027")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, cmm.NewErrorHandler(err, "Error decodificando las oportunidades por activar", cmm.LevelDatabase, "SLISE028")
	}

	return result, cmm.NewEmptyErrorHandler()
}
