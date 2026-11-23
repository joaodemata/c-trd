// backtester ejecuta las corridas de backtest que la API dejo en cola (POST /operation/backtest/create_runs). Es un
// proceso aparte del servidor HTTP y del cronjob: lo lanza cada minuto un cron del VPS dentro del contenedor de la API
// de cada entorno (mismas variables de entorno, misma red con Mongo y la misma carpeta de backtests). Cada ejecucion
// marca fallidas las corridas abandonadas, procesa hasta -max-runs corridas y termina.
//
// Variables de entorno: las mismas de la API (JM_CTRD_MDB_*, JM_CTRD_SECRET_* de los proveedores y
// JM_CTRD_BACKTESTS_DIR). En Windows se leen del .env como en main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"runtime"
	"time"

	"c_trd/backtest"
	cmm "c_trd/common"
	"c_trd/models"

	"github.com/joho/godotenv"
)

func main() {
	maxRuns := flag.Int("max-runs", cmm.BacktestRunsPerInvocation, "corridas a procesar en esta ejecucion")
	flag.Parse()
	if *maxRuns < 1 {
		log.Fatal("[BACKTEST] -max-runs debe ser al menos 1")
	}

	// Igual que main.go: el .env solo se carga en Windows
	if runtime.GOOS == "windows" {
		if err := godotenv.Load(); err != nil {
			log.Fatalf("[BACKTEST] Error cargando el .env: %v", err)
		}
	}

	db := cmm.ConnectDB(models.RegistryList)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.Client().Disconnect(ctx); err != nil {
			log.Printf("[BACKTEST] Error desconectando MongoDB: %v", err)
		}
	}()

	processed := backtest.ProcessQueuedBacktests(*maxRuns)
	fmt.Printf("[BACKTEST] Corridas procesadas: %d\n", processed)
}
