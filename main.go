package main

import (
	"c_trd/cronjobs"
	"c_trd/routers"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 1. Creamos un Context que puede ser cancelado
	ctxCronjob, cancelCronjob := context.WithCancel(context.Background())
	defer cancelCronjob() // Por seguridad, nos aseguramos de limpiarlo al final


	fmt.Println("Initializing cronjobs...")
	cronjobs.InitCronjobs(ctxCronjob)


	fmt.Println("Initializing server...")
	// 1. Configurar el servidor HTTP
	mux := http.NewServeMux()
	
	// 2. Routes 
	routers.SetupRouter(mux)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 5. Graceful Shutdown (Apagado seguro)
	// Esto intercepta señales del sistema (como Ctrl+C) para cerrar conexiones limpiamente
	go func() {
		fmt.Printf("Servidor escuchando en http://localhost%s\n", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error iniciando servidor: %v", err)
		}
	}()

	// Canal para escuchar señales de interrupción
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit // El programa se bloquea aquí hasta recibir la señal

	fmt.Println("\nApagando servidor...")
	
	cancelCronjob()

	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTimeout()

	if err := srv.Shutdown(ctxTimeout); err != nil {
		log.Fatalf("Error forzando el apagado: %v", err)
	}

	fmt.Println("Servidor detenido correctamente.")
}