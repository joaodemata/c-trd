package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/providers"
)

// CtrdClient cliente HTTP de la API de c-trd con la sesion del usuario agente
type CtrdClient struct {
	BaseURL string
	HTTP    *http.Client
	token   string
}

func NewCtrdClient(baseURL string) *CtrdClient {
	return &CtrdClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// apiEnvelope respuesta generica de c-trd (cmm.APIResponse) con la data sin decodificar
type apiEnvelope struct {
	Status   string          `json:"status"`
	Message  string          `json:"message"`
	Tracking string          `json:"tracking"`
	Data     json.RawMessage `json:"data"`
}

// do envia la peticion y decodifica data en out (si no es nil). Si c-trd responde FAIL el mensaje del error lleva el
// tracking, el mensaje y la data (ej: los problemas de validacion) para devolverselos al agente
func (c *CtrdClient) do(ctx context.Context, method string, path string, query url.Values, body any, out any) *cmm.ErrorHandler {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return cmm.NewErrorHandler(err, "No se pudo serializar el body", cmm.LevelFatal, "AGCLIE001")
		}
		reader = bytes.NewReader(encoded)
	}

	target := c.BaseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	request, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return cmm.NewErrorHandler(err, "No se pudo armar la peticion a c-trd", cmm.LevelFatal, "AGCLIE002")
	}
	request.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}

	response, err := c.HTTP.Do(request)
	if err != nil {
		return cmm.NewErrorHandler(err, fmt.Sprintf("c-trd no respondio %s %s: %v", method, path, err), cmm.LevelFatal, "AGCLIE003")
	}
	defer response.Body.Close()

	var envelope apiEnvelope
	if err := json.NewDecoder(io.LimitReader(response.Body, 20<<20)).Decode(&envelope); err != nil {
		return cmm.NewErrorHandler(err, fmt.Sprintf("Respuesta invalida de c-trd %s %s (HTTP %d)", method, path, response.StatusCode), cmm.LevelFatal, "AGCLIE004")
	}

	if envelope.Status != "SUCCESS" {
		message := fmt.Sprintf("%s %s: %s", envelope.Tracking, envelope.Message, string(envelope.Data))
		return cmm.NewErrorHandler(fmt.Errorf("c-trd %s", envelope.Tracking), strings.TrimSpace(message), cmm.LevelError, "AGCLIE005")
	}

	if out != nil && len(envelope.Data) > 0 {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return cmm.NewErrorHandler(err, fmt.Sprintf("No se pudo leer la data de %s %s: %v", method, path, err), cmm.LevelFatal, "AGCLIE006")
		}
	}
	return cmm.NewEmptyErrorHandler()
}

// Login inicia la sesion del usuario agente
func (c *CtrdClient) Login(ctx context.Context, username string, password string) *cmm.ErrorHandler {
	var data struct {
		Token string `json:"token"`
	}
	err := c.do(ctx, http.MethodPost, "/auth/login", nil, map[string]string{"username": username, "password": password}, &data)
	if !err.IsEmtpy() {
		return err
	}
	c.token = data.Token
	return cmm.NewEmptyErrorHandler()
}

// Logout cierra la sesion del usuario agente
func (c *CtrdClient) Logout(ctx context.Context) *cmm.ErrorHandler {
	return c.do(ctx, http.MethodPost, "/auth/logout", nil, nil, nil)
}

// RunStart corrida creada por c-trd y la tesis que debe procesar
type RunStart struct {
	IDRun  string                      `json:"idRun"`
	Thesis models.AgentThesesModelType `json:"thesis"`
}

// StartRun toma hasta 5 tesis de la cola (FIFO) y crea sus corridas
func (c *CtrdClient) StartRun(ctx context.Context) ([]RunStart, *cmm.ErrorHandler) {
	var data struct {
		Runs []RunStart `json:"runs"`
	}
	err := c.do(ctx, http.MethodPost, "/operation/agent/start_run", nil, nil, &data)
	return data.Runs, err
}

// RunStep un intento de un agente (mismo formato que fv.AgentRunStepFormat)
type RunStep struct {
	Agent    string         `json:"agent"`
	Attempt  int            `json:"attempt"`
	Output   map[string]any `json:"output"`
	Problems []string       `json:"problems"`
}

// FinishRunRequest body de PUT /operation/agent/finish_run (fv.FinishAgentRunFormat)
type FinishRunRequest struct {
	IdRun      string    `json:"idRun"`
	TagStatus  string    `json:"tagStatus"`
	Steps      []RunStep `json:"steps"`
	Conditions []string  `json:"conditions"`
	IdTrigger  string    `json:"idTrigger,omitempty"`
	Error      string    `json:"error,omitempty"`
	Requeue    bool      `json:"requeue"`
}

// FinishRun cierra la corrida
func (c *CtrdClient) FinishRun(ctx context.Context, finish FinishRunRequest) *cmm.ErrorHandler {
	return c.do(ctx, http.MethodPut, "/operation/agent/finish_run", nil, finish, nil)
}

// listAll recorre todas las paginas de un listado paginado ("page" es el total de documentos)
func listAll[T any](ctx context.Context, c *CtrdClient, path string, key string) ([]T, *cmm.ErrorHandler) {
	all := []T{}
	for page := 1; ; page++ {
		var data map[string]json.RawMessage
		query := url.Values{"limit": {"100"}, "page": {strconv.Itoa(page)}}
		if err := c.do(ctx, http.MethodGet, path, query, nil, &data); !err.IsEmtpy() {
			return all, err
		}

		var items []T
		if err := json.Unmarshal(data[key], &items); err != nil {
			return all, cmm.NewErrorHandler(err, fmt.Sprintf("No se pudo leer %s de %s", key, path), cmm.LevelFatal, "AGCLIE007")
		}
		var total int
		if err := json.Unmarshal(data["page"], &total); err != nil {
			return all, cmm.NewErrorHandler(err, fmt.Sprintf("No se pudo leer el total de %s", path), cmm.LevelFatal, "AGCLIE008")
		}

		all = append(all, items...)
		if len(items) == 0 || len(all) >= total {
			return all, cmm.NewEmptyErrorHandler()
		}
	}
}

// ListConditions todas las condiciones existentes
func (c *CtrdClient) ListConditions(ctx context.Context) ([]models.ConditionsModelType, *cmm.ErrorHandler) {
	return listAll[models.ConditionsModelType](ctx, c, "/list/conditions", "conditions")
}

// ListApis todas las APIs activas
func (c *CtrdClient) ListApis(ctx context.Context) ([]models.ApisModelType, *cmm.ErrorHandler) {
	return listAll[models.ApisModelType](ctx, c, "/list/apis", "apis")
}

// ListCallbacks callbacks que leen un formato de respuesta
func (c *CtrdClient) ListCallbacks(ctx context.Context, responseFormat string) ([]providers.CallbackInfo, *cmm.ErrorHandler) {
	var data struct {
		Callbacks []providers.CallbackInfo `json:"callbacks"`
	}
	err := c.do(ctx, http.MethodGet, "/list/callbacks", url.Values{"responseFormat": {responseFormat}}, nil, &data)
	return data.Callbacks, err
}

// ListCatalogs catalogos de un tipo
func (c *CtrdClient) ListCatalogs(ctx context.Context, typeCatalog string) ([]models.CatalogsModelType, *cmm.ErrorHandler) {
	var data struct {
		Catalogs []models.CatalogsModelType `json:"catalogs"`
	}
	err := c.do(ctx, http.MethodGet, "/list/catalogs", url.Values{"typeCatalog": {typeCatalog}}, nil, &data)
	return data.Catalogs, err
}

// CreateCondition crea una condicion (queda pendiente de revision) y retorna su id
func (c *CtrdClient) CreateCondition(ctx context.Context, body map[string]any) (string, *cmm.ErrorHandler) {
	var data struct {
		IdCondition string `json:"idCondition"`
	}
	err := c.do(ctx, http.MethodPost, "/operation/create/condition", nil, body, &data)
	return data.IdCondition, err
}

// CreateTrigger crea un trigger (queda pendiente de revision) y retorna su id
func (c *CtrdClient) CreateTrigger(ctx context.Context, body map[string]any) (string, *cmm.ErrorHandler) {
	var data struct {
		IdTrigger string `json:"idTrigger"`
	}
	err := c.do(ctx, http.MethodPost, "/operation/create/trigger", nil, body, &data)
	return data.IdTrigger, err
}
