package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	cmm "c_trd/common"
)

// LLM ejecuta un agente: system prompt + entrada JSON -> texto de la respuesta. Un error significa que no se pudo
// llamar al modelo (ej: se acabo el uso del plan) y la corrida debe detenerse
type LLM interface {
	Complete(ctx context.Context, systemPrompt string, input string) (string, *cmm.ErrorHandler)
}

// ClaudeCLI usa Claude Code en modo headless (claude -p) con la sesion del plan ya iniciada en el servidor.
// El agente no tiene herramientas: todo el contexto viene en la entrada y el runner hace las llamadas a c-trd
type ClaudeCLI struct {
	Bin     string        // ruta del binario, por defecto "claude"
	Model   string        // opcional, ej: "opus"; vacio usa el del plan
	Timeout time.Duration // por llamada
}

// Herramientas que se le quitan al agente, solo debe razonar y responder JSON
const claudeDisallowedTools = "Bash,Edit,Write,MultiEdit,NotebookEdit,WebFetch,WebSearch,Task,Read,Glob,Grep"

// claudeResult sobre de la salida de "claude -p --output-format json"
type claudeResult struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
}

func (c ClaudeCLI) Complete(ctx context.Context, systemPrompt string, input string) (string, *cmm.ErrorHandler) {
	bin := c.Bin
	if bin == "" {
		bin = "claude"
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	// Carpeta vacia para que el agente no vea ningun archivo del servidor
	workDir, err := os.MkdirTemp("", "ctrd-agent-")
	if err != nil {
		return "", cmm.NewErrorHandler(err, "No se pudo crear la carpeta temporal del agente", cmm.LevelFatal, "AGCLAE001")
	}
	defer os.RemoveAll(workDir)

	args := []string{
		"-p",
		"--output-format", "json",
		"--system-prompt", systemPrompt,
		"--disallowedTools", claudeDisallowedTools,
		"--max-turns", "3",
	}
	if c.Model != "" {
		args = append(args, "--model", c.Model)
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(callCtx, bin, args...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := fmt.Sprintf("Fallo claude -p: %v. stderr: %s stdout: %s", err, truncate(stderr.String(), 1000), truncate(stdout.String(), 1000))
		return "", cmm.NewErrorHandler(err, message, cmm.LevelFatal, "AGCLAE002")
	}

	var result claudeResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return "", cmm.NewErrorHandler(err, "La salida de claude -p no es el JSON esperado: "+truncate(stdout.String(), 1000), cmm.LevelFatal, "AGCLAE003")
	}
	if result.IsError {
		message := fmt.Sprintf("claude -p respondio con error (%s): %s", result.Subtype, truncate(result.Result, 1000))
		return "", cmm.NewErrorHandler(fmt.Errorf("claude error %s", result.Subtype), message, cmm.LevelFatal, "AGCLAE004")
	}

	return result.Result, cmm.NewEmptyErrorHandler()
}

func truncate(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
