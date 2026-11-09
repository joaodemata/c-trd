// agent_runner ejecuta los agentes de c-trd sobre la cola de tesis. Lo lanza el cron del VPS los jueves en la noche,
// antes de que se reinicie el uso semanal del plan de Claude (viernes 1am hora local). Usa Claude Code en modo headless (claude -p), que debe tener la sesion iniciada
// en el servidor, y la API de c-trd con el usuario agente (claude).
//
// Variables de entorno:
//
//	CTRD_AGENT_API_URL       requerida, ej: https://api.dev.example.com
//	CTRD_AGENT_PASSWORD      requerida, contraseña del usuario agente
//	CTRD_AGENT_USERNAME      opcional, por defecto "claude"
//	CTRD_AGENT_CLAUDE_BIN    opcional, por defecto "claude"
//	CTRD_AGENT_MODEL         opcional, modelo de claude -p (vacio usa el del plan)
//	CTRD_AGENT_MAX_ATTEMPTS  opcional, intentos por agente, por defecto 3
package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"c_trd/agents"
)

func main() {
	apiURL := os.Getenv("CTRD_AGENT_API_URL")
	password := os.Getenv("CTRD_AGENT_PASSWORD")
	if apiURL == "" || password == "" {
		log.Fatal("[AGENTS] CTRD_AGENT_API_URL y CTRD_AGENT_PASSWORD son requeridas")
	}

	username := envOrDefault("CTRD_AGENT_USERNAME", "claude")
	maxAttempts, convErr := strconv.Atoi(envOrDefault("CTRD_AGENT_MAX_ATTEMPTS", "3"))
	if convErr != nil || maxAttempts < 1 || maxAttempts > 5 {
		log.Fatal("[AGENTS] CTRD_AGENT_MAX_ATTEMPTS debe ser un numero entre 1 y 5")
	}

	// Limite total de la corrida: 5 tesis con sus reintentos
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	client := agents.NewCtrdClient(apiURL)
	if err := client.Login(ctx, username, password); !err.IsEmtpy() {
		log.Fatalf("[AGENTS] Login de %s fallo: %s", username, err.Message)
	}

	runner := agents.Runner{
		Client:      client,
		LLM:         agents.ClaudeCLI{Bin: envOrDefault("CTRD_AGENT_CLAUDE_BIN", "claude"), Model: os.Getenv("CTRD_AGENT_MODEL")},
		MaxAttempts: maxAttempts,
	}

	err := runner.Run(ctx)
	if errLogout := client.Logout(context.Background()); !errLogout.IsEmtpy() {
		log.Printf("[AGENTS] Logout fallo: %s", errLogout.Message)
	}
	if !err.IsEmtpy() {
		log.Fatalf("[AGENTS] Corrida detenida (%s): %s", err.TrackingCode, err.Message)
	}
	log.Println("[AGENTS] Corrida terminada")
}

func envOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
