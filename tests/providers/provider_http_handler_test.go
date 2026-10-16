package providers_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	cmm "c_trd/common"
	"c_trd/providers"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestExecuteRequest_HostValidation(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	t.Cleanup(server.Close)

	serverURL, _ := url.Parse(server.URL)
	bearer := &providers.HTTPAuth{Type: "BEARER"}

	tests := []struct {
		name         string
		rawURL       string
		auth         *providers.HTTPAuth
		allowedHosts []string
		wantTracking string // vacio si la peticion debe salir
	}{
		{name: "sin auth ni hosts permitidos", rawURL: server.URL},
		{name: "host permitido sin distinguir mayusculas", rawURL: "http://LOCALHOST:" + serverURL.Port(), allowedHosts: []string{"localhost"}},
		{name: "auth sin hosts configurados", rawURL: "https://api.example.com/x", auth: bearer, wantTracking: "PRHTTPE011"},
		{name: "auth nunca se envia por http", rawURL: server.URL, auth: bearer, allowedHosts: []string{"127.0.0.1"}, wantTracking: "PRHTTPE012"},
		{name: "host fuera de la lista", rawURL: "https://evil.example.com/x", allowedHosts: []string{"api.example.com"}, wantTracking: "PRHTTPE013"},
		{name: "subdominio no permitido", rawURL: "https://api.example.com.evil.io/x", allowedHosts: []string{"api.example.com"}, wantTracking: "PRHTTPE013"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := &providers.HTTPRequestConfig{URL: tt.rawURL, Method: http.MethodGet, Auth: tt.auth}
			credentials := providers.RequestCredentials{SecretRef: "TEST_TOKEN", AllowedHosts: tt.allowedHosts}

			_, err := providers.ExecuteRequest(context.Background(), config, credentials)

			if tt.wantTracking == "" {
				testutil.RequireEmptyError(t, err)
				return
			}
			testutil.RequireTrackingCode(t, err, tt.wantTracking)
		})
	}

	// Solo los dos casos validos deben llegar al servidor
	t.Cleanup(func() {
		if got := hits.Load(); got != 2 {
			t.Errorf("el servidor recibio %d peticiones, se esperaban 2", got)
		}
	})
}

// capturedRequest datos de la peticion que recibio el servidor de prueba
type capturedRequest struct {
	header http.Header
	query  url.Values
}

// TestExecuteRequest_Auth no es paralela: usa t.Setenv para los secretos y cambia http.DefaultTransport
func TestExecuteRequest_Auth(t *testing.T) {
	t.Setenv(cmm.ProviderSecretEnvPrefix+"TEST_TOKEN", "s3cr3t")
	t.Setenv(cmm.ProviderSecretEnvPrefix+"TEST_BASIC", "user:pass")

	received := make(chan capturedRequest, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/unauthorized" {
			// Algunas APIs devuelven la key recibida en el mensaje de error
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, "invalid api key s3cr3t")
			return
		}
		received <- capturedRequest{header: r.Header.Clone(), query: r.URL.Query()}
	}))
	t.Cleanup(server.Close)

	allowed := []string{"127.0.0.1"}

	t.Run("el secreto no aparece en errores de conexion", func(t *testing.T) {
		// Sin confiar en el certificado del servidor la peticion falla en el handshake TLS,
		// y el error del cliente incluye la URL con el query string
		config := &providers.HTTPRequestConfig{URL: server.URL, Method: http.MethodGet, Auth: &providers.HTTPAuth{Type: "API_KEY", Key: "apikey", In: "QUERY"}}

		_, err := providers.ExecuteRequest(context.Background(), config, providers.RequestCredentials{SecretRef: "TEST_TOKEN", AllowedHosts: allowed})

		testutil.RequireTrackingCode(t, err, "PRHTTPE003")
		if strings.Contains(err.Error(), "s3cr3t") {
			t.Errorf("el error contiene el secreto: %v", err)
		}
		if !strings.Contains(err.Error(), "?<redacted>") {
			t.Errorf("el error deberia conservar la URL sin el query: %v", err)
		}
	})

	useTrustedTransport(t, server)

	successTests := []struct {
		name      string
		auth      providers.HTTPAuth
		secretRef string
		check     func(t *testing.T, got capturedRequest)
	}{
		{
			name:      "API_KEY en query",
			auth:      providers.HTTPAuth{Type: "api_key", Key: "apikey", In: "query"},
			secretRef: "TEST_TOKEN",
			check: func(t *testing.T, got capturedRequest) {
				if got.query.Get("apikey") != "s3cr3t" {
					t.Errorf("query = %v, se esperaba apikey=s3cr3t", got.query)
				}
			},
		},
		{
			name:      "API_KEY en header",
			auth:      providers.HTTPAuth{Type: "API_KEY", Key: "X-Api-Key", In: "HEADER"},
			secretRef: "TEST_TOKEN",
			check: func(t *testing.T, got capturedRequest) {
				if got.header.Get("X-Api-Key") != "s3cr3t" {
					t.Errorf("X-Api-Key = %q, se esperaba s3cr3t", got.header.Get("X-Api-Key"))
				}
			},
		},
		{
			name:      "BEARER",
			auth:      providers.HTTPAuth{Type: "BEARER"},
			secretRef: "TEST_TOKEN",
			check: func(t *testing.T, got capturedRequest) {
				if got.header.Get("Authorization") != "Bearer s3cr3t" {
					t.Errorf("Authorization = %q, se esperaba Bearer s3cr3t", got.header.Get("Authorization"))
				}
			},
		},
		{
			name:      "BASIC",
			auth:      providers.HTTPAuth{Type: "BASIC"},
			secretRef: "TEST_BASIC",
			check: func(t *testing.T, got capturedRequest) {
				if got.header.Get("Authorization") != "Basic dXNlcjpwYXNz" {
					t.Errorf("Authorization = %q, se esperaba Basic dXNlcjpwYXNz", got.header.Get("Authorization"))
				}
			},
		},
	}

	for _, tt := range successTests {
		t.Run(tt.name, func(t *testing.T) {
			config := &providers.HTTPRequestConfig{URL: server.URL, Method: http.MethodGet, Auth: &tt.auth}

			_, err := providers.ExecuteRequest(context.Background(), config, providers.RequestCredentials{SecretRef: tt.secretRef, AllowedHosts: allowed})

			testutil.RequireEmptyError(t, err)
			tt.check(t, <-received)
		})
	}

	errorTests := []struct {
		name         string
		auth         providers.HTTPAuth
		secretRef    string
		wantTracking string
	}{
		{"proveedor sin secret_ref", providers.HTTPAuth{Type: "BEARER"}, "", "PRHTTPE008"},
		// Evita leer variables arbitrarias del entorno, ej: PATH
		{"secret_ref con caracteres no permitidos", providers.HTTPAuth{Type: "BEARER"}, "test_token", "PRHTTPE009"},
		{"variable de entorno inexistente", providers.HTTPAuth{Type: "BEARER"}, "TEST_DOES_NOT_EXIST", "PRHTTPE010"},
		{"API_KEY sin key", providers.HTTPAuth{Type: "API_KEY", In: "QUERY"}, "TEST_TOKEN", "PRHTTPE014"},
		{"API_KEY con ubicacion invalida", providers.HTTPAuth{Type: "API_KEY", Key: "k", In: "BODY"}, "TEST_TOKEN", "PRHTTPE015"},
		{"BASIC sin usuario:password", providers.HTTPAuth{Type: "BASIC"}, "TEST_TOKEN", "PRHTTPE016"},
		{"tipo no soportado", providers.HTTPAuth{Type: "OAUTH"}, "TEST_TOKEN", "PRHTTPE017"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			config := &providers.HTTPRequestConfig{URL: server.URL, Method: http.MethodGet, Auth: &tt.auth}

			_, err := providers.ExecuteRequest(context.Background(), config, providers.RequestCredentials{SecretRef: tt.secretRef, AllowedHosts: allowed})

			testutil.RequireTrackingCode(t, err, tt.wantTracking)
		})
	}

	t.Run("el secreto no aparece en el body de una respuesta de error", func(t *testing.T) {
		config := &providers.HTTPRequestConfig{URL: server.URL + "/unauthorized", Method: http.MethodGet, Auth: &providers.HTTPAuth{Type: "BEARER"}}

		_, err := providers.ExecuteRequest(context.Background(), config, providers.RequestCredentials{SecretRef: "TEST_TOKEN", AllowedHosts: allowed})

		testutil.RequireTrackingCode(t, err, "PRHTTPE004")
		if strings.Contains(err.Error(), "s3cr3t") {
			t.Errorf("el error contiene el secreto: %v", err)
		}
		if !strings.Contains(err.Error(), "<redacted>") {
			t.Errorf("el error deberia indicar que se oculto el secreto: %v", err)
		}
	})
}

// useTrustedTransport hace que http.DefaultTransport confie en el certificado del servidor TLS de prueba.
// ExecuteRequest crea su propio http.Client sin Transport, por eso usa http.DefaultTransport.
// Solo se puede usar en pruebas que no son paralelas
func useTrustedTransport(t *testing.T, server *httptest.Server) {
	t.Helper()

	original := http.DefaultTransport
	http.DefaultTransport = server.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = original })
}

func TestExecuteRequest_SendsQueryParams(t *testing.T) {
	t.Parallel()

	received := make(chan url.Values, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.URL.Query()
		io.WriteString(w, `{"ok":true}`)
	}))
	t.Cleanup(server.Close)

	config := &providers.HTTPRequestConfig{
		URL:       server.URL + "/data?interval=daily",
		Method:    "get",
		QueryData: map[string]string{"symbol": "SPY"},
	}

	body, err := providers.ExecuteRequest(context.Background(), config, providers.RequestCredentials{})

	testutil.RequireEmptyError(t, err)
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %s", body)
	}

	query := <-received
	if query.Get("interval") != "daily" || query.Get("symbol") != "SPY" {
		t.Errorf("query = %v, se esperaba interval=daily y symbol=SPY", query)
	}
}

func TestExecuteRequest_SendsBsonBodyAsJSONObject(t *testing.T) {
	t.Parallel()

	type receivedRequest struct {
		contentType string
		body        string
	}
	received := make(chan receivedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- receivedRequest{contentType: r.Header.Get("Content-Type"), body: string(body)}
	}))
	t.Cleanup(server.Close)

	// Asi decodifica Mongo un body_data guardado como objeto
	config := &providers.HTTPRequestConfig{
		URL:    server.URL,
		Method: http.MethodPost,
		BodyData: primitive.D{
			{Key: "symbol", Value: "SPY"},
			{Key: "filters", Value: primitive.A{primitive.D{{Key: "field", Value: "volume"}}}},
			{Key: "meta", Value: primitive.M{"source": "cron"}},
		},
	}

	_, err := providers.ExecuteRequest(context.Background(), config, providers.RequestCredentials{})
	testutil.RequireEmptyError(t, err)

	got := <-received
	if got.contentType != "application/json" {
		t.Errorf("Content-Type = %q, se esperaba application/json", got.contentType)
	}

	want := `{"filters":[{"field":"volume"}],"meta":{"source":"cron"},"symbol":"SPY"}`
	if got.body != want {
		t.Errorf("body = %s, se esperaba %s", got.body, want)
	}
	if !json.Valid([]byte(got.body)) {
		t.Errorf("el body no es JSON valido: %s", got.body)
	}
}

func TestExecuteRequest_RetryPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		statuses     []int // status por intento, el ultimo se repite
		maxRetries   int
		wantAttempts int32
		wantTracking string // vacio si la peticion termina bien
	}{
		{name: "reintenta 5xx hasta tener exito", statuses: []int{503, 500, 200}, maxRetries: 3, wantAttempts: 3},
		{name: "reintenta 429", statuses: []int{429, 200}, maxRetries: 1, wantAttempts: 2},
		{name: "agota los reintentos", statuses: []int{500}, maxRetries: 2, wantAttempts: 3, wantTracking: "PRHTTPE004"},
		{name: "no reintenta 4xx", statuses: []int{404}, maxRetries: 3, wantAttempts: 1, wantTracking: "PRHTTPE004"},
		{name: "sin politica no reintenta", statuses: []int{503}, maxRetries: 0, wantAttempts: 1, wantTracking: "PRHTTPE004"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempt := int(attempts.Add(1))
				w.WriteHeader(tt.statuses[min(attempt, len(tt.statuses))-1])
			}))
			t.Cleanup(server.Close)

			config := &providers.HTTPRequestConfig{
				URL:         server.URL,
				Method:      http.MethodGet,
				RetryPolicy: &providers.RetryPolicy{MaxRetries: tt.maxRetries, DelayMs: 1},
			}

			_, err := providers.ExecuteRequest(context.Background(), config, providers.RequestCredentials{})

			if tt.wantTracking == "" {
				testutil.RequireEmptyError(t, err)
			} else {
				testutil.RequireTrackingCode(t, err, tt.wantTracking)
			}
			if got := attempts.Load(); got != tt.wantAttempts {
				t.Errorf("intentos = %d, se esperaban %d", got, tt.wantAttempts)
			}
		})
	}
}

func TestExecuteRequest_DoesNotFollowRedirectOutsideAllowedHosts(t *testing.T) {
	t.Parallel()

	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHits.Add(1)
	}))
	t.Cleanup(target.Close)

	targetURL, _ := url.Parse(target.URL)
	// Mismo servidor pero con otro hostname, que no esta en allowed_hosts
	redirectTo := "http://localhost:" + targetURL.Port() + "/"

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTo, http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	config := &providers.HTTPRequestConfig{URL: origin.URL, Method: http.MethodGet}
	_, err := providers.ExecuteRequest(context.Background(), config, providers.RequestCredentials{AllowedHosts: []string{"127.0.0.1"}})

	testutil.RequireTrackingCode(t, err, "PRHTTPE004")
	if targetHits.Load() != 0 {
		t.Error("se siguio la redireccion hacia un host no permitido")
	}
}

func TestExecuteRequest_RejectsTooLargeResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, cmm.ProviderMaxResponseBytes+1))
	}))
	t.Cleanup(server.Close)

	config := &providers.HTTPRequestConfig{URL: server.URL, Method: http.MethodGet}
	_, err := providers.ExecuteRequest(context.Background(), config, providers.RequestCredentials{})

	testutil.RequireTrackingCode(t, err, "PRHTTPE007")
}

func TestExecuteRequest_CanceledContext(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Con el contexto cancelado no se reintenta aunque la politica lo permita
	config := &providers.HTTPRequestConfig{URL: server.URL, Method: http.MethodGet, RetryPolicy: &providers.RetryPolicy{MaxRetries: 5, DelayMs: 1}}
	_, err := providers.ExecuteRequest(ctx, config, providers.RequestCredentials{})

	testutil.RequireTrackingCode(t, err, "PRHTTPE003")
	if hits.Load() != 0 {
		t.Errorf("el servidor recibio %d peticiones, se esperaban 0", hits.Load())
	}
}

func TestValidateRequestTarget(t *testing.T) {
	t.Parallel()

	apiKey := &providers.HTTPAuth{Type: "API_KEY", Key: "apikey", In: "QUERY"}
	hosts := []string{"www.alphavantage.co"}
	valid := providers.RequestCredentials{SecretRef: "ALPHAVANTAGE_APIKEY", AllowedHosts: hosts}

	tests := []struct {
		name         string
		url          string
		auth         *providers.HTTPAuth
		credentials  providers.RequestCredentials
		wantTracking string // vacio si es valida
	}{
		{name: "con auth valida", url: "https://www.alphavantage.co/query", auth: apiKey, credentials: valid},
		{name: "sin auth ni hosts", url: "http://localhost:8080/series"},
		{name: "url invalida", url: "https://www.alpha vantage.co/%zz", wantTracking: "PRHTTPE019"},
		{name: "auth sin hosts", url: "https://www.alphavantage.co/query", auth: apiKey, credentials: providers.RequestCredentials{SecretRef: "X"}, wantTracking: "PRHTTPE011"},
		{name: "auth sin https", url: "http://www.alphavantage.co/query", auth: apiKey, credentials: valid, wantTracking: "PRHTTPE012"},
		{name: "host no permitido", url: "https://evil.com/query", credentials: valid, wantTracking: "PRHTTPE013"},
		{name: "auth sin secret_ref", url: "https://www.alphavantage.co/query", auth: apiKey, credentials: providers.RequestCredentials{AllowedHosts: hosts}, wantTracking: "PRHTTPE020"},
		{name: "secret_ref invalido", url: "https://www.alphavantage.co/query", auth: apiKey, credentials: providers.RequestCredentials{SecretRef: "../path", AllowedHosts: hosts}, wantTracking: "PRHTTPE020"},
		{name: "api key sin in", url: "https://www.alphavantage.co/query", auth: &providers.HTTPAuth{Type: "API_KEY", Key: "apikey"}, credentials: valid, wantTracking: "PRHTTPE015"},
		{name: "tipo de auth desconocido", url: "https://www.alphavantage.co/query", auth: &providers.HTTPAuth{Type: "OAUTH"}, credentials: valid, wantTracking: "PRHTTPE017"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := providers.ValidateRequestTarget(&providers.HTTPRequestConfig{URL: tt.url, Auth: tt.auth}, tt.credentials)
			if tt.wantTracking == "" {
				testutil.RequireEmptyError(t, err)
				return
			}
			testutil.RequireTrackingCode(t, err, tt.wantTracking)
		})
	}
}
