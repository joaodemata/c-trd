package cronjobs

import (
	"c_trd/models"
	"c_trd/providers"
	"context"
	"fmt"
	"log"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Validamos el status de las condiciones que son de tipo cronjob y actualizamos todas las oportunidades ligadas a las respectivas condiciones
func GetCronjobConditionsService() ([]models.ConditionsModelType, *cmm.ErrorHandler) {
	// Inicializamos el slice donde guardaremos los resultados (vacío por defecto, no nil)
	result := make([]models.ConditionsModelType, 0)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second) // Subimos un poco el timeout para listados
	defer cancel()

	// Tiempo actual (PC) UTC

	currentTime := time.Now().UTC()

	// Construir el Filtro (Query)
	filter := bson.M{
		"next_execution_at": bson.M{
			"$lt": currentTime,
		},
		"is_webhook_only": false,
		"is_processing":   false,
		"logical_delete":  false,
	}

	//  Opciones del Query
	opts := options.Find().
		SetProjection(bson.M{
			// "metadata": 0, // Descomenta si quieres excluir metadatos pesados en el listado
			"logical_delete": 0,
			"core_db":        0,
			"core_version":   0,
			"user_db":        0,
			"schema_db":      0,
		})

	// 4. Ejecutar el Query (Usar OpportunityModel en vez de TriggersModel)
	cursor, err := models.ConditionsModel.Find(ctx, filter, opts)
	if err != nil {
		return result, cmm.NewErrorHandler(err, "Error consultando listado", cmm.LevelDatabase, "CRCCE001")
	}
	// Cerramos el cursor al terminar de procesar
	defer cursor.Close(ctx)

	// 5. Decodificar todos los resultados en el Slice
	if err = cursor.All(ctx, &result); err != nil {
		return result, cmm.NewErrorHandler(err, "Error decodificando cursor", cmm.LevelDatabase, "CRCCE002")
	}

	return result, cmm.NewEmptyErrorHandler()
}

// ProcessConditions iterates over a slice of conditions, executes their HTTP requests,
// and dynamically calls the corresponding callback function.
// TODO: Colocar workers/go functions etc
func ProcessConditions(conditions []models.ConditionsModelType) {
	// Iteramos
	for _, condition := range conditions {

		fmt.Printf("Processing condition: %s (ID: %s)\n", condition.NameCondition, condition.ID.Hex())

		// 1. Validate if there is an HTTP config to execute
		if condition.HttpRequest.URL == "" {
			log.Printf("Condition %s has no RequestConfig, skipping...", condition.ID.Hex())
			continue
		}

		// 2. Execute the HTTP request
		responseBody, err := providers.ExecuteRequest(&condition.HttpRequest)
		if err != nil {
			log.Printf("Error executing HTTP request for condition %s: %v\n", condition.ID.Hex(), err)
			// Decide if you want to continue to the next condition or stop
			continue
		}

		// 3. Dynamic function execution
		callbackName := condition.CallbackFunction
		if callbackName == "" {
			log.Printf("Condition %s executed successfully (HTTP %d), but no CallbackFunction was defined.\n", condition.ID.Hex())
			continue
		}

		// Look up the function in the registry
		callback, exists := providers.FunctionRegistry[callbackName]
		if !exists {
			log.Printf("Warning: Callback function '%s' not found in FunctionRegistry for condition %s\n", callbackName, condition.ID.Hex())
			continue
		}

		// 4. Execute the dynamic callback
		fmt.Printf("Invoking callback: %s\n", callbackName)
		conditionAccepted, err := callback(responseBody)
		if !err.IsEmtpy() {
			log.Printf("Error executing callback '%s' for condition %s: %v\n", callbackName, condition.ID.Hex(), err)
		}

		// Si la condicion esta aceptada y tiene un status diferente
		// TODO: descomentar y mejorar
		if conditionAccepted && condition.TagStatus != "STATUS-ACTIVE" {
			mongo.NewUpdateOneModel()
			// 	// --- NUEVO: Crear un contexto con un timeout de seguridad (ej: 10 segundos) ---
			// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			// 	defer cancel() // Libera los recursos del contexto al salir de la función

			// 	// 1. Iniciar la sesión de MongoDB
			// 	session, err := cmm.AppConfig.MongoClient.StartSession()
			// 	if err != nil {
			// 		log.Printf("Error iniciando sesión de MongoDB: %v", err)
			// 		continue
			// 	}
			// 	defer session.EndSession(ctx) // Ahora ctx ya existe

			// 	// 2. Definir opciones (Write Concern "majority")
			// 	txnOpts := options.Transaction().SetWriteConcern(writeconcern.Majority())

			// 	// 3. Ejecutar bloque transaccional
			// 	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
			// 		// A. Actualizamos la condicion
			// 		errCond := services.UpdateCondition(sessCtx, condition.ID, )
			// 		if errCond != nil {
			// 			return nil, errCond // Dispara el ROLLBACK
			// 		}

			// 		// B. Actualizamos las oportunidades
			// 		_, errOpp := services.UpdateOpportunitiesStatus(sessCtx /* tus parámetros */)
			// 		if errOpp != nil {
			// 			return nil, errOpp // Dispara el ROLLBACK
			// 		}

			// 		return nil, nil // Dispara el COMMIT
			// 	}, txnOpts)
			// 	// 4. Validar resultado
			// 	if err != nil {
			// 		log.Printf("Transacción abortada: %v", err)
			// 		continue
			// 	}

			// 	log.Println("Transacción ejecutada correctamente.")
		}
	}
	return
}

// TODO: referencia para bulkwrite en mongodb utilizando go functions (Mejor eficiencia)
// func processLargeArray(ctx context.Context, data []MyDataType, coll1, coll2 *mongo.Collection) error {
// 	batchSize := 500
// 	maxWorkers := 20 // Límite de concurrencia basado en tu maxPoolSize de MongoDB, no en la CPU

// 	// Semáforo para limitar la cantidad de goroutines simultáneas
// 	sem := make(chan struct{}, maxWorkers)
// 	var wg sync.WaitGroup
// 	errChan := make(chan error, len(data)/batchSize+1)

// 	for i := 0; i < len(data); i += batchSize {
// 		end := i + batchSize
// 		if end > len(data) {
// 			end = len(data)
// 		}

// 		batch := data[i:end]
// 		wg.Add(1)

// 		go func(b []MyDataType) {
// 			defer wg.Done()

// 			sem <- struct{}{} // Adquirir token
// 			defer func() { <-sem }() // Liberar token

// 			// Preparar modelos para BulkWrite
// 			var modelsColl1 []mongo.WriteModel
// 			var modelsColl2 []mongo.WriteModel

// 			for _, item := range b {
// 				// Crear tus UpdateOneModel para ambas colecciones
// 				update1 := mongo.NewUpdateOneModel().SetFilter(...).SetUpdate(...)
// 				update2 := mongo.NewUpdateOneModel().SetFilter(...).SetUpdate(...)

// 				modelsColl1 = append(modelsColl1, update1)
// 				modelsColl2 = append(modelsColl2, update2)
// 			}

// 			// Ejecutar BulkWrite (unordered es más rápido si las operaciones son independientes)
// 			bulkOpts := options.BulkWrite().SetOrdered(false)

// 			_, err := coll1.BulkWrite(ctx, modelsColl1, bulkOpts)
// 			if err != nil {
// 				errChan <- fmt.Errorf("error coll1 batch: %w", err)
// 				return
// 			}

// 			_, err = coll2.BulkWrite(ctx, modelsColl2, bulkOpts)
// 			if err != nil {
// 				errChan <- fmt.Errorf("error coll2 batch: %w", err)
// 				return
// 			}

// 		}(batch)
// 	}

// 	wg.Wait()
// 	close(errChan)

// 	// Manejo de errores
// 	for err := range errChan {
// 		if err != nil {
// 			return err // O hacer log y continuar, según tu lógica de negocio
// 		}
// 	}

// 	return nil
// }
