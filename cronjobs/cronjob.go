package cronjobs

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

// Evita que dos ejecuciones de ProcessConditions se solapen si una tarda mas de un tick
var isCheckingConditions atomic.Bool

// Evita que dos ejecuciones de ProcessExpiredOpportunities se solapen
var isCheckingOpportunities atomic.Bool

// StartTicker ahora recibe un ctx (Context)
func InitCronjobs(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		fmt.Println("Starting native cronjob...")

		// Bucle infinito manejado con 'select'
		for {
			select {
			// Caso 1: El context recibe la orden de cancelación desde main.go
			case <-ctx.Done():
				fmt.Println("stopping cronjob...")
				return // Esto saca a la goroutine del bucle y la termina

			// Caso 2: El ticker hace "tick" (pasa 1 minuto)
			case <-ticker.C:
				fmt.Println("Corriendo tarea")

				// _ ,err := services.FetchDataService("BTC", "USD")
				// Llamamos con una go rutine el servicio de actualizar oportunidades expiradas
				go runCheckOpportunities()
				// Llamamos con una go rutine el servicio de actualizar condiciones y verificar si la oportunidad pasa a activa
				go runCheckConditions()
			}
		}
	}()
}

// runCheckConditions ejecuta ProcessConditions si no hay otra ejecucion en curso
func runCheckConditions() {
	if !isCheckingConditions.CompareAndSwap(false, true) {
		log.Println("ProcessConditions sigue en ejecucion, se omite este tick")
		return
	}
	defer isCheckingConditions.Store(false)

	ProcessConditions()
}

// runCheckOpportunities ejecuta ProcessExpiredOpportunities si no hay otra ejecucion en curso
func runCheckOpportunities() {
	if !isCheckingOpportunities.CompareAndSwap(false, true) {
		log.Println("ProcessExpiredOpportunities sigue en ejecucion, se omite este tick")
		return
	}
	defer isCheckingOpportunities.Store(false)

	ProcessExpiredOpportunities()
}
