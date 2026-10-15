package cronjobs

import (
	"c_trd/models"
	"c_trd/providers"
	"c_trd/services"
	"log"
	"sync"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// conditionResult resultado de evaluar una condicion
type conditionResult struct {
	condition  models.ConditionsModelType
	executedAt time.Time
	evaluated  bool // true si la API y el callback respondieron correctamente
	accepted   bool
}

// ProcessConditions evalua las condiciones de tipo cronjob, actualiza su status, lo replica en las
// oportunidades en curso y pasa a ACTIVE las oportunidades con todas sus condiciones activas
func ProcessConditions() {
	// 1. Obtenemos las condiciones a evaluar
	conditions, err := services.GetCronjobConditionsService()
	if !err.IsEmtpy() {
		log.Printf("Error obteniendo las condiciones: %v", err)
		return
	}

	if len(conditions) > 0 {
		// 2. Bloqueamos las condiciones para que no se procesen dos veces
		ids := make([]primitive.ObjectID, len(conditions))
		for i, condition := range conditions {
			ids[i] = condition.ID
		}

		if err := services.LockConditionsService(ids); !err.IsEmtpy() {
			log.Printf("Error bloqueando las condiciones: %v", err)
			return
		}

		// 3. Evaluamos las condiciones con workers
		results := evaluateConditions(conditions)

		// 4. Guardamos los resultados en lotes con BulkWrite
		persistConditionResults(results)
	}

	// 5. Activamos las oportunidades con todas sus condiciones activas. Se ejecuta siempre para
	// recuperar oportunidades que quedaron pendientes por un fallo en una ejecucion anterior
	activated, err := services.ActivateCompletedOpportunitiesService()
	if !err.IsEmtpy() {
		log.Printf("Error activando las oportunidades: %v", err)
		return
	}

	log.Printf("Condiciones procesadas: %d, oportunidades activadas: %d", len(conditions), activated)
}

// evaluateConditions ejecuta las peticiones y callbacks de las condiciones con un pool de workers
func evaluateConditions(conditions []models.ConditionsModelType) []conditionResult {
	results := make([]conditionResult, len(conditions))
	jobs := make(chan int)
	var wg sync.WaitGroup

	workers := min(cmm.ConditionEvaluationWorkers, len(conditions))
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Cada worker escribe en un indice distinto, no hace falta mutex
			for i := range jobs {
				results[i] = evaluateCondition(conditions[i])
			}
		}()
	}

	for i := range conditions {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return results
}

// evaluateCondition ejecuta la peticion HTTP de la condicion y su callback
func evaluateCondition(condition models.ConditionsModelType) (result conditionResult) {
	result = conditionResult{condition: condition, executedAt: time.Now().UTC()}

	// Un panic en un callback no debe tumbar el cronjob ni dejar la condicion bloqueada
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic evaluando la condicion %s: %v", condition.ID.Hex(), r)
			result.evaluated = false
		}
	}()

	// 1. Validamos que exista la peticion
	if condition.HttpRequest.URL == "" {
		log.Printf("Condition %s has no RequestConfig, skipping...", condition.ID.Hex())
		return result
	}

	// 2. Buscamos el callback en el registro
	callbackName := condition.CallbackFunction
	callback, exists := providers.FunctionRegistry[callbackName]
	if !exists {
		log.Printf("Warning: Callback function '%s' not found in FunctionRegistry for condition %s", callbackName, condition.ID.Hex())
		return result
	}

	// 3. Ejecutamos la peticion HTTP
	responseBody, err := providers.ExecuteRequest(&condition.HttpRequest)
	if !err.IsEmtpy() {
		log.Printf("Error executing HTTP request for condition %s: %v", condition.ID.Hex(), err)
		return result
	}

	// 4. Ejecutamos el callback
	accepted, err := callback(responseBody)
	if !err.IsEmtpy() {
		log.Printf("Error executing callback '%s' for condition %s: %v", callbackName, condition.ID.Hex(), err)
		return result
	}

	result.evaluated = true
	result.accepted = accepted
	return result
}

// persistConditionResults guarda los resultados en lotes concurrentes limitados por un semaforo
func persistConditionResults(results []conditionResult) {
	sem := make(chan struct{}, cmm.BulkWriteMaxWorkers)
	var wg sync.WaitGroup

	for i := 0; i < len(results); i += cmm.BulkWriteBatchSize {
		end := min(i+cmm.BulkWriteBatchSize, len(results))

		wg.Add(1)
		go func(batch []conditionResult) {
			defer wg.Done()

			sem <- struct{}{}        // Adquirir token
			defer func() { <-sem }() // Liberar token

			persistConditionBatch(batch)
		}(results[i:end])
	}

	wg.Wait()
}

// persistConditionBatch guarda el lote en una transaccion (oportunidades y condiciones).
// Si la transaccion falla se hace rollback y solo se libera el lock, sin tocar next_execution_at,
// asi la siguiente ejecucion del cronjob vuelve a evaluar las condiciones del lote
func persistConditionBatch(batch []conditionResult) {
	changes := make([]services.ConditionStatusChange, 0)
	updates := make([]services.ConditionExecutionUpdate, len(batch))

	for i, r := range batch {
		newStatus := newConditionStatus(r)
		if newStatus != nil {
			changes = append(changes, services.ConditionStatusChange{IDCondition: r.condition.ID, NewStatus: *newStatus})
		}

		updates[i] = services.ConditionExecutionUpdate{
			IDCondition:     r.condition.ID,
			ExecutedAt:      r.executedAt,
			NextExecutionAt: r.executedAt.Add(conditionInterval(r.condition)),
			NewStatus:       newStatus,
		}
	}

	if err := services.SaveConditionResultsService(changes, updates); !err.IsEmtpy() {
		log.Printf("Error guardando el lote de condiciones, se reintentara en el siguiente ciclo: %v", err)

		// Liberamos el lock para que las condiciones no queden bloqueadas
		ids := make([]primitive.ObjectID, len(batch))
		for i, r := range batch {
			ids[i] = r.condition.ID
		}
		if err := services.UnlockConditionsService(ids); !err.IsEmtpy() {
			log.Printf("Error liberando el lock de las condiciones: %v", err)
		}
	}
}

// newConditionStatus retorna el nuevo status de la condicion o nil si no cambia o no se pudo evaluar
func newConditionStatus(r conditionResult) *cmm.CatalogStatus {
	if !r.evaluated {
		return nil
	}

	target := cmm.GenericStatusInactive
	if r.accepted {
		target = cmm.GenericStatusActive
	}

	if r.condition.TagStatus == target.Tag {
		return nil
	}

	return &target
}

// conditionInterval retorna el intervalo de ejecucion de la condicion
func conditionInterval(condition models.ConditionsModelType) time.Duration {
	seconds := cmm.DefaultConditionIntervalSeconds
	if condition.IntervalSeconds != nil && *condition.IntervalSeconds > 0 {
		seconds = *condition.IntervalSeconds
	}

	return time.Duration(seconds) * time.Second
}
