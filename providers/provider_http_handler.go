package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
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
	Type string `bson:"type" json:"type" description:"Tipo de Auth: 'BEARER', 'BASIC', 'API_KEY'"`
	Key  string `bson:"key,omitempty" json:"key,omitempty" description:"Nombre del key (para API_KEY en query o header)"`
	// Value se mantiene para poder decodificar documentos antiguos, pero se ignora: el secreto
	// siempre se obtiene de la variable de entorno indicada en secret_ref del proveedor
	Value string `bson:"value" json:"value" description:"Obsoleto, se ignora. El secreto viene del secret_ref del proveedor"`
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
	MaxRetries int `bson:"max_retries" json:"maxRetries" description:"Número máximo de reintentos (sin contar el primer intento)"`
	DelayMs    int `bson:"delay_ms" json:"delayMs" description:"Milisegundos a esperar entre reintentos"`
}

// RequestCredentials datos del proveedor necesarios para autenticar y validar la peticion
type RequestCredentials struct {
	SecretRef    string   // Nombre del secreto, se resuelve como JM_CTRD_SECRET_<SecretRef>
	AllowedHosts []string // Hosts a los que se permite enviar la peticion
}

// Solo se aceptan nombres de secreto en mayusculas, numeros y guion bajo
var secretRefPattern = regexp.MustCompile(`^[A-Z0-9_]+$`)

// ExecuteRequest processes the config and executes the HTTP request
func ExecuteRequest(ctx context.Context, config *HTTPRequestConfig, credentials RequestCredentials) ([]byte, *cmm.ErrorHandler) {
	method := strings.ToUpper(config.Method)

	// 1. Parse Base URL directly from config
	parsedURL, err := url.Parse(config.URL)
	if err != nil {
		return nil, cmm.NewErrorHandler(redactURLError(err), "error parsing the URL.", cmm.LevelFatal, "PRHTTPE001")
	}

	// 2. Validamos que el host pertenezca al proveedor antes de adjuntar cualquier secreto
	if err := validateHost(parsedURL, credentials.AllowedHosts, config.Auth != nil); !err.IsEmtpy() {
		return nil, err
	}

	// 3. Resolvemos el secreto y donde se inyecta
	authQuery := map[string]string{}
	authHeaders := map[string]string{}
	secret := ""
	if config.Auth != nil {
		resolved, err := resolveSecret(credentials.SecretRef)
		if !err.IsEmtpy() {
			return nil, err
		}
		secret = resolved

		authQuery, authHeaders, err = authParams(config.Auth, secret)
		if !err.IsEmtpy() {
			return nil, err
		}
	}

	// 4. Build Query Params
	query := parsedURL.Query()
	for key, val := range config.QueryData {
		query.Add(key, val)
	}
	for key, val := range authQuery {
		query.Set(key, val)
	}
	parsedURL.RawQuery = query.Encode()

	// 5. Process the Body (if it exists and is not a GET request). Se guarda en bytes para
	// crear un reader nuevo en cada intento
	var jsonBody []byte
	if config.BodyData != nil && method != http.MethodGet {
		jsonBody, err = json.Marshal(bsonToJSONValue(config.BodyData))
		if err != nil {
			return nil, cmm.NewErrorHandler(err, "error marshaling body.", cmm.LevelFatal, "PRHTTPE002")
		}
	}

	// 6. Configure the HTTP Client with Timeout. Las redirecciones a hosts fuera del proveedor no se siguen
	timeout := time.Duration(config.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = cmm.ProviderDefaultTimeout
	}
	client := &http.Client{Timeout: timeout, CheckRedirect: redirectPolicy(credentials.AllowedHosts)}

	// 7. Retry Policy setup: MaxRetries son reintentos adicionales al primer intento
	maxRetries := 0
	delay := time.Duration(0)
	if config.RetryPolicy != nil {
		maxRetries = min(max(config.RetryPolicy.MaxRetries, 0), cmm.ProviderMaxRetries)
		delay = min(time.Duration(max(config.RetryPolicy.DelayMs, 0))*time.Millisecond, cmm.ProviderMaxRetryDelay)
	}

	// 8. Execution loop (with retries)
	reqErr := cmm.NewEmptyErrorHandler()
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			if err := waitRetry(ctx, delay); !err.IsEmtpy() {
				return nil, err
			}
		}

		body, retryable, err := doRequest(ctx, client, method, parsedURL, jsonBody, config.Headers, authHeaders, secret)
		if err.IsEmtpy() {
			return body, err
		}

		reqErr = err
		if !retryable {
			break
		}
	}

	return nil, reqErr
}

// doRequest ejecuta un intento de la peticion. Retorna si el error permite reintentar
func doRequest(ctx context.Context, client *http.Client, method string, target *url.URL, jsonBody []byte, headers map[string]string, authHeaders map[string]string, secret string) ([]byte, bool, *cmm.ErrorHandler) {
	var bodyReader io.Reader
	if jsonBody != nil {
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, target.String(), bodyReader)
	if err != nil {
		return nil, false, cmm.NewErrorHandler(redactURLError(err), "error creating request", cmm.LevelFatal, "PRHTTPE005")
	}

	if jsonBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add Headers from Config
	for key, val := range headers {
		req.Header.Set(key, val)
	}

	// Authentication via Header
	for key, val := range authHeaders {
		req.Header.Set(key, val)
	}

	// Execute the request. Si el contexto termino no tiene sentido reintentar
	resp, err := client.Do(req)
	if err != nil {
		return nil, ctx.Err() == nil, cmm.NewErrorHandler(redactURLError(err), "Request error", cmm.LevelError, "PRHTTPE003")
	}
	defer resp.Body.Close()

	// Limitamos el tamaño de la respuesta para no agotar la memoria
	body, err := io.ReadAll(io.LimitReader(resp.Body, cmm.ProviderMaxResponseBytes+1))
	if err != nil {
		return nil, ctx.Err() == nil, cmm.NewErrorHandler(err, "error reading response body", cmm.LevelError, "PRHTTPE006")
	}
	if len(body) > cmm.ProviderMaxResponseBytes {
		return nil, false, cmm.NewErrorHandler(fmt.Errorf("response exceeds %d bytes", cmm.ProviderMaxResponseBytes), "response body too large", cmm.LevelError, "PRHTTPE007")
	}

	// If it's a successful status (2xx), return
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return body, false, cmm.NewEmptyErrorHandler()
	}

	// Solo se reintenta ante limite de peticiones o errores del servidor, los 4xx no cambian al reintentar
	retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500

	snippet := string(body[:min(len(body), 200)])
	if secret != "" {
		snippet = strings.ReplaceAll(snippet, secret, "<redacted>")
	}
	statusErr := fmt.Errorf("status %d: %s", resp.StatusCode, snippet)

	return nil, retryable, cmm.NewErrorHandler(statusErr, "Request returned a non-2xx status", cmm.LevelError, "PRHTTPE004")
}

// resolveSecret obtiene el secreto del proveedor desde la variable de entorno JM_CTRD_SECRET_<secretRef>
func resolveSecret(secretRef string) (string, *cmm.ErrorHandler) {
	if secretRef == "" {
		return "", cmm.NewErrorHandler(nil, "the request requires auth but the provider has no secret_ref", cmm.LevelError, "PRHTTPE008")
	}

	if !secretRefPattern.MatchString(secretRef) {
		return "", cmm.NewErrorHandler(fmt.Errorf("invalid secret_ref %q", secretRef), "secret_ref must only contain A-Z, 0-9 and _", cmm.LevelError, "PRHTTPE009")
	}

	envName := cmm.ProviderSecretEnvPrefix + secretRef
	value := os.Getenv(envName)
	if value == "" {
		return "", cmm.NewErrorHandler(fmt.Errorf("env var %s is not set", envName), "provider secret not found", cmm.LevelError, "PRHTTPE010")
	}

	return value, cmm.NewEmptyErrorHandler()
}

// validateHost verifica que la URL apunte a un host permitido del proveedor. Si la peticion lleva
// secreto el proveedor debe tener hosts configurados y la URL debe ser https
func validateHost(target *url.URL, allowedHosts []string, requiresAuth bool) *cmm.ErrorHandler {
	if requiresAuth && len(allowedHosts) == 0 {
		return cmm.NewErrorHandler(nil, "the request requires auth but the provider has no allowed_hosts", cmm.LevelError, "PRHTTPE011")
	}

	if requiresAuth && !strings.EqualFold(target.Scheme, "https") {
		return cmm.NewErrorHandler(fmt.Errorf("scheme %q", target.Scheme), "secrets can only be sent over https", cmm.LevelError, "PRHTTPE012")
	}

	if len(allowedHosts) > 0 && !isHostAllowed(target.Hostname(), allowedHosts) {
		return cmm.NewErrorHandler(fmt.Errorf("host %q", target.Hostname()), "host is not in the provider allowed_hosts", cmm.LevelError, "PRHTTPE013")
	}

	return cmm.NewEmptyErrorHandler()
}

// authParams calcula los query params y headers a inyectar segun el tipo de autenticacion
func authParams(auth *HTTPAuth, secret string) (map[string]string, map[string]string, *cmm.ErrorHandler) {
	query := map[string]string{}
	headers := map[string]string{}

	switch strings.ToUpper(auth.Type) {
	case "API_KEY":
		if auth.Key == "" {
			return query, headers, cmm.NewErrorHandler(nil, "API_KEY auth requires a key", cmm.LevelError, "PRHTTPE014")
		}
		switch strings.ToUpper(auth.In) {
		case "QUERY":
			query[auth.Key] = secret
		case "HEADER":
			headers[auth.Key] = secret
		default:
			return query, headers, cmm.NewErrorHandler(fmt.Errorf("in %q", auth.In), "API_KEY auth must be in QUERY or HEADER", cmm.LevelError, "PRHTTPE015")
		}
	case "BEARER":
		headers["Authorization"] = "Bearer " + secret
	case "BASIC":
		// El secreto debe tener el formato usuario:password
		if !strings.Contains(secret, ":") {
			return query, headers, cmm.NewErrorHandler(nil, "BASIC auth secret must be user:password", cmm.LevelError, "PRHTTPE016")
		}
		headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(secret))
	default:
		return query, headers, cmm.NewErrorHandler(fmt.Errorf("type %q", auth.Type), "unsupported auth type", cmm.LevelError, "PRHTTPE017")
	}

	return query, headers, cmm.NewEmptyErrorHandler()
}

// waitRetry espera el delay entre reintentos o termina si el contexto se cancela
func waitRetry(ctx context.Context, delay time.Duration) *cmm.ErrorHandler {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return cmm.NewErrorHandler(ctx.Err(), "deadline reached before retrying the request", cmm.LevelError, "PRHTTPE018")
	case <-timer.C:
		return cmm.NewEmptyErrorHandler()
	}
}

// redirectPolicy no sigue redirecciones hacia hosts fuera del proveedor para no filtrar headers de auth.
// En ese caso se retorna la respuesta 3xx, que se trata como un status no exitoso
func redirectPolicy(allowedHosts []string) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		if len(allowedHosts) > 0 && !isHostAllowed(req.URL.Hostname(), allowedHosts) {
			return http.ErrUseLastResponse
		}
		return nil
	}
}

// isHostAllowed compara el host contra la lista de hosts permitidos (sin distinguir mayusculas)
func isHostAllowed(host string, allowedHosts []string) bool {
	for _, allowed := range allowedHosts {
		if strings.EqualFold(host, allowed) {
			return true
		}
	}
	return false
}

// redactURLError quita el query string de la URL de un *url.Error para no registrar secretos en los logs
func redactURLError(err error) error {
	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return err
	}

	redacted := urlErr.URL
	if i := strings.IndexByte(redacted, '?'); i >= 0 {
		redacted = redacted[:i] + "?<redacted>"
	}

	return &url.Error{Op: urlErr.Op, URL: redacted, Err: urlErr.Err}
}

// bsonToJSONValue convierte los documentos que decodifica Mongo en un interface{} (primitive.D y primitive.A)
// a map y slice, porque json.Marshal serializa un primitive.D como [{"Key":..,"Value":..}] y no como un objeto
func bsonToJSONValue(value interface{}) interface{} {
	switch v := value.(type) {
	case primitive.D:
		object := make(map[string]interface{}, len(v))
		for _, element := range v {
			object[element.Key] = bsonToJSONValue(element.Value)
		}
		return object
	case primitive.M:
		object := make(map[string]interface{}, len(v))
		for key, element := range v {
			object[key] = bsonToJSONValue(element)
		}
		return object
	case primitive.A:
		list := make([]interface{}, len(v))
		for i, element := range v {
			list[i] = bsonToJSONValue(element)
		}
		return list
	default:
		return value
	}
}
