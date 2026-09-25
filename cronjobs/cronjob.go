package cronjobs

import (
	"context"
	"fmt"
	"time"
)

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
				// Llamamos con una go rutine el servicio de actualizar condiciones y verificar si la oportunidad pasa a activa
			}
		}
	}()
}
