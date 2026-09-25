package providers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	cmm "c_trd/common"
)

// HTTPRequestConfig engloba toda la configuración de la petición a ejecutar
type HTTPRequestConfig struct {
	URL          string            `bson:"url" json:"url" description:"URL base, soporta interpolación ej: https://api.com/data/{{userId}}"`
	Method       string            `bson:"method" json:"method" description:"Método HTTP (GET, POST, PUT, DELETE, etc.)"`
	Headers      map[string]string `bson:"headers,omitempty" json:"headers,omitempty" description:"Cabeceras HTTP. "`
	DataLocation string            `bson:"data_location,omitempty" json:"data_location,omitempty" description:"Por donde se envian los parametros"`

	// Usamos interface{} para poder guardar un string, un map o un array de objetos dependiendo de lo que exija la API
	QueryData map[string]string `bson:"query_data,omitempty" json:"query_data,omitempty" description:"Payload de la petición"`
	BodyData  interface{}       `bson:"body_data,omitempty" json:"body_data,omitempty" description:"Payload de la petición"`

	Auth        *HTTPAuth         `bson:"auth,omitempty" json:"auth,omitempty" description:"Configuraciones de autorización (Bearer, Basic, API Key)"`
	DynamicVars []DynamicVariable `bson:"dynamic_vars,omitempty" json:"dynamicVars,omitempty" description:"Variables dinámicas requeridas para construir esta petición"`

	TimeoutSeconds int          `bson:"timeout_seconds,omitempty" json:"timeoutSeconds,omitempty" description:"Tiempo de espera máximo en segundos"`
	RetryPolicy    *RetryPolicy `bson:"retry_policy,omitempty" json:"retryPolicy,omitempty" description:"Configuración de reintentos en caso de fallo"`
}

// HTTPAuth maneja la autenticación sin mezclarla manualmente en los headers
type HTTPAuth struct {
	Type  string `bson:"type" json:"type" description:"Tipo de Auth: 'BEARER', 'BASIC', 'API_KEY'"`
	Key   string `bson:"key,omitempty" json:"key,omitempty" description:"Nombre del key (para API_KEY en query o header)"`
	Value string `bson:"value" json:"value" description:"El token, credencial o password (puede ser una variable ej: {{secretToken}})"`
	In    string `bson:"in,omitempty" json:"in,omitempty" description:"Dónde inyectarlo: 'HEADER' o 'QUERY'"`
}

// DynamicVariable define cómo el motor (tu worker en Go) debe buscar los datos dinámicos antes de hacer la petición
type DynamicVariable struct {
	Name         string `bson:"name" json:"name" description:"Nombre de la variable (ej: 'userId') para reemplazar {{userId}}"`
	Source       string `bson:"source" json:"source" description:"De dónde sacar el dato: 'WEBHOOK_PAYLOAD', 'PREVIOUS_CONDITION', 'DATABASE'"`
	SourcePath   string `bson:"source_path" json:"sourcePath" description:"Ruta JSONPath para extraer el dato (ej: 'user.profile.id')"`
	DefaultValue string `bson:"default_value,omitempty" json:"defaultValue,omitempty" description:"Valor por defecto si no se encuentra el dato"`
	IsRequired   bool   `bson:"is_required" json:"isRequired" description:"Si es true y el dato no se encuentra, la petición aborta"`
}

// RetryPolicy define qué hacer si la API externa falla (opcional pero muy recomendado)
type RetryPolicy struct {
	MaxRetries int `bson:"max_retries" json:"maxRetries" description:"Número máximo de reintentos"`
	DelayMs    int `bson:"delay_ms" json:"delayMs" description:"Milisegundos a esperar entre reintentos"`
}

// ExecuteRequest processes the config and executes the HTTP request
func ExecuteRequest(config *HTTPRequestConfig) ([]byte, error) {
	// 1. Parse Base URL directly from config
	parsedURL, err := url.Parse(config.URL)
	if err != nil {
		return nil, cmm.NewErrorHandler(err, "error parsing the URL.", cmm.LevelFatal, "PRHTTPE001")
	}

	// 2. Build Query Params
	query := parsedURL.Query()
	for key, val := range config.QueryData {
		query.Add(key, val)
	}

	// 3. Handle Authentication if exists (in Query)
	if config.Auth != nil && strings.ToUpper(config.Auth.In) == "QUERY" {
		query.Add(config.Auth.Key, config.Auth.Value)
	}
	parsedURL.RawQuery = query.Encode()

	// 4. Process the Body (if it exists and is not a GET request)
	var bodyReader io.Reader
	if config.BodyData != nil && strings.ToUpper(config.Method) != http.MethodGet {
		jsonBody, err := json.Marshal(config.BodyData)
		if err != nil {
			return nil, cmm.NewErrorHandler(err, "error marshaling body.", cmm.LevelFatal, "PRHTTPE002")
		}
		bodyReader = bytes.NewBuffer(jsonBody)
	}

	// 5. Configure the HTTP Client with Timeout
	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Second // Default fallback
	}
	client := &http.Client{Timeout: timeout}

	// 6. Retry Policy setup
	maxRetries := 1
	delayMs := 0
	if config.RetryPolicy != nil {
		maxRetries = config.RetryPolicy.MaxRetries
		delayMs = config.RetryPolicy.DelayMs
	}
	if maxRetries < 1 {
		maxRetries = 1
	}

	var finalResponse []byte
	var finalStatusCode int
	var reqErr error

	// 7. Execution loop (with retries)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest(strings.ToUpper(config.Method), parsedURL.String(), bodyReader)
		if err != nil {
			return nil, cmm.NewErrorHandler(err, "error creating request", cmm.LevelFatal, "PRHTTPE002")
		}

		// Set default basic Headers
		req.Header.Set("Content-Type", "application/json")

		// Add Headers from Config
		for key, val := range config.Headers {
			req.Header.Set(key, val)
		}

		// Authentication via Header
		if config.Auth != nil && strings.ToUpper(config.Auth.In) == "HEADER" {
			req.Header.Set(config.Auth.Key, config.Auth.Value)
		}

		// Execute the request
		resp, err := client.Do(req)
		if err != nil {

			reqErr = cmm.NewErrorHandler(err, "Request error", cmm.LevelFatal, "PRHTTPE00")
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
			continue
		}

		finalStatusCode = resp.StatusCode
		finalResponse, _ = io.ReadAll(resp.Body)
		resp.Body.Close()

		// If it's a successful status (2xx), break the loop and return
		if finalStatusCode >= 200 && finalStatusCode < 300 {
			return finalResponse, cmm.NewEmptyErrorHandler()
		}
		// TODO: ESTO NO ME GUSTA NADA BOTTLENECK
		// If it fails, save the error and try again (if attempts remain)
		reqErr = fmt.Errorf("HTTP %d: %s", finalStatusCode, string(finalResponse))
		if attempt < maxRetries {
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
		}
	}

	return finalResponse, reqErr
}
