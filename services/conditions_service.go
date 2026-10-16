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
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// ConditionExecutionUpdate datos a guardar de una condicion luego de ser evaluada por el cronjob
type ConditionExecutionUpdate struct {
	IDCondition     primitive.ObjectID
	ExecutedAt      time.Time
	NextExecutionAt time.Time
	NewStatus       *cmm.CatalogStatus // nil si el status no cambia
}

// GetCronjobConditionsService obtiene las condiciones de tipo cronjob que deben ser evaluadas
func GetCronjobConditionsService() ([]models.ConditionsModelType, *cmm.ErrorHandler) {
	// Inicializamos el slice donde guardaremos los resultados (vacío por defecto, no nil)
	result := make([]models.ConditionsModelType, 0)

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	// Tiempo actual UTC
	currentTime := time.Now().UTC()

	// Construir el Filtro (Query). Si next_execution_at es null la condicion nunca se ha ejecutado
	filter := bson.M{
		"$or": []bson.M{
			{"next_execution_at": bson.M{"$lt": currentTime}},
			{"next_execution_at": nil},
		},
		"is_webhook_only": false,
		"is_processing":   false,
		"logical_delete":  false,
	}

	// Opciones del Query
	opts := options.Find().
		SetProjection(bson.M{
			"logical_delete": 0,
			"core_db":        0,
			"core_version":   0,
			"user_db":        0,
			"schema_db":      0,
		})

	// Ejecutar el Query
	cursor, err := models.ConditionsModel.Find(ctx, filter, opts)
	if err != nil {
		return result, cmm.NewErrorHandler(err, "Error consultando listado", cmm.LevelDatabase, "SCONE001")
	}
	// Cerramos el cursor al terminar de procesar
	defer cursor.Close(ctx)

	// Decodificar todos los resultados en el Slice
	if err = cursor.All(ctx, &result); err != nil {
		return result, cmm.NewErrorHandler(err, "Error decodificando cursor", cmm.LevelDatabase, "SCONE002")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// GetConditionsByIDsService obtiene las condiciones no borradas de los ids enviados. Las que no existen no se retornan
func GetConditionsByIDsService(ids []primitive.ObjectID) ([]models.ConditionsModelType, *cmm.ErrorHandler) {
	result := make([]models.ConditionsModelType, 0)
	if len(ids) == 0 {
		return result, cmm.NewEmptyErrorHandler()
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": bson.M{"$in": ids}, "logical_delete": false}
	opts := options.Find().SetProjection(bson.M{
		"_id":                   1,
		"id_provider":           1,
		"name_provider":         1,
		"name_condition":        1,
		"description_condition": 1,
	})

	cursor, err := models.ConditionsModel.Find(ctx, filter, opts)
	if err != nil {
		return result, cmm.NewErrorHandler(err, "Error consultando las condiciones", cmm.LevelDatabase, "SCONE008")
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &result); err != nil {
		return result, cmm.NewErrorHandler(err, "Error decodificando cursor", cmm.LevelDatabase, "SCONE009")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// LockConditionsService coloca el lock is_processing a las condiciones
func LockConditionsService(ids []primitive.ObjectID) *cmm.ErrorHandler {
	if len(ids) == 0 {
		return cmm.NewEmptyErrorHandler()
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": bson.M{"$in": ids}}
	update := bson.M{
		"$set": bson.M{
			"is_processing":   true,
			"last_updated_at": time.Now().UTC(),
		},
	}

	if _, err := models.ConditionsModel.UpdateMany(ctx, filter, update); err != nil {
		return cmm.NewErrorHandler(err, "Error bloqueando las condiciones.", cmm.LevelDatabase, "SCONE003")
	}

	return cmm.NewEmptyErrorHandler()
}

// UnlockConditionsService libera el lock is_processing de las condiciones sin modificar su programacion
func UnlockConditionsService(ids []primitive.ObjectID) *cmm.ErrorHandler {
	if len(ids) == 0 {
		return cmm.NewEmptyErrorHandler()
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	filter := bson.M{"_id": bson.M{"$in": ids}}
	update := bson.M{
		"$set": bson.M{
			"is_processing":   false,
			"last_updated_at": time.Now().UTC(),
		},
	}

	if _, err := models.ConditionsModel.UpdateMany(ctx, filter, update); err != nil {
		return cmm.NewErrorHandler(err, "Error liberando el lock de las condiciones.", cmm.LevelDatabase, "SCONE004")
	}

	return cmm.NewEmptyErrorHandler()
}

// UpdateConditionsExecutionService guarda el resultado de la evaluacion de las condiciones,
// reprograma la siguiente ejecucion y libera el lock en un solo BulkWrite.
// Recibe el ctx de la transaccion (SessionContext)
func UpdateConditionsExecutionService(ctx context.Context, updates []ConditionExecutionUpdate) *cmm.ErrorHandler {
	if len(updates) == 0 {
		return cmm.NewEmptyErrorHandler()
	}

	now := time.Now().UTC()
	writes := make([]mongo.WriteModel, 0, len(updates))

	for _, u := range updates {
		set := bson.M{
			"is_processing":     false,
			"last_executed_at":  u.ExecutedAt,
			"next_execution_at": u.NextExecutionAt,
			"last_updated_at":   now,
		}

		// Solo actualizamos el status si cambio
		if u.NewStatus != nil {
			set["id_status"] = u.NewStatus.ID
			set["tag_status"] = u.NewStatus.Tag
			set["status"] = u.NewStatus.Name
		}

		writes = append(writes, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": u.IDCondition}).
			SetUpdate(bson.M{"$set": set}))
	}

	// Unordered es más rápido ya que las operaciones son independientes
	if _, err := models.ConditionsModel.BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false)); err != nil {
		return cmm.NewErrorHandler(err, "Error actualizando la ejecucion de las condiciones.", cmm.LevelDatabase, "SCONE005")
	}

	return cmm.NewEmptyErrorHandler()
}

// SaveConditionResultsService guarda en una transaccion el status de las condiciones en las oportunidades
// y la ejecucion de las condiciones. Si algo falla se hace rollback de todo el lote.
// Requiere que MongoDB sea un replica set
func SaveConditionResultsService(changes []ConditionStatusChange, updates []ConditionExecutionUpdate) *cmm.ErrorHandler {
	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	// Iniciamos la sesion. Usamos cmm.Database porque cmm.AppConfig puede ser nil al iniciar
	session, err := cmm.Database.Client().StartSession()
	if err != nil {
		return cmm.NewErrorHandler(err, "Error iniciando la sesion de MongoDB.", cmm.LevelDatabase, "SCONE006")
	}
	defer session.EndSession(ctx)

	txnOpts := options.Transaction().SetWriteConcern(writeconcern.Majority())

	// El callback del driver debe retornar error nil para hacer COMMIT, cualquier otro valor hace ROLLBACK
	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// A. Replicamos el status en las oportunidades en curso
		if errOpp := UpdateOpportunitiesConditionsStatusService(sessCtx, changes); !errOpp.IsEmtpy() {
			return nil, errOpp
		}

		// B. Actualizamos las condiciones, reprogramamos y liberamos el lock
		if errCond := UpdateConditionsExecutionService(sessCtx, updates); !errCond.IsEmtpy() {
			return nil, errCond
		}

		return nil, nil
	}, txnOpts)
	if err != nil {
		return cmm.NewErrorHandler(err, "Transaccion abortada guardando las condiciones.", cmm.LevelDatabase, "SCONE007")
	}

	return cmm.NewEmptyErrorHandler()
}
