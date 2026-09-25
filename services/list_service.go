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

	// Set timeout context for the query
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create filter and variable to hold the result
	// TODO: agregar el idStatus al query
	filter := bson.M{"_id": idTriggerParsed, "logical_delete": false}

	// Extraemos la data que nos importa
	opts := options.FindOne().SetProjection(bson.M{
		"pine_code":       0,
		"last_updated_at": 0,
		"created_date":    0,
		"logical_delete":  0,
		"core_db":         0,
		"core_version":    0,
		"user_db":         0,
		"schema_db":       0,
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

// GetActiveOpportunity busca si hay una oportunidad activa de algun idTrigger
func GetActiveOpportunity(idTrigger primitive.ObjectID) (models.OpportunitiesModelType, *cmm.ErrorHandler) {
	var result models.OpportunitiesModelType

	// 2. Crear contexto con timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 3. Crear filtro: idTrigger, status activo y que no esté borrado lógicamente
	filter := bson.M{
		"id_trigger":     idTrigger,
		"tag_status":     "OPPORTUNITY-ONGOING",
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
