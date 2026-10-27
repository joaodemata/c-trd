package cronjobs

import (
	"c_trd/models"
	"c_trd/providers"
	"c_trd/services"
	"context"
	"log"
	"sync"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ConditionResult resultado de evaluar una condicion
type ConditionResult struct {
	Condition  models.ConditionsModelType
	ExecutedAt time.Time
	Evaluated  bool // true si la API y el callback respondieron correctamente
	Accepted   bool
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
		// 2. Obtenemos los proveedores de las condiciones (secreto y hosts permitidos) antes de bloquear,
		// asi un fallo aqui no deja condiciones bloqueadas
		providerIDs := make([]primitive.ObjectID, len(conditions))
		for i, condition := range conditions {
			providerIDs[i] = condition.IDProvider
		}

		providersByID, err := services.GetProvidersByIDsService(providerIDs)
		if !err.IsEmtpy() {
			log.Printf("Error obteniendo los proveedores: %v", err)
			return
		}

		// 3. Bloqueamos las condiciones para que no se procesen dos veces
		ids := make([]primitive.ObjectID, len(conditions))
		for i, condition := range conditions {
			ids[i] = condition.ID
		}

		if err := services.LockConditionsService(ids); !err.IsEmtpy() {
			log.Printf("Error bloqueando las condiciones: %v", err)
			return
		}

		// 4. Evaluamos las condiciones con workers, con un tiempo maximo para todo el ciclo
		ctx, cancel := context.WithTimeout(context.Background(), cmm.ConditionEvaluationTimeout)
		results := EvaluateConditions(ctx, conditions, providersByID)
		cancel()

		// 5. Guardamos los resultados en lotes con BulkWrite
		persistConditionResults(results)
	}

	// 6. Corregimos las copias de las condiciones en las oportunidades en curso. Una oportunidad creada por el
	// hook mientras se guardaba un lote puede nacer con un status viejo que el paso 5 ya no le replica
	syncOpportunitiesConditions()

	// 7. Activamos las oportunidades con todas sus condiciones activas. Se ejecuta siempre para
	// recuperar oportunidades que quedaron pendientes por un fallo en una ejecucion anterior
	activated := activateCompletedOpportunities()

	log.Printf("Condiciones procesadas: %d, oportunidades activadas: %d", len(conditions), activated)
}

// activateCompletedOpportunities pasa a ACTIVE las oportunidades con todas sus condiciones activas y guarda el precio
// de mercado de su asset. El precio se consulta una vez por asset; si falla la oportunidad se activa igual sin precio
func activateCompletedOpportunities() int64 {
	now := time.Now().UTC()

	opportunities, err := services.GetCompletedOpportunitiesService(now)
	if !err.IsEmtpy() {
		log.Printf("Error obteniendo las oportunidades por activar: %v", err)
		return 0
	}

	if len(opportunities) == 0 {
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.ActivationPricesTimeout)
	defer cancel()

	prices := make(map[primitive.ObjectID]*float64)
	activations := make([]services.OpportunityActivation, 0, len(opportunities))
	for _, opportunity := range opportunities {
		price, fetched := prices[opportunity.IDAsset]
		if !fetched {
			price = assetMarketPrice(ctx, opportunity.IDAsset)
			prices[opportunity.IDAsset] = price
		}

		activations = append(activations, services.OpportunityActivation{IDOpportunity: opportunity.ID, PriceActiveOpportunity: price})
	}

	activated, err := services.ActivateOpportunitiesService(activations, time.Now().UTC())
	if !err.IsEmtpy() {
		log.Printf("Error activando las oportunidades: %v", err)
		return 0
	}

	return activated
}

// assetMarketPrice retorna el precio de mercado del asset o nil si no se pudo obtener
func assetMarketPrice(ctx context.Context, idAsset primitive.ObjectID) *float64 {
	price, err := services.GetAssetMarketPriceService(ctx, idAsset)
	if !err.IsEmtpy() {
		log.Printf("No se obtuvo el precio del asset %s, la oportunidad se actualiza sin precio: %v", idAsset.Hex(), err)
		return nil
	}

	return &price
}

// EvaluateConditions ejecuta las peticiones y callbacks de las condiciones con un pool de workers
func EvaluateConditions(ctx context.Context, conditions []models.ConditionsModelType, providersByID map[primitive.ObjectID]models.ProvidersModelType) []ConditionResult {
	results := make([]ConditionResult, len(conditions))
	jobs := make(chan int)
	var wg sync.WaitGroup

	workers := min(cmm.ConditionEvaluationWorkers, len(conditions))
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Cada worker escribe en un indice distinto, no hace falta mutex
			for i := range jobs {
				results[i] = EvaluateCondition(ctx, conditions[i], providersByID)
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

// EvaluateCondition ejecuta la peticion HTTP de la condicion y su callback
func EvaluateCondition(ctx context.Context, condition models.ConditionsModelType, providersByID map[primitive.ObjectID]models.ProvidersModelType) (result ConditionResult) {
	result = ConditionResult{Condition: condition, ExecutedAt: time.Now().UTC()}

	// Un panic en un callback no debe tumbar el cronjob ni dejar la condicion bloqueada
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic evaluando la condicion %s: %v", condition.ID.Hex(), r)
			result.Evaluated = false
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

	// 3. Ejecutamos la peticion HTTP con las credenciales del proveedor. Si el proveedor no existe
	// las credenciales quedan vacias y una peticion con auth falla sin enviar nada
	provider := providersByID[condition.IDProvider]
	credentials := providers.RequestCredentials{SecretRef: provider.SecretRef, AllowedHosts: provider.AllowedHosts}

	responseBody, err := providers.ExecuteRequest(ctx, &condition.HttpRequest, credentials)
	if !err.IsEmtpy() {
		log.Printf("Error executing HTTP request for condition %s: %v", condition.ID.Hex(), err)
		return result
	}

	// 4. Ejecutamos el callback
	accepted, err := callback.Fn(responseBody, condition.ParametersCallback)
	if !err.IsEmtpy() {
		log.Printf("Error executing callback '%s' for condition %s: %v", callbackName, condition.ID.Hex(), err)
		return result
	}

	result.Evaluated = true
	result.Accepted = accepted
	return result
}

// persistConditionResults guarda los resultados en lotes concurrentes limitados por un semaforo
func persistConditionResults(results []ConditionResult) {
	sem := make(chan struct{}, cmm.BulkWriteMaxWorkers)
	var wg sync.WaitGroup

	for i := 0; i < len(results); i += cmm.BulkWriteBatchSize {
		end := min(i+cmm.BulkWriteBatchSize, len(results))

		wg.Add(1)
		go func(batch []ConditionResult) {
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
func persistConditionBatch(batch []ConditionResult) {
	changes := make([]services.ConditionStatusChange, 0)
	updates := make([]services.ConditionExecutionUpdate, len(batch))

	for i, r := range batch {
		newStatus := NewConditionStatus(r)
		if newStatus != nil {
			changes = append(changes, services.ConditionStatusChange{IDCondition: r.Condition.ID, NewStatus: *newStatus})
		}

		updates[i] = services.ConditionExecutionUpdate{
			IDCondition:     r.Condition.ID,
			ExecutedAt:      r.ExecutedAt,
			NextExecutionAt: r.ExecutedAt.Add(ConditionInterval(r.Condition)),
			NewStatus:       newStatus,
		}
	}

	if err := services.SaveConditionResultsService(changes, updates); !err.IsEmtpy() {
		log.Printf("Error guardando el lote de condiciones, se reintentara en el siguiente ciclo: %v", err)

		// Liberamos el lock para que las condiciones no queden bloqueadas
		ids := make([]primitive.ObjectID, len(batch))
		for i, r := range batch {
			ids[i] = r.Condition.ID
		}
		if err := services.UnlockConditionsService(ids); !err.IsEmtpy() {
			log.Printf("Error liberando el lock de las condiciones: %v", err)
		}
	}
}

// NewConditionStatus retorna el nuevo status de la condicion o nil si no cambia o no se pudo evaluar
func NewConditionStatus(r ConditionResult) *cmm.CatalogStatus {
	if !r.Evaluated {
		return nil
	}

	target := cmm.GenericStatusInactive
	if r.Accepted {
		target = cmm.GenericStatusActive
	}

	if r.Condition.TagStatus == target.Tag {
		return nil
	}

	return &target
}

// syncOpportunitiesConditions replica el status real de las condiciones en las oportunidades en curso que tengan
// una copia distinta. Solo el cronjob cambia el status de las condiciones y sus ejecuciones no se solapan, asi
// que el status leido aqui no cambia antes de escribirlo. Si falla se reintenta en el siguiente ciclo
func syncOpportunitiesConditions() {
	opportunities, err := services.GetOngoingOpportunitiesConditionsService()
	if !err.IsEmtpy() {
		log.Printf("Error obteniendo las oportunidades en curso: %v", err)
		return
	}

	idConditions := make([]primitive.ObjectID, 0)
	for _, opportunity := range opportunities {
		for _, condition := range opportunity.Conditions {
			idConditions = append(idConditions, condition.IDCondition)
		}
	}

	conditions, err := services.GetConditionsByIDsService(idConditions)
	if !err.IsEmtpy() {
		log.Printf("Error obteniendo las condiciones de las oportunidades en curso: %v", err)
		return
	}

	changes := OutOfSyncConditions(opportunities, conditions)
	if len(changes) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmm.DBTimeout)
	defer cancel()

	if err := services.UpdateOpportunitiesConditionsStatusService(ctx, changes); !err.IsEmtpy() {
		log.Printf("Error sincronizando las condiciones de las oportunidades: %v", err)
		return
	}

	log.Printf("Condiciones sincronizadas en oportunidades en curso: %d", len(changes))
}

// OutOfSyncConditions retorna el status real de cada condicion que tiene una copia distinta en alguna oportunidad.
// Las condiciones que ya no existen (borradas) se ignoran. Cada condicion aparece una sola vez
func OutOfSyncConditions(opportunities []models.OpportunitiesModelType, conditions []models.ConditionsModelType) []services.ConditionStatusChange {
	current := make(map[primitive.ObjectID]models.ConditionsModelType, len(conditions))
	for _, condition := range conditions {
		current[condition.ID] = condition
	}

	changes := make([]services.ConditionStatusChange, 0)
	added := make(map[primitive.ObjectID]bool)
	for _, opportunity := range opportunities {
		for _, copied := range opportunity.Conditions {
			condition, exists := current[copied.IDCondition]
			if !exists || added[condition.ID] || copied.TagStatus == condition.TagStatus {
				continue
			}

			added[condition.ID] = true
			changes = append(changes, services.ConditionStatusChange{
				IDCondition: condition.ID,
				NewStatus:   cmm.CatalogStatus{ID: condition.IDStatus, Tag: condition.TagStatus, Name: condition.Status},
			})
		}
	}

	return changes
}

// ConditionInterval retorna el intervalo de ejecucion de la condicion
func ConditionInterval(condition models.ConditionsModelType) time.Duration {
	seconds := cmm.DefaultConditionIntervalSeconds
	if condition.IntervalSeconds != nil && *condition.IntervalSeconds > 0 {
		seconds = *condition.IntervalSeconds
	}

	return time.Duration(seconds) * time.Second
}
