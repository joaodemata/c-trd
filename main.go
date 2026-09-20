package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	cmm "c_trd/common"
	"c_trd/cronjobs"
	"c_trd/models"
	"c_trd/routers"

	"github.com/joho/godotenv"
)

func main() {
	// Cargamos las variables de entorno solo si es windows 
	if runtime.GOOS == "windows" {
		
		err := godotenv.Load()	

		// Validamos si hay  error
		if err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}


	// 1. Initialize MongoDB
	fmt.Println("Connecting to MongoDB...")
	db := cmm.ConnectDB(models.RegistryList)
	mongoClient := db.Client()

	// 2. Setup context for cronjobs
	ctxCronjob, cancelCronjob := context.WithCancel(context.Background())
	defer cancelCronjob() 

	fmt.Println("Initializing cronjobs...")
	cronjobs.InitCronjobs(ctxCronjob)

	// 3. Setup HTTP server
	fmt.Println("Initializing server...")
	mux := http.NewServeMux()

	// Start web socket  
	hub := cmm.NewHub()
	routers.SetupRouter(mux, hub)

	
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 4. Start HTTP server in a goroutine
	go func() {
		fmt.Printf("Server listening on http://localhost%s\n", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
		}()
		
	// Start websocket
	go hub.Run()

	// 5. Wait for system interrupt signals (Ctrl+C)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit 

	fmt.Println("\nShutting down server...")
	
	// 6. Stop cronjobs
	cancelCronjob()

	// 7. Graceful shutdown timeout (5 seconds)
	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()

	// 8. Shutdown HTTP server
	if err := srv.Shutdown(ctxTimeout); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	// 9. Disconnect MongoDB
	fmt.Println("Disconnecting from MongoDB...")
	if err := mongoClient.Disconnect(ctxTimeout); err != nil {
		log.Printf("MongoDB disconnect error: %v", err)
	}

	fmt.Println("Server stopped successfully.")
}
