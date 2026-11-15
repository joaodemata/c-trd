package services

import (
	"context"
	"fmt"
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
// GetOpportunitiesService obtiene un listado paginado de las oportunidades que puede ver idUser: las publicas, las
// suyas y las de lista blanca donde esta en allowed_users. Con idUser cero (usuario sin registro) solo las publicas

// Retorna: el arreglo de datos, el total de documentos encontrados (para paginación) y el error handler
func GetOpportunitiesService(Limit int, Page int, Search string, StartDate time.Time, EndDate time.Time, idUser primitive.ObjectID) ([]models.OpportunitiesModelType, int64, *cmm.ErrorHandler) {
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

	// 1. Construir el Filtro (Query). La visibilidad va en $and para no chocar con el $or de la busqueda
	filter := bson.M{"logical_delete": false, "$and": []bson.M{models.VisibleToUserFilter(idUser)}}

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
	opts := options.FindOne().SetProjection(bson.M{"_id": 1, "type_catalog": 1, "tag": 1, "name": 1, "description": 1})

	err := models.CatalogsModel.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE010")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetTriggerByWebhookService busca la version vigente (no borrada) del trigger con el id de webhook enviado.
// Si no existe retorna el trigger vacio sin error
func GetTriggerByWebhookService(idWebhook primitive.ObjectID) (models.TriggersModelType, *cmm.ErrorHandler) {
	var result models.TriggersModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"id_webhook": idWebhook, "logical_delete": false}
	opts := options.FindOne().SetProjection(bson.M{
		"last_updated_at": 0,
		"created_date":    0,
		"logical_delete":  0,
		"core_db":         0,
		"core_version":    0,
		"user_db":         0,
		"schema_db":       0,
	})

	err := models.TriggersModel.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE036")
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

// GetConditionByTagService busca una condicion no borrada por su tag. Si no existe retorna la condicion vacia sin error
func GetConditionByTagService(tag string) (models.ConditionsModelType, *cmm.ErrorHandler) {
	var result models.ConditionsModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"tag": tag, "logical_delete": false}
	opts := options.FindOne().SetProjection(bson.M{"_id": 1, "tag": 1})

	err := models.ConditionsModel.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE035")
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
// el proveedor y el callback. Con idStatuses solo se listan las condiciones con alguno de esos status; vacio no filtra.
// Retorna: las condiciones de la pagina, el total de documentos encontrados (para paginación) y el error handler
func GetConditionsService(Limit int, Page int, Search string, idStatuses []primitive.ObjectID) ([]models.ConditionsModelType, int64, *cmm.ErrorHandler) {
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
	if len(idStatuses) > 0 {
		filter["id_status"] = bson.M{"$in": idStatuses}
	}
	if Search != "" {
		regexPattern := primitive.Regex{Pattern: regexp.QuoteMeta(Search), Options: "i"}
		filter["$or"] = []bson.M{
			{"tag": regexPattern},
			{"name": regexPattern},
			{"description": regexPattern},
			{"name_type_condition": regexPattern},
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
			"_id":                 1,
			"id_provider":         1,
			"name_provider":       1,
			"tag":                 1,
			"name":                1,
			"description":         1,
			"id_type_condition":   1,
			"tag_type_condition":  1,
			"name_type_condition": 1,
			"is_webhook_only":     1,
			"interval_seconds":    1,
			"next_execution_at":   1,
			"last_executed_at":    1,
			"id_api":              1,
			"callback_function":   1,
			"parameters_callback": 1,
			"id_status":           1,
			"tag_status":          1,
			"status":              1,
			"created_date":        1,
			"id_user":             1,
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

// GetFinishedOpportunitiesService lista las oportunidades listas para pasar a FINISHED (ver finishedOpportunitiesFilter).
// Solo retorna el id y el asset para consultar el precio de mercado antes de finalizarlas
func GetFinishedOpportunitiesService(now time.Time) ([]models.OpportunitiesModelType, *cmm.ErrorHandler) {
	result := make([]models.OpportunitiesModelType, 0)

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	opts := options.Find().SetProjection(bson.M{"_id": 1, "id_asset": 1})

	cursor, err := models.OpportunitiesModel.Find(ctx, finishedOpportunitiesFilter(now), opts)
	if err != nil {
		return result, cmm.NewErrorHandler(err, "Error consultando las oportunidades por finalizar", cmm.LevelDatabase, "SLISE033")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, cmm.NewErrorHandler(err, "Error decodificando las oportunidades por finalizar", cmm.LevelDatabase, "SLISE034")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetTriggersService lista paginados los triggers, los mas recientes primero. No retorna los campos de auditoria.
// Sin ShowReferences solo lista los triggers activos y no borrados; con ShowReferences tambien los inactivos y los
// reemplazados (borrados logicamente al crear su nueva version). TagStatus filtra por tag_status si viene.
// La busqueda es literal (sin regex) sobre el nombre, tag, descripcion, asset, accion y status; las fechas filtran
// created_date. IdReferenceRoot (si no es cero) limita a las versiones de un trigger: la raiz y sus reemplazos.
// Retorna: los triggers de la pagina, el total de documentos encontrados (para paginación) y el error handler
func GetTriggersService(Limit int, Page int, Search string, StartDate time.Time, EndDate time.Time, TagStatus string, ShowReferences bool, IdReferenceRoot primitive.ObjectID) ([]models.TriggersModelType, int64, *cmm.ErrorHandler) {
	result := make([]models.TriggersModelType, 0)

	// Validaciones de seguridad para la paginación
	if Limit <= 0 {
		Limit = 10 // Valor por defecto
	}
	if Page <= 0 {
		Page = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{}
	if !ShowReferences {
		filter["logical_delete"] = false
		filter["tag_status"] = cmm.GenericStatusActive.Tag
		if TagStatus == cmm.GenericStatusPendingForApprove.Tag || TagStatus == cmm.GenericStatusNotApproved.Tag {
			filter["tag_status"] = TagStatus
		}
	} else if TagStatus != "" {
		filter["tag_status"] = TagStatus
	}

	if !StartDate.IsZero() && !EndDate.IsZero() {
		filter["created_date"] = bson.M{"$gte": StartDate, "$lte": EndDate}
	} else if !StartDate.IsZero() {
		filter["created_date"] = bson.M{"$gte": StartDate}
	} else if !EndDate.IsZero() {
		filter["created_date"] = bson.M{"$lte": EndDate}
	}

	// Va en $and porque el $or de primer nivel lo usa la busqueda
	if !IdReferenceRoot.IsZero() {
		filter["$and"] = []bson.M{{"$or": []bson.M{
			{"_id": IdReferenceRoot},
			{"id_reference_root": IdReferenceRoot},
		}}}
	}

	if Search != "" {
		regexPattern := primitive.Regex{Pattern: regexp.QuoteMeta(Search), Options: "i"}
		filter["$or"] = []bson.M{
			{"name": regexPattern},
			{"tag": regexPattern},
			{"description": regexPattern},
			{"asset": regexPattern},
			{"tag_asset": regexPattern},
			{"action": regexPattern},
			{"status": regexPattern},
		}
	}

	totalDocs, err := models.TriggersModel.CountDocuments(ctx, filter)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error contando los triggers", cmm.LevelDatabase, "SLISE029")
	}
	if totalDocs == 0 {
		return result, 0, cmm.NewEmptyErrorHandler()
	}

	opts := options.Find().
		SetSkip(int64((Page - 1) * Limit)).
		SetLimit(int64(Limit)).
		SetSort(bson.M{"created_date": -1}).
		SetProjection(bson.M{
			"id_webhook":     0,
			"logical_delete": 0,
			"core_db":        0,
			"core_version":   0,
			"user_db":        0,
			"schema_db":      0,
		})

	cursor, err := models.TriggersModel.Find(ctx, filter, opts)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error consultando los triggers", cmm.LevelDatabase, "SLISE030")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error decodificando los triggers", cmm.LevelDatabase, "SLISE031")
	}

	return result, totalDocs, cmm.NewEmptyErrorHandler()
}

// GetCatalogByTagService busca un catalogo no borrado por su tag y su type_catalog. Si no existe retorna el catalogo
// vacio sin error
func GetCatalogByTagService(tag string, typeCatalog string) (models.CatalogsModelType, *cmm.ErrorHandler) {
	var result models.CatalogsModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"tag": tag, "type_catalog": typeCatalog, "logical_delete": false}
	opts := options.FindOne().SetProjection(bson.M{"_id": 1, "type_catalog": 1, "tag": 1, "name": 1})

	err := models.CatalogsModel.FindOne(ctx, filter, opts).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, "Error consultando el catalogo", cmm.LevelDatabase, "SLISE032")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetActiveUsersByIDsService busca los usuarios activos y no borrados de ids, solo trae su id. Los que no existen
// o estan inactivos no se retornan
func GetActiveUsersByIDsService(ids []primitive.ObjectID) ([]models.UsersModelType, *cmm.ErrorHandler) {
	result := make([]models.UsersModelType, 0)
	if len(ids) == 0 {
		return result, cmm.NewEmptyErrorHandler()
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": bson.M{"$in": ids}, "is_active": true, "logical_delete": false}
	opts := options.Find().SetProjection(bson.M{"_id": 1})

	cursor, err := models.UsersModel.Find(ctx, filter, opts)
	if err != nil {
		return result, cmm.NewErrorHandler(err, "Error consultando los usuarios", cmm.LevelDatabase, "SLISE037")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, cmm.NewErrorHandler(err, "Error decodificando los usuarios", cmm.LevelDatabase, "SLISE038")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetAgentThesesService lista paginadas las tesis de la cola de los agentes, las mas antiguas primero (orden FIFO).
// tagStatus vacio lista todas
func GetAgentThesesService(Limit int, Page int, Search string, TagStatus string) ([]models.AgentThesesModelType, int64, *cmm.ErrorHandler) {
	result := make([]models.AgentThesesModelType, 0)
	if Limit <= 0 {
		Limit = 10
	}
	if Page <= 0 {
		Page = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"logical_delete": false}
	if TagStatus != "" {
		filter["tag_status"] = TagStatus
	}
	if Search != "" {
		regexPattern := primitive.Regex{Pattern: regexp.QuoteMeta(Search), Options: "i"}
		filter["$or"] = []bson.M{
			{"thesis": regexPattern},
			{"tag_asset": regexPattern},
			{"asset": regexPattern},
		}
	}

	totalDocs, err := models.AgentThesesModel.CountDocuments(ctx, filter)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error contando las tesis", cmm.LevelDatabase, "SLISE039")
	}
	if totalDocs == 0 {
		return result, 0, cmm.NewEmptyErrorHandler()
	}

	opts := options.Find().
		SetSkip(int64((Page - 1) * Limit)).
		SetLimit(int64(Limit)).
		SetSort(bson.D{{Key: "created_date", Value: 1}, {Key: "_id", Value: 1}})

	cursor, err := models.AgentThesesModel.Find(ctx, filter, opts)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error consultando las tesis", cmm.LevelDatabase, "SLISE040")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error decodificando las tesis", cmm.LevelDatabase, "SLISE041")
	}

	return result, totalDocs, cmm.NewEmptyErrorHandler()
}

// GetAgentThesisService busca una tesis no borrada por su ID. Si no existe retorna la tesis vacia sin error
func GetAgentThesisService(idThesis primitive.ObjectID) (models.AgentThesesModelType, *cmm.ErrorHandler) {
	var result models.AgentThesesModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	err := models.AgentThesesModel.FindOne(ctx, bson.M{"_id": idThesis, "logical_delete": false}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE042")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetAgentRunsService lista paginadas las corridas de los agentes, las mas recientes primero. idThesis cero lista todas
func GetAgentRunsService(Limit int, Page int, Search string, TagStatus string, IdThesis primitive.ObjectID) ([]models.AgentRunsModelType, int64, *cmm.ErrorHandler) {
	result := make([]models.AgentRunsModelType, 0)
	if Limit <= 0 {
		Limit = 10
	}
	if Page <= 0 {
		Page = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"logical_delete": false}
	if TagStatus != "" {
		filter["tag_status"] = TagStatus
	}
	if !IdThesis.IsZero() {
		filter["id_thesis"] = IdThesis
	}
	if Search != "" {
		regexPattern := primitive.Regex{Pattern: regexp.QuoteMeta(Search), Options: "i"}
		filter["$or"] = []bson.M{
			{"thesis": regexPattern},
			{"tag_asset": regexPattern},
			{"asset": regexPattern},
		}
	}

	totalDocs, err := models.AgentRunsModel.CountDocuments(ctx, filter)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error contando las corridas", cmm.LevelDatabase, "SLISE043")
	}
	if totalDocs == 0 {
		return result, 0, cmm.NewEmptyErrorHandler()
	}

	opts := options.Find().
		SetSkip(int64((Page - 1) * Limit)).
		SetLimit(int64(Limit)).
		SetSort(bson.D{{Key: "started_at", Value: -1}, {Key: "_id", Value: -1}})

	cursor, err := models.AgentRunsModel.Find(ctx, filter, opts)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error consultando las corridas", cmm.LevelDatabase, "SLISE044")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error decodificando las corridas", cmm.LevelDatabase, "SLISE045")
	}

	return result, totalDocs, cmm.NewEmptyErrorHandler()
}

// GetAgentRunService busca una corrida no borrada por su ID. Si no existe retorna la corrida vacia sin error
func GetAgentRunService(idRun primitive.ObjectID) (models.AgentRunsModelType, *cmm.ErrorHandler) {
	var result models.AgentRunsModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	err := models.AgentRunsModel.FindOne(ctx, bson.M{"_id": idRun, "logical_delete": false}).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE046")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// CountTriggersByConditionService cuenta los triggers no borrados ni rechazados que usan la condicion
func CountTriggersByConditionService(idCondition primitive.ObjectID) (int64, *cmm.ErrorHandler) {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{
		"conditions.id_condition": idCondition,
		"tag_status":              bson.M{"$ne": cmm.GenericStatusNotApproved.Tag},
		"logical_delete":          false,
	}
	total, err := models.TriggersModel.CountDocuments(ctx, filter)
	if err != nil {
		return 0, cmm.NewErrorHandler(err, "Error contando los triggers de la condicion", cmm.LevelDatabase, "SLISE047")
	}

	return total, cmm.NewEmptyErrorHandler()
}

// ---------- Portfolio y transacciones ----------

// GetPortfolioService busca un portfolio no borrado del usuario. Si no existe o es de otro usuario retorna el portfolio
// vacio sin error
func GetPortfolioService(idPortfolio primitive.ObjectID, idUser primitive.ObjectID) (models.PortfolioModelType, *cmm.ErrorHandler) {
	var result models.PortfolioModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": idPortfolio, "id_user": idUser, "logical_delete": false}
	err := models.PortfolioModel.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE048")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetPortfolioByTagService busca un portfolio no borrado del usuario por su tag. Si no existe retorna el portfolio vacio
// sin error
func GetPortfolioByTagService(tag string, idUser primitive.ObjectID) (models.PortfolioModelType, *cmm.ErrorHandler) {
	var result models.PortfolioModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"tag": tag, "id_user": idUser, "logical_delete": false}
	err := models.PortfolioModel.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE049")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetPortfoliosService lista paginados los portfolios del usuario, los mas antiguos primero
func GetPortfoliosService(Limit int, Page int, Search string, StartDate time.Time, EndDate time.Time, idUser primitive.ObjectID) ([]models.PortfolioModelType, int64, *cmm.ErrorHandler) {
	result := make([]models.PortfolioModelType, 0)
	if Limit <= 0 {
		Limit = 10
	}
	if Page <= 0 {
		Page = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"id_user": idUser, "logical_delete": false}
	if !StartDate.IsZero() && !EndDate.IsZero() {
		filter["created_date"] = bson.M{"$gte": StartDate, "$lte": EndDate}
	} else if !StartDate.IsZero() {
		filter["created_date"] = bson.M{"$gte": StartDate}
	} else if !EndDate.IsZero() {
		filter["created_date"] = bson.M{"$lte": EndDate}
	}
	if Search != "" {
		regexPattern := primitive.Regex{Pattern: regexp.QuoteMeta(Search), Options: "i"}
		filter["$or"] = []bson.M{
			{"tag": regexPattern},
			{"name": regexPattern},
			{"description": regexPattern},
		}
	}

	totalDocs, err := models.PortfolioModel.CountDocuments(ctx, filter)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error contando los portfolios", cmm.LevelDatabase, "SLISE050")
	}
	if totalDocs == 0 {
		return result, 0, cmm.NewEmptyErrorHandler()
	}

	opts := options.Find().
		SetSkip(int64((Page - 1) * Limit)).
		SetLimit(int64(Limit)).
		SetSort(bson.D{{Key: "created_date", Value: 1}, {Key: "_id", Value: 1}})

	cursor, err := models.PortfolioModel.Find(ctx, filter, opts)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error consultando los portfolios", cmm.LevelDatabase, "SLISE051")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error decodificando los portfolios", cmm.LevelDatabase, "SLISE052")
	}

	return result, totalDocs, cmm.NewEmptyErrorHandler()
}

// GetPortfolioTransactionService busca una transaccion no borrada del usuario. Si no existe o es de otro usuario
// retorna la transaccion vacia sin error
func GetPortfolioTransactionService(idTransaction primitive.ObjectID, idUser primitive.ObjectID) (models.PortfolioTransactionsModelType, *cmm.ErrorHandler) {
	var result models.PortfolioTransactionsModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": idTransaction, "id_user": idUser, "logical_delete": false}
	err := models.PortfolioTransactionsModel.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE053")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// PortfolioTransactionsFilter filtros opcionales del listado de transacciones de un portfolio. Los ids en cero y el tag
// vacio no filtran
type PortfolioTransactionsFilter struct {
	IDAsset            primitive.ObjectID
	IDOpportunity      primitive.ObjectID
	TagTypeTransaction string
}

// GetPortfolioTransactionsService lista paginadas las transacciones de un portfolio del usuario, las ejecutadas mas
// recientemente primero. startDate y endDate filtran por executed_at
func GetPortfolioTransactionsService(Limit int, Page int, Search string, StartDate time.Time, EndDate time.Time, idPortfolio primitive.ObjectID, idUser primitive.ObjectID, filters PortfolioTransactionsFilter) ([]models.PortfolioTransactionsModelType, int64, *cmm.ErrorHandler) {
	result := make([]models.PortfolioTransactionsModelType, 0)
	if Limit <= 0 {
		Limit = 10
	}
	if Page <= 0 {
		Page = 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"id_portfolio": idPortfolio, "id_user": idUser, "logical_delete": false}
	if !filters.IDAsset.IsZero() {
		filter["id_asset"] = filters.IDAsset
	}
	if !filters.IDOpportunity.IsZero() {
		filter["id_opportunity"] = filters.IDOpportunity
	}
	if filters.TagTypeTransaction != "" {
		filter["tag_type_transaction"] = filters.TagTypeTransaction
	}
	if !StartDate.IsZero() && !EndDate.IsZero() {
		filter["executed_at"] = bson.M{"$gte": StartDate, "$lte": EndDate}
	} else if !StartDate.IsZero() {
		filter["executed_at"] = bson.M{"$gte": StartDate}
	} else if !EndDate.IsZero() {
		filter["executed_at"] = bson.M{"$lte": EndDate}
	}
	if Search != "" {
		regexPattern := primitive.Regex{Pattern: regexp.QuoteMeta(Search), Options: "i"}
		filter["$or"] = []bson.M{
			{"comment": regexPattern},
			{"tag_asset": regexPattern},
			{"name_asset": regexPattern},
		}
	}

	totalDocs, err := models.PortfolioTransactionsModel.CountDocuments(ctx, filter)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error contando las transacciones", cmm.LevelDatabase, "SLISE054")
	}
	if totalDocs == 0 {
		return result, 0, cmm.NewEmptyErrorHandler()
	}

	opts := options.Find().
		SetSkip(int64((Page - 1) * Limit)).
		SetLimit(int64(Limit)).
		SetSort(bson.D{{Key: "executed_at", Value: -1}, {Key: "_id", Value: -1}})

	cursor, err := models.PortfolioTransactionsModel.Find(ctx, filter, opts)
	if err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error consultando las transacciones", cmm.LevelDatabase, "SLISE055")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, 0, cmm.NewErrorHandler(err, "Error decodificando las transacciones", cmm.LevelDatabase, "SLISE056")
	}

	return result, totalDocs, cmm.NewEmptyErrorHandler()
}

// GetVisibleOpportunityService busca una oportunidad no borrada que el usuario puede ver (publica, suya o de lista
// blanca donde esta). Si no existe o no la puede ver retorna la oportunidad vacia sin error
func GetVisibleOpportunityService(idOpportunity primitive.ObjectID, idUser primitive.ObjectID) (models.OpportunitiesModelType, *cmm.ErrorHandler) {
	var result models.OpportunitiesModelType

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := models.VisibleToUserFilter(idUser)
	filter["_id"] = idOpportunity
	filter["logical_delete"] = false

	err := models.OpportunitiesModel.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return result, cmm.NewEmptyErrorHandler()
		}

		return result, cmm.NewErrorHandler(err, err.Error(), cmm.LevelDatabase, "SLISE057")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetAllPortfolioTransactionsService retorna todas las transacciones no borradas de un portfolio del usuario, sin
// paginar, para calcular el ROI
func GetAllPortfolioTransactionsService(idPortfolio primitive.ObjectID, idUser primitive.ObjectID) ([]models.PortfolioTransactionsModelType, *cmm.ErrorHandler) {
	result := make([]models.PortfolioTransactionsModelType, 0)

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"id_portfolio": idPortfolio, "id_user": idUser, "logical_delete": false}
	opts := options.Find().SetSort(bson.D{{Key: "executed_at", Value: 1}, {Key: "_id", Value: 1}})

	cursor, err := models.PortfolioTransactionsModel.Find(ctx, filter, opts)
	if err != nil {
		return result, cmm.NewErrorHandler(err, "Error consultando las transacciones del portfolio", cmm.LevelDatabase, "SLISE058")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, cmm.NewErrorHandler(err, "Error decodificando las transacciones del portfolio", cmm.LevelDatabase, "SLISE059")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetVisibleOpportunitiesByIDsService busca las oportunidades no borradas que el usuario puede ver, por su ID. Las que
// no existen o ya no puede ver no se incluyen en el mapa
func GetVisibleOpportunitiesByIDsService(ids []primitive.ObjectID, idUser primitive.ObjectID) (map[primitive.ObjectID]models.OpportunitiesModelType, *cmm.ErrorHandler) {
	result := make(map[primitive.ObjectID]models.OpportunitiesModelType, len(ids))
	if len(ids) == 0 {
		return result, cmm.NewEmptyErrorHandler()
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := models.VisibleToUserFilter(idUser)
	filter["_id"] = bson.M{"$in": ids}
	filter["logical_delete"] = false

	cursor, err := models.OpportunitiesModel.Find(ctx, filter)
	if err != nil {
		return result, cmm.NewErrorHandler(err, "Error consultando las oportunidades de las transacciones", cmm.LevelDatabase, "SLISE060")
	}
	defer cursor.Close(ctx)

	opportunities := make([]models.OpportunitiesModelType, 0, len(ids))
	if err = cursor.All(ctx, &opportunities); err != nil {
		return result, cmm.NewErrorHandler(err, "Error decodificando las oportunidades de las transacciones", cmm.LevelDatabase, "SLISE061")
	}

	for _, opportunity := range opportunities {
		result[opportunity.ID] = opportunity
	}

	return result, cmm.NewEmptyErrorHandler()
}

// PortfolioAssetRoi posicion del asset con su valor de mercado. Los campos de mercado son null si no se obtuvo el precio
type PortfolioAssetRoi struct {
	models.PortfolioAsset
	MarketPrice   *float64 `json:"marketPrice"`
	MarketValue   *float64 `json:"marketValue"`
	UnrealizedPnL *float64 `json:"unrealizedPnl"`
}

// PortfolioRoiTotals totales del portfolio. Los assets sin precio se valoran a su costo (sin ganancia no realizada)
type PortfolioRoiTotals struct {
	// Total pagado en compras (value + fee), base del ROI sobre lo invertido
	InvestedCost  float64  `json:"investedCost"`
	CostBasis     float64  `json:"costBasis"`
	MarketValue   float64  `json:"marketValue"`
	RealizedPnL   float64  `json:"realizedPnl"`
	UnrealizedPnL float64  `json:"unrealizedPnl"`
	RoiOnInvested *float64 `json:"roiOnInvested"`
	// Movimientos de efectivo: cash = depositos - retiros - compras + ventas + dividendos (con comisiones)
	NetDeposits float64  `json:"netDeposits"`
	Cash        float64  `json:"cash"`
	Equity      float64  `json:"equity"`
	RoiOnEquity *float64 `json:"roiOnEquity"`
}

// OpportunityRoi resultado de las transacciones ligadas a una oportunidad, con su propio costo promedio (no el del
// portfolio). Los datos de la oportunidad quedan vacios si ya no es visible para el usuario
type OpportunityRoi struct {
	IDOpportunity primitive.ObjectID `json:"idOpportunity"`
	TagTrigger    string             `json:"tagTrigger"`
	TagAction     string             `json:"tagAction"`
	TagStatus     string             `json:"tagStatus"`
	IDAsset       primitive.ObjectID `json:"idAsset"`
	TagAsset      string             `json:"tagAsset"`
	Transactions  int                `json:"transactions"`
	// Posicion de la oportunidad
	QuantityBought float64  `json:"quantityBought"`
	QuantitySold   float64  `json:"quantitySold"`
	OpenQuantity   float64  `json:"openQuantity"`
	InvestedCost   float64  `json:"investedCost"`
	Proceeds       float64  `json:"proceeds"`
	AverageCost    float64  `json:"averageCost"`
	RealizedPnL    float64  `json:"realizedPnl"`
	MarketPrice    *float64 `json:"marketPrice"`
	UnrealizedPnL  *float64 `json:"unrealizedPnl"`
	Roi            *float64 `json:"roi"`
	// Cambio de precio entre la activacion y el cierre de la oportunidad (lo planeado) para compararlo con roi (lo
	// ejecutado). null si la oportunidad no tiene esos precios
	PlannedPriceChange *float64 `json:"plannedPriceChange"`
	Warning            string   `json:"warning,omitempty"`
}

// PortfolioRoi ROI del portfolio, de cada asset y de cada oportunidad ligada. partial indica que algun precio no se
// pudo obtener y esos assets se valoraron a su costo
type PortfolioRoi struct {
	IDPortfolio   primitive.ObjectID  `json:"idPortfolio"`
	Partial       bool                `json:"partial"`
	Assets        []PortfolioAssetRoi `json:"assets"`
	Totals        PortfolioRoiTotals  `json:"totals"`
	Opportunities []OpportunityRoi    `json:"opportunities"`
	Warnings      []string            `json:"warnings"`
	CalculatedAt  time.Time           `json:"calculatedAt"`
}

// ratio retorna numerator / denominator, o nil si el denominador no es positivo
func ratio(numerator float64, denominator float64) *float64 {
	if denominator <= 0 {
		return nil
	}
	value := numerator / denominator
	return &value
}

// BuildPortfolioRoiService calcula el ROI con la cartera del portfolio, todas sus transacciones, los precios de
// mercado por asset (los que faltan se valoran a su costo) y las oportunidades visibles ligadas:
//   - roiOnInvested = (ganancia realizada + no realizada) / total pagado en compras
//   - roiOnEquity = (equity - depositos netos) / depositos netos, con equity = cash + valor de mercado. Es null si no
//     hay depositos o si el cash queda negativo (compras sin los depositos que las cubren)
//   - roi de cada oportunidad = (ganancia realizada + no realizada) / total pagado en sus compras, con su propio costo
//     promedio. Es null si tiene ventas mayores a sus compras o le falta el precio con cantidad abierta
func BuildPortfolioRoiService(portfolio models.PortfolioModelType, transactions []models.PortfolioTransactionsModelType, prices map[primitive.ObjectID]float64, opportunities map[primitive.ObjectID]models.OpportunitiesModelType, now time.Time) PortfolioRoi {
	roi := PortfolioRoi{
		IDPortfolio:   portfolio.ID,
		Assets:        make([]PortfolioAssetRoi, 0, len(portfolio.Assets)),
		Opportunities: make([]OpportunityRoi, 0),
		Warnings:      make([]string, 0),
		CalculatedAt:  now,
	}

	// 1. Assets: valor de mercado y ganancia no realizada de cada posicion
	for _, asset := range portfolio.Assets {
		assetRoi := PortfolioAssetRoi{PortfolioAsset: asset}
		roi.Totals.CostBasis += asset.CostBasis
		roi.Totals.RealizedPnL += asset.RealizedPnL

		price, hasPrice := prices[asset.IDAsset]
		if !hasPrice && asset.Quantity > 0 {
			roi.Partial = true
			roi.Totals.MarketValue += asset.CostBasis
			roi.Warnings = append(roi.Warnings, fmt.Sprintf("No se obtuvo el precio de %s, se valoro a su costo", asset.TagAsset))
			roi.Assets = append(roi.Assets, assetRoi)
			continue
		}

		marketValue := asset.Quantity * price
		unrealized := marketValue - asset.CostBasis
		if hasPrice {
			assetRoi.MarketPrice = &price
		}
		assetRoi.MarketValue = &marketValue
		assetRoi.UnrealizedPnL = &unrealized
		roi.Totals.MarketValue += marketValue
		roi.Totals.UnrealizedPnL += unrealized
		roi.Assets = append(roi.Assets, assetRoi)
	}

	// 2. Movimientos de efectivo y total pagado en compras
	byOpportunity := map[primitive.ObjectID][]models.PortfolioTransactionsModelType{}
	opportunityOrder := make([]primitive.ObjectID, 0)
	for _, transaction := range transactions {
		switch transaction.TagTypeTransaction {
		case cmm.TransactionTypeBuy.Tag:
			roi.Totals.InvestedCost += transaction.Value + transaction.Fee
			roi.Totals.Cash -= transaction.Value + transaction.Fee
		case cmm.TransactionTypeSell.Tag, cmm.TransactionTypeDividend.Tag:
			roi.Totals.Cash += transaction.Value - transaction.Fee
		case cmm.TransactionTypeDeposit.Tag:
			roi.Totals.NetDeposits += transaction.Value - transaction.Fee
			roi.Totals.Cash += transaction.Value - transaction.Fee
		case cmm.TransactionTypeWithdrawal.Tag:
			roi.Totals.NetDeposits -= transaction.Value + transaction.Fee
			roi.Totals.Cash -= transaction.Value + transaction.Fee
		}

		if transaction.IDOpportunity == nil {
			continue
		}
		if _, seen := byOpportunity[*transaction.IDOpportunity]; !seen {
			opportunityOrder = append(opportunityOrder, *transaction.IDOpportunity)
		}
		byOpportunity[*transaction.IDOpportunity] = append(byOpportunity[*transaction.IDOpportunity], transaction)
	}

	// 3. ROI del portfolio
	roi.Totals.RoiOnInvested = ratio(roi.Totals.RealizedPnL+roi.Totals.UnrealizedPnL, roi.Totals.InvestedCost)
	roi.Totals.Equity = roi.Totals.Cash + roi.Totals.MarketValue
	if roi.Totals.NetDeposits <= 0 {
		roi.Warnings = append(roi.Warnings, "Sin depositos registrados no se calcula el ROI sobre el patrimonio")
	} else if roi.Totals.Cash < -portfolioQuantityEpsilon {
		roi.Warnings = append(roi.Warnings, "Hay compras sin los depositos que las cubren, no se calcula el ROI sobre el patrimonio")
	} else {
		roi.Totals.RoiOnEquity = ratio(roi.Totals.Equity-roi.Totals.NetDeposits, roi.Totals.NetDeposits)
	}

	// 4. ROI de cada oportunidad con sus propias transacciones
	for _, idOpportunity := range opportunityOrder {
		linked := byOpportunity[idOpportunity]
		opportunityRoi := OpportunityRoi{IDOpportunity: idOpportunity, Transactions: len(linked)}
		if linked[0].IDAsset != nil {
			opportunityRoi.IDAsset = *linked[0].IDAsset
		}
		opportunityRoi.TagAsset = linked[0].TagAsset

		opportunity, visible := opportunities[idOpportunity]
		if visible {
			opportunityRoi.TagTrigger = opportunity.TagTrigger
			opportunityRoi.TagAction = opportunity.TagAction
			opportunityRoi.TagStatus = opportunity.TagStatus
			if opportunity.PriceActiveOportunity != nil && opportunity.PriceAchieved != nil {
				opportunityRoi.PlannedPriceChange = ratio(*opportunity.PriceAchieved-*opportunity.PriceActiveOportunity, *opportunity.PriceActiveOportunity)
			}
		}

		for _, transaction := range linked {
			switch transaction.TagTypeTransaction {
			case cmm.TransactionTypeBuy.Tag:
				opportunityRoi.QuantityBought += transaction.Quantity
				opportunityRoi.InvestedCost += transaction.Value + transaction.Fee
			case cmm.TransactionTypeSell.Tag:
				opportunityRoi.QuantitySold += transaction.Quantity
				opportunityRoi.Proceeds += transaction.Value - transaction.Fee
			}
		}

		position, _, err := BuildPortfolioAssetPositionService(linked)
		if !err.IsEmtpy() {
			opportunityRoi.Warning = "La oportunidad tiene ventas mayores a sus compras ligadas, su ROI no se calcula"
			roi.Opportunities = append(roi.Opportunities, opportunityRoi)
			continue
		}
		opportunityRoi.OpenQuantity = position.Quantity
		opportunityRoi.AverageCost = position.AverageCost
		opportunityRoi.RealizedPnL = position.RealizedPnL

		unrealized := 0.0
		price, hasPrice := prices[opportunityRoi.IDAsset]
		if hasPrice {
			opportunityRoi.MarketPrice = &price
		}
		if position.Quantity > 0 && !hasPrice {
			opportunityRoi.Warning = "No se obtuvo el precio del asset, su ROI no se calcula"
			roi.Opportunities = append(roi.Opportunities, opportunityRoi)
			continue
		}
		if position.Quantity > 0 {
			unrealized = position.Quantity*price - position.CostBasis
		}
		opportunityRoi.UnrealizedPnL = &unrealized
		opportunityRoi.Roi = ratio(position.RealizedPnL+unrealized, opportunityRoi.InvestedCost)
		roi.Opportunities = append(roi.Opportunities, opportunityRoi)
	}

	return roi
}
