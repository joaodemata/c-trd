package services_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/providers"
	"c_trd/services"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func pointValues(points []providers.GoogleTrendsFilePoint) []float64 {
	values := make([]float64, len(points))
	for i, point := range points {
		values[i] = point.Value
	}
	return values
}

func TestParseGoogleTrendsCSVService_WeeklyExport(t *testing.T) {
	t.Parallel()

	content := testutil.ReadFixture(t, "google_trends_weekly.csv")
	// La ultima semana (27 sep) termina el 4 de octubre, antes de la subida
	uploadedAt := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

	series, err := services.ParseGoogleTrendsCSVService(content, uploadedAt)

	testutil.RequireEmptyError(t, err)
	if series.Keyword != "bitcoin" {
		t.Errorf("keyword = %q, se esperaba bitcoin", series.Keyword)
	}
	if series.PeriodSeconds != int64(7*24*time.Hour/time.Second) {
		t.Errorf("periodSeconds = %d, se esperaba una semana", series.PeriodSeconds)
	}
	// "<1" se interpreta como 0
	if want := []float64{40, 0, 75, 100}; !slices.Equal(pointValues(series.Points), want) {
		t.Errorf("valores = %v, se esperaba %v", pointValues(series.Points), want)
	}
}

func TestParseGoogleTrendsCSVService_DropsIncompleteLastPeriod(t *testing.T) {
	t.Parallel()

	content := testutil.ReadFixture(t, "google_trends_weekly.csv")
	// El 1 de octubre la semana del 27 de septiembre aun no termina
	uploadedAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	series, err := services.ParseGoogleTrendsCSVService(content, uploadedAt)

	testutil.RequireEmptyError(t, err)
	if want := []float64{40, 0, 75}; !slices.Equal(pointValues(series.Points), want) {
		t.Errorf("valores = %v, se esperaba %v", pointValues(series.Points), want)
	}
}

func TestParseGoogleTrendsCSVService_Formats(t *testing.T) {
	t.Parallel()

	uploadedAt := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		content    string
		wantPeriod time.Duration
		wantPoints int
	}{
		{"exportacion con BOM", "\xef\xbb\xbfDay,eth: (Worldwide)\n2026-09-01,1\n2026-09-02,2\n2026-09-03,3\n", 24 * time.Hour, 3},
		{"exportacion por hora", "Time,eth: (Worldwide)\n2026-09-01T10,1\n2026-09-01T11,2\n", time.Hour, 2},
		{"exportacion mensual", "Month,eth: (Worldwide)\n2026-07,1\n2026-08,2\n2026-09,3\n", 31 * 24 * time.Hour, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			series, err := services.ParseGoogleTrendsCSVService([]byte(tt.content), uploadedAt)

			testutil.RequireEmptyError(t, err)
			if series.Keyword != "eth" {
				t.Errorf("keyword = %q, se esperaba eth", series.Keyword)
			}
			if got := time.Duration(series.PeriodSeconds) * time.Second; got != tt.wantPeriod {
				t.Errorf("periodo = %v, se esperaba %v", got, tt.wantPeriod)
			}
			if len(series.Points) != tt.wantPoints {
				t.Errorf("puntos = %d, se esperaban %d", len(series.Points), tt.wantPoints)
			}
		})
	}
}

func TestParseGoogleTrendsCSVService_Errors(t *testing.T) {
	t.Parallel()

	uploadedAt := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		content      string
		wantTracking string
	}{
		{"filas despues de los datos", "Week,a: (W)\n2026-09-06,1\n2026-09-13,2\ntotal,3\n", "SOPRE007"},
		{"comparacion de varias busquedas", "Week,a: (W),b: (W)\n2026-09-06,1,2\n", "SOPRE008"},
		{"fechas desordenadas", "Week,a: (W)\n2026-09-13,1\n2026-09-06,2\n", "SOPRE009"},
		{"fechas repetidas", "Week,a: (W)\n2026-09-06,1\n2026-09-06,2\n", "SOPRE009"},
		{"una sola fecha", "Week,a: (W)\n2026-09-06,1\n", "SOPRE010"},
		{"sin datos", "Category: All categories\n", "SOPRE010"},
		{"valor no numerico", "Week,a: (W)\n2026-09-06,abc\n", "SOPRE019"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := services.ParseGoogleTrendsCSVService([]byte(tt.content), uploadedAt)
			testutil.RequireTrackingCode(t, err, tt.wantTracking)
		})
	}

	t.Run("menos de 2 periodos completos", func(t *testing.T) {
		t.Parallel()

		// La segunda semana aun no termina, queda un solo periodo completo
		content := "Week,a: (W)\n2026-09-06,1\n2026-09-13,2\n"
		_, err := services.ParseGoogleTrendsCSVService([]byte(content), time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
		testutil.RequireTrackingCode(t, err, "SOPRE011")
	})
}

// TestGoogleTrendsFileService no es paralela porque t.Setenv cambia la carpeta de los CSV
func TestGoogleTrendsFileService(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(cmm.GoogleTrendsDirEnv, dir)

	t.Run("guarda y lee el CSV del asset", func(t *testing.T) {
		idAsset := primitive.NewObjectID()

		testutil.RequireEmptyError(t, services.SaveGoogleTrendsFileService(idAsset, testutil.ReadFixture(t, "google_trends_weekly.csv")))

		series, err := services.GetGoogleTrendsFileService(idAsset)
		testutil.RequireEmptyError(t, err)
		if series.IDAsset != idAsset.Hex() {
			t.Errorf("idAsset = %s, se esperaba %s", series.IDAsset, idAsset.Hex())
		}
		if len(series.Points) != 4 {
			t.Errorf("puntos = %d, se esperaban 4", len(series.Points))
		}
	})

	t.Run("reemplaza el CSV anterior sin dejar temporales", func(t *testing.T) {
		idAsset := primitive.NewObjectID()
		newContent := []byte("Day,eth: (Worldwide)\n2026-09-01,1\n2026-09-02,2\n2026-09-03,3\n")

		testutil.RequireEmptyError(t, services.SaveGoogleTrendsFileService(idAsset, testutil.ReadFixture(t, "google_trends_weekly.csv")))
		testutil.RequireEmptyError(t, services.SaveGoogleTrendsFileService(idAsset, newContent))

		series, err := services.GetGoogleTrendsFileService(idAsset)
		testutil.RequireEmptyError(t, err)
		if series.Keyword != "eth" {
			t.Errorf("keyword = %q, se esperaba el CSV nuevo (eth)", series.Keyword)
		}

		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			t.Fatalf("ReadDir: %v", readErr)
		}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) != ".csv" {
				t.Errorf("archivo inesperado en la carpeta: %s", entry.Name())
			}
		}
	})

	t.Run("asset sin CSV", func(t *testing.T) {
		_, err := services.GetGoogleTrendsFileService(primitive.NewObjectID())
		testutil.RequireTrackingCode(t, err, "SOPRE016")
	})
}

func ptr[T any](v T) *T { return &v }

// newTestApi API de query con data fija y un parametro de cada tipo
func newTestApi() models.ApisModelType {
	return models.ApisModelType{
		Method:         "get",
		URL:            "https://www.alphavantage.co/query",
		DataLocation:   cmm.ApiDataLocationQuery,
		Headers:        map[string]string{"Accept": "application/json"},
		Data:           primitive.D{{Key: "function", Value: "REALTIME_PUT_CALL_RATIO"}, {Key: "version", Value: int32(2)}},
		Auth:           &providers.HTTPAuth{Type: "API_KEY", Key: "apikey", In: "QUERY", Value: "viejo-secreto"},
		RetryPolicy:    &providers.RetryPolicy{MaxRetries: 2, DelayMs: 500},
		TimeoutSeconds: 15,
		Parameters: []models.ApiParameter{
			{Name: "symbol", DataType: cmm.ApiParameterTypeString, IsRequired: true},
			{Name: "limit", DataType: cmm.ApiParameterTypeInteger, DefaultValue: ptr("10")},
			{Name: "ratio", DataType: cmm.ApiParameterTypeFloat},
			{Name: "adjusted", DataType: cmm.ApiParameterTypeBoolean},
			{Name: "from", DataType: cmm.ApiParameterTypeDate},
			{Name: "interval", DataType: cmm.ApiParameterTypeString, Options: &models.ApiParameterOptions{
				Source: cmm.ApiParameterOptionsStatic,
				Values: []models.ApiParameterOption{{Value: "daily", Label: "Diario"}, {Value: "weekly", Label: "Semanal"}},
			}},
			{Name: "exchange", DataType: cmm.ApiParameterTypeString, Options: &models.ApiParameterOptions{Source: cmm.ApiParameterOptionsApi}},
		},
	}
}

func TestBuildConditionHTTPRequestService_Query(t *testing.T) {
	t.Parallel()

	api := newTestApi()
	config, err := services.BuildConditionHTTPRequestService(api, map[string]string{
		"symbol": "AAPL", "ratio": "1.5", "adjusted": "true", "from": "2026-01-31", "interval": "weekly", "exchange": "NYSE",
	})
	testutil.RequireEmptyError(t, err)

	// La data fija, los parametros enviados y los valores por defecto van por query
	want := map[string]string{
		"function": "REALTIME_PUT_CALL_RATIO", "version": "2",
		"symbol": "AAPL", "limit": "10", "ratio": "1.5", "adjusted": "true", "from": "2026-01-31", "interval": "weekly", "exchange": "NYSE",
	}
	if !reflect.DeepEqual(config.QueryData, want) {
		t.Errorf("query_data = %v, want %v", config.QueryData, want)
	}
	if config.BodyData != nil {
		t.Errorf("body_data = %v, una API QUERY no envia body", config.BodyData)
	}
	if config.Method != "GET" || config.URL != api.URL || config.DataLocation != cmm.ApiDataLocationQuery || config.TimeoutSeconds != 15 {
		t.Errorf("config = %+v, se esperaba la configuracion de la API", config)
	}

	// El auth y los reintentos se copian, nunca con el secreto y sin compartir memoria con la API
	if config.Auth == nil || config.Auth.Key != "apikey" || config.Auth.Value != "" || config.Auth == api.Auth {
		t.Errorf("auth = %+v, se esperaba una copia sin secreto", config.Auth)
	}
	if config.RetryPolicy == nil || *config.RetryPolicy != *api.RetryPolicy || config.RetryPolicy == api.RetryPolicy {
		t.Errorf("retry_policy = %+v, se esperaba una copia de %+v", config.RetryPolicy, api.RetryPolicy)
	}
	config.Headers["Accept"] = "text/plain"
	if api.Headers["Accept"] != "application/json" {
		t.Error("los headers de la condicion no pueden compartir el mapa de la API")
	}
}

func TestBuildConditionHTTPRequestService_Body(t *testing.T) {
	t.Parallel()

	api := newTestApi()
	api.Method = "POST"
	api.DataLocation = "body"
	api.Auth = nil
	api.RetryPolicy = nil

	config, err := services.BuildConditionHTTPRequestService(api, map[string]string{"symbol": "AAPL", "ratio": "0.6", "adjusted": "0"})
	testutil.RequireEmptyError(t, err)

	// El body conserva los tipos de dato para enviarse como JSON
	want := map[string]any{
		"function": "REALTIME_PUT_CALL_RATIO", "version": int32(2),
		"symbol": "AAPL", "limit": int64(10), "ratio": 0.6, "adjusted": false,
	}
	if !reflect.DeepEqual(config.BodyData, want) {
		t.Errorf("body_data = %#v, want %#v", config.BodyData, want)
	}
	if config.QueryData != nil {
		t.Errorf("query_data = %v, una API BODY no envia query", config.QueryData)
	}
	if config.DataLocation != cmm.ApiDataLocationBody || config.Auth != nil || config.RetryPolicy != nil {
		t.Errorf("config = %+v, se esperaba BODY sin auth ni reintentos", config)
	}
}

func TestBuildConditionHTTPRequestService_WithoutData(t *testing.T) {
	t.Parallel()

	// Una API sin parametros ni data fija no necesita data_location
	api := models.ApisModelType{Method: "GET", URL: "http://localhost:8080/operation/google_trends/series"}
	config, err := services.BuildConditionHTTPRequestService(api, nil)

	testutil.RequireEmptyError(t, err)
	if config.QueryData != nil || config.BodyData != nil {
		t.Errorf("config = %+v, no se esperaba data", config)
	}
}

func TestBuildConditionHTTPRequestService_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		edit         func(api *models.ApisModelType)
		parameters   map[string]string
		wantTracking string
	}{
		{name: "metodo no soportado", edit: func(a *models.ApisModelType) { a.Method = "TRACE" }, wantTracking: "SOPRE027"},
		{name: "sin metodo", edit: func(a *models.ApisModelType) { a.Method = "" }, wantTracking: "SOPRE027"},
		{name: "url sin esquema", edit: func(a *models.ApisModelType) { a.URL = "www.alphavantage.co/query" }, wantTracking: "SOPRE028"},
		{name: "url ftp", edit: func(a *models.ApisModelType) { a.URL = "ftp://alphavantage.co" }, wantTracking: "SOPRE028"},
		{name: "parametro desconocido", parameters: map[string]string{"symbol": "AAPL", "apikey": "x"}, wantTracking: "SOPRE029"},
		{name: "falta un requerido", parameters: map[string]string{"limit": "5"}, wantTracking: "SOPRE030"},
		{name: "parametro reemplaza la data fija", wantTracking: "SOPRE031", parameters: map[string]string{"symbol": "AAPL"},
			edit: func(a *models.ApisModelType) { a.Data = primitive.D{{Key: "symbol", Value: "MSFT"}} }},
		{name: "body en GET", parameters: map[string]string{"symbol": "AAPL"}, wantTracking: "SOPRE032",
			edit: func(a *models.ApisModelType) { a.DataLocation = cmm.ApiDataLocationBody }},
		{name: "data_location invalido", parameters: map[string]string{"symbol": "AAPL"}, wantTracking: "SOPRE033",
			edit: func(a *models.ApisModelType) { a.DataLocation = "PATH" }},
		{name: "opcion STATIC invalida", parameters: map[string]string{"symbol": "AAPL", "interval": "monthly"}, wantTracking: "SOPRE034"},
		{name: "origen de opciones desconocido", parameters: map[string]string{"symbol": "AAPL"}, wantTracking: "SOPRE035",
			edit: func(a *models.ApisModelType) {
				a.Parameters[0].Options = &models.ApiParameterOptions{Source: "DATABASE"}
			}},
		{name: "tipo de dato desconocido", parameters: map[string]string{"symbol": "AAPL"}, wantTracking: "SOPRE036",
			edit: func(a *models.ApisModelType) { a.Parameters[0].DataType = "ARRAY" }},
		{name: "entero invalido", parameters: map[string]string{"symbol": "AAPL", "limit": "1.5"}, wantTracking: "SOPRE037"},
		{name: "decimal invalido", parameters: map[string]string{"symbol": "AAPL", "ratio": "alto"}, wantTracking: "SOPRE037"},
		{name: "booleano invalido", parameters: map[string]string{"symbol": "AAPL", "adjusted": "si"}, wantTracking: "SOPRE037"},
		{name: "fecha invalida", parameters: map[string]string{"symbol": "AAPL", "from": "31/01/2026"}, wantTracking: "SOPRE037"},
		{name: "valor por defecto invalido", parameters: map[string]string{"symbol": "AAPL"}, wantTracking: "SOPRE037",
			edit: func(a *models.ApisModelType) { a.Parameters[1].DefaultValue = ptr("diez") }},
		{name: "data fija que no es objeto", parameters: map[string]string{"symbol": "AAPL"}, wantTracking: "SOPRE038",
			edit: func(a *models.ApisModelType) { a.Data = primitive.A{"function"} }},
		{name: "data fija anidada por query", parameters: map[string]string{"symbol": "AAPL"}, wantTracking: "SOPRE039",
			edit: func(a *models.ApisModelType) {
				a.Data = primitive.D{{Key: "filters", Value: primitive.D{{Key: "a", Value: 1}}}}
			}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			api := newTestApi()
			if tt.edit != nil {
				tt.edit(&api)
			}

			_, err := services.BuildConditionHTTPRequestService(api, tt.parameters)
			testutil.RequireTrackingCode(t, err, tt.wantTracking)
		})
	}
}
