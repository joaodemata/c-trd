package services_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
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

// newTestCallback callback con un umbral requerido y una ventana opcional con valor por defecto
func newTestCallback() providers.CallbackDefinition {
	window := "12"
	return providers.CallbackDefinition{
		ID: "TestCallback",
		Parameters: []providers.CallbackParameter{
			{Name: "threshold", Label: "Umbral", DataType: cmm.ApiParameterTypeFloat, IsRequired: true},
			{Name: "window", Label: "Ventana", DataType: cmm.ApiParameterTypeInteger, DefaultValue: &window},
			{Name: "inclusive", Label: "Incluye el umbral", DataType: cmm.ApiParameterTypeBoolean},
		},
	}
}

func TestBuildConditionCallbackParametersService(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		callback   providers.CallbackDefinition
		parameters map[string]string
		want       map[string]any
	}{
		{name: "valores tipados y default", callback: newTestCallback(), parameters: map[string]string{"threshold": "60.5"},
			want: map[string]any{"threshold": 60.5, "window": int64(12)}},
		{name: "reemplaza el default y agrega el opcional", callback: newTestCallback(), parameters: map[string]string{"threshold": "60", "window": "4", "inclusive": "true"},
			want: map[string]any{"threshold": 60.0, "window": int64(4), "inclusive": true}},
		{name: "callback sin parametros", callback: providers.CallbackDefinition{ID: "SinParametros"}, parameters: nil,
			want: map[string]any{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := services.BuildConditionCallbackParametersService(tt.callback, tt.parameters)
			testutil.RequireEmptyError(t, err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parameters_callback = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestBuildConditionCallbackParametersService_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		callback     providers.CallbackDefinition
		parameters   map[string]string
		wantTracking string
	}{
		{name: "parametro que el callback no acepta", callback: newTestCallback(), parameters: map[string]string{"threshold": "60", "symbol": "AAPL"}, wantTracking: "SOPRE051"},
		{name: "callback sin parametros recibe uno", callback: providers.CallbackDefinition{ID: "SinParametros"}, parameters: map[string]string{"threshold": "60"}, wantTracking: "SOPRE051"},
		{name: "falta el requerido", callback: newTestCallback(), parameters: map[string]string{"window": "4"}, wantTracking: "SOPRE052"},
		{name: "valor que no es del tipo", callback: newTestCallback(), parameters: map[string]string{"threshold": "alto"}, wantTracking: "SOPRE037"},
		{name: "tipo de dato no soportado", callback: providers.CallbackDefinition{Parameters: []providers.CallbackParameter{{Name: "list", DataType: "ARRAY", IsRequired: true}}},
			parameters: map[string]string{"list": "a,b"}, wantTracking: "SOPRE036"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := services.BuildConditionCallbackParametersService(tt.callback, tt.parameters)
			testutil.RequireTrackingCode(t, err, tt.wantTracking)
		})
	}
}

// newRangeTestCallback callback con un umbral FLOAT en [0, 100], una ventana INTEGER mayor o igual a 1 y un texto
// con limites que se ignoran porque no es numerico
func newRangeTestCallback() providers.CallbackDefinition {
	zero, one, hundred := 0.0, 1.0, 100.0
	return providers.CallbackDefinition{
		ID: "RangeTestCallback",
		Parameters: []providers.CallbackParameter{
			{Name: "threshold", Label: "Umbral", DataType: cmm.ApiParameterTypeFloat, IsRequired: true, MinValue: &zero, MaxValue: &hundred},
			{Name: "window", Label: "Ventana", DataType: cmm.ApiParameterTypeInteger, MinValue: &one},
			{Name: "symbol", Label: "Simbolo", DataType: cmm.ApiParameterTypeString, MinValue: &one, MaxValue: &one},
		},
	}
}

func TestBuildConditionCallbackParametersService_Range(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		parameters   map[string]string
		wantTracking string
	}{
		{"limite inferior incluido", map[string]string{"threshold": "0"}, ""},
		{"limite superior incluido", map[string]string{"threshold": "100"}, ""},
		{"dentro del rango", map[string]string{"threshold": "25.5", "window": "1"}, ""},
		{"sin limite superior", map[string]string{"threshold": "50", "window": "1000"}, ""},
		{"texto ignora los limites", map[string]string{"threshold": "50", "symbol": "USDT"}, ""},
		{"negativo", map[string]string{"threshold": "-1"}, "SOPRE053"},
		{"negativo decimal", map[string]string{"threshold": "-0.01"}, "SOPRE053"},
		{"mayor al maximo", map[string]string{"threshold": "100.1"}, "SOPRE053"},
		{"entero menor al minimo", map[string]string{"threshold": "50", "window": "0"}, "SOPRE053"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := services.BuildConditionCallbackParametersService(newRangeTestCallback(), tt.parameters)

			if tt.wantTracking != "" {
				testutil.RequireTrackingCode(t, err, tt.wantTracking)
				return
			}
			testutil.RequireEmptyError(t, err)
		})
	}
}

func TestBuildConditionCallbackParametersService_RangeErrorDetail(t *testing.T) {
	t.Parallel()

	// El detalle del log debe tener los limites y no las direcciones de memoria de MinValue y MaxValue
	_, err := services.BuildConditionCallbackParametersService(newRangeTestCallback(), map[string]string{"threshold": "150"})

	testutil.RequireTrackingCode(t, err, "SOPRE053")
	want := `parameter "threshold" value 150 not entre 0 y 100`
	if err.Err == nil || err.Err.Error() != want {
		t.Errorf("detalle = %v, want %q", err.Err, want)
	}
}

func TestBuildConditionCallbackParametersService_RegisteredThreshold(t *testing.T) {
	t.Parallel()

	// Los callbacks reales de CoinMarketCap rechazan un umbral negativo al crear la condicion
	for _, name := range []string{"FearAndGreedBelow", "BtcDominanceAbove", "UsdtDominanceBelow", "Others3DominanceAbove"} {
		_, err := services.BuildConditionCallbackParametersService(providers.FunctionRegistry[name], map[string]string{"threshold": "-5"})
		testutil.RequireTrackingCode(t, err, "SOPRE053")
	}
}

// incompatibleCondition condicion guardada; todas leen la misma API y peticion salvo que se cambie
func incompatibleCondition(callback string, params map[string]any) models.ConditionsModelType {
	return models.ConditionsModelType{
		ID: primitive.NewObjectID(), Tag: callback, CallbackFunction: callback, ParametersCallback: params,
		IDApi:       incompatibleApi,
		HttpRequest: providers.HTTPRequestConfig{URL: "https://api.example.com/v1", Method: "GET"},
	}
}

var incompatibleApi = primitive.NewObjectID()

func TestFindIncompatibleConditionsService(t *testing.T) {
	t.Parallel()

	fear := incompatibleCondition("FearAndGreedFear", map[string]any{})
	fearCopy := fear
	fearCopy.ID = primitive.NewObjectID()
	fredRising := incompatibleCondition("FredSeriesRising", map[string]any{})
	fredFallingOther := incompatibleCondition("FredSeriesFalling", map[string]any{})
	fredFallingOther.HttpRequest.QueryData = map[string]string{"series_id": "DGS10"}
	fearOtherApi := incompatibleCondition("FearAndGreedGreed", map[string]any{})
	fearOtherApi.IDApi = primitive.NewObjectID()

	tests := []struct {
		name       string
		conditions []models.ConditionsModelType
		want       string
	}{
		{"sin condiciones", nil, ""},
		{"compatibles", []models.ConditionsModelType{fear, incompatibleCondition("BtcDominanceAbove60", map[string]any{}), fredRising}, ""},
		{"zonas excluyentes", []models.ConditionsModelType{fear, incompatibleCondition("FearAndGreedGreed", map[string]any{})}, "excluyentes"},
		{"zonas de otra API", []models.ConditionsModelType{fear, fearOtherApi}, ""},
		{"misma condicion", []models.ConditionsModelType{fear, fearCopy}, "misma condicion"},
		{"FED excluyentes", []models.ConditionsModelType{incompatibleCondition("FedNextMeetingCutLikely", map[string]any{}), incompatibleCondition("FedNextMeetingHikeLikely", map[string]any{})}, "excluyentes"},
		{"FRED misma serie", []models.ConditionsModelType{fredRising, incompatibleCondition("FredSeriesFalling", map[string]any{})}, "excluyentes"},
		{"FRED otra serie", []models.ConditionsModelType{fredRising, fredFallingOther}, ""},
		{"umbrales sin rango", []models.ConditionsModelType{
			incompatibleCondition("FearAndGreedBelow", map[string]any{"threshold": int64(25)}),
			incompatibleCondition("FearAndGreedAbove", map[string]any{"threshold": 30.0}),
		}, "> 30 y < 25"},
		{"umbrales con rango (int32 de Mongo)", []models.ConditionsModelType{
			incompatibleCondition("FearAndGreedBelow", map[string]any{"threshold": int64(25)}),
			incompatibleCondition("FearAndGreedAbove", map[string]any{"threshold": int32(20)}),
		}, ""},
		{"espejo OTHERS2 sin rango", []models.ConditionsModelType{
			incompatibleCondition("BtcDominanceAbove60", map[string]any{}),
			incompatibleCondition("Others2DominanceAbove", map[string]any{"threshold": 45.0}),
		}, "> 60 y < 55"},
		{"espejo OTHERS2 con rango", []models.ConditionsModelType{
			incompatibleCondition("BtcDominanceAbove60", map[string]any{}),
			incompatibleCondition("Others2DominanceAbove", map[string]any{"threshold": 35.0}),
		}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			problems := services.FindIncompatibleConditionsService(tt.conditions)
			if tt.want == "" {
				if len(problems) != 0 {
					t.Fatalf("problemas = %v, se esperaba ninguno", problems)
				}
				return
			}
			if len(problems) != 1 || !strings.Contains(problems[0], tt.want) {
				t.Fatalf("problemas = %v, se esperaba uno con %q", problems, tt.want)
			}
		})
	}
}

// ---------- Portfolio ----------

// portfolioTx transaccion de un asset para reconstruir posiciones, el dia es el orden de ejecucion
func portfolioTx(idAsset primitive.ObjectID, status cmm.CatalogStatus, day int, quantity, value, fee float64) models.PortfolioTransactionsModelType {
	executedAt := time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC)
	return models.PortfolioTransactionsModelType{
		ID:                 primitive.NewObjectID(),
		TagTypeTransaction: status.Tag,
		IDAsset:            &idAsset,
		TagAsset:           "ASSET-BTC",
		NameAsset:          "Bitcoin",
		Quantity:           quantity,
		Value:              value,
		Fee:                fee,
		ExecutedAt:         executedAt,
		CreatedDate:        executedAt,
	}
}

// requireAmount compara montos en float64 con margen de redondeo
func requireAmount(t *testing.T, field string, got, want float64) {
	t.Helper()
	if diff := got - want; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("%s = %v, se esperaba %v", field, got, want)
	}
}

func TestBuildPortfolioAssetPositionService(t *testing.T) {
	t.Parallel()

	idAsset := primitive.NewObjectID()
	buy, sell, dividend := cmm.TransactionTypeBuy, cmm.TransactionTypeSell, cmm.TransactionTypeDividend

	tests := []struct {
		name         string
		transactions []models.PortfolioTransactionsModelType
		quantity     float64
		averageCost  float64
		costBasis    float64
		realizedPnL  float64
	}{
		{
			name:         "una compra con comision",
			transactions: []models.PortfolioTransactionsModelType{portfolioTx(idAsset, buy, 1, 2, 200, 2)},
			quantity:     2, averageCost: 101, costBasis: 202,
		},
		{
			name: "dos compras promedian el costo",
			transactions: []models.PortfolioTransactionsModelType{
				portfolioTx(idAsset, buy, 1, 1, 100, 0),
				portfolioTx(idAsset, buy, 2, 1, 200, 0),
			},
			quantity: 2, averageCost: 150, costBasis: 300,
		},
		{
			name: "venta parcial realiza ganancia y no cambia el costo promedio",
			transactions: []models.PortfolioTransactionsModelType{
				portfolioTx(idAsset, buy, 1, 2, 200, 0),
				portfolioTx(idAsset, sell, 2, 1, 150, 1),
			},
			quantity: 1, averageCost: 100, costBasis: 100, realizedPnL: 49,
		},
		{
			name: "venta total deja la posicion en cero con la ganancia realizada",
			transactions: []models.PortfolioTransactionsModelType{
				portfolioTx(idAsset, buy, 1, 0.3, 30, 0),
				portfolioTx(idAsset, sell, 2, 0.1, 5, 0),
				portfolioTx(idAsset, sell, 3, 0.2, 10, 0),
			},
			quantity: 0, averageCost: 0, costBasis: 0, realizedPnL: -15,
		},
		{
			name: "dividendo suma a la ganancia realizada sin mover la cantidad",
			transactions: []models.PortfolioTransactionsModelType{
				portfolioTx(idAsset, buy, 1, 10, 1000, 0),
				portfolioTx(idAsset, dividend, 2, 0, 25, 0.5),
			},
			quantity: 10, averageCost: 100, costBasis: 1000, realizedPnL: 24.5,
		},
		{
			name: "se ordenan por fecha de ejecucion y no por el orden recibido",
			transactions: []models.PortfolioTransactionsModelType{
				portfolioTx(idAsset, sell, 5, 1, 300, 0),
				portfolioTx(idAsset, buy, 1, 1, 100, 0),
			},
			quantity: 0, realizedPnL: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			position, exists, err := services.BuildPortfolioAssetPositionService(tt.transactions)

			testutil.RequireEmptyError(t, err)
			if !exists {
				t.Fatal("con transacciones el asset debe seguir en el portfolio")
			}
			requireAmount(t, "quantity", position.Quantity, tt.quantity)
			requireAmount(t, "averageCost", position.AverageCost, tt.averageCost)
			requireAmount(t, "costBasis", position.CostBasis, tt.costBasis)
			requireAmount(t, "realizedPnl", position.RealizedPnL, tt.realizedPnL)
			if position.IDAsset != idAsset || position.TagAsset != "ASSET-BTC" {
				t.Errorf("asset = %s %q, se esperaba el de las transacciones", position.IDAsset.Hex(), position.TagAsset)
			}
		})
	}
}

func TestBuildPortfolioAssetPositionService_Errors(t *testing.T) {
	t.Parallel()

	idAsset := primitive.NewObjectID()

	tests := []struct {
		name         string
		transactions []models.PortfolioTransactionsModelType
		wantTracking string
	}{
		{"venta sin compras", []models.PortfolioTransactionsModelType{
			portfolioTx(idAsset, cmm.TransactionTypeSell, 1, 1, 100, 0),
		}, "SOPRE073"},
		{"venta mayor a lo disponible en ese momento", []models.PortfolioTransactionsModelType{
			portfolioTx(idAsset, cmm.TransactionTypeBuy, 1, 1, 100, 0),
			portfolioTx(idAsset, cmm.TransactionTypeSell, 2, 2, 300, 0),
			portfolioTx(idAsset, cmm.TransactionTypeBuy, 3, 5, 500, 0),
		}, "SOPRE073"},
		{"deposito entre las transacciones del asset", []models.PortfolioTransactionsModelType{
			portfolioTx(idAsset, cmm.TransactionTypeDeposit, 1, 0, 100, 0),
		}, "SOPRE074"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := services.BuildPortfolioAssetPositionService(tt.transactions)
			testutil.RequireTrackingCode(t, err, tt.wantTracking)
		})
	}
}

func TestBuildPortfolioAssetPositionService_NoTransactions(t *testing.T) {
	t.Parallel()

	_, exists, err := services.BuildPortfolioAssetPositionService(nil)

	testutil.RequireEmptyError(t, err)
	if exists {
		t.Error("sin transacciones el asset sale del portfolio")
	}
}

func TestSetPortfolioAssetPositionService(t *testing.T) {
	t.Parallel()

	btc, eth, sol := primitive.NewObjectID(), primitive.NewObjectID(), primitive.NewObjectID()
	assets := []models.PortfolioAsset{{IDAsset: btc, Quantity: 1}, {IDAsset: eth, Quantity: 2}}

	tests := []struct {
		name         string
		idAsset      primitive.ObjectID
		position     *models.PortfolioAsset
		wantAssets   []primitive.ObjectID
		wantQuantity map[primitive.ObjectID]float64
	}{
		{"reemplaza en su lugar", btc, &models.PortfolioAsset{IDAsset: btc, Quantity: 5}, []primitive.ObjectID{btc, eth}, map[primitive.ObjectID]float64{btc: 5, eth: 2}},
		{"agrega al final", sol, &models.PortfolioAsset{IDAsset: sol, Quantity: 3}, []primitive.ObjectID{btc, eth, sol}, map[primitive.ObjectID]float64{btc: 1, eth: 2, sol: 3}},
		{"quita el asset", btc, nil, []primitive.ObjectID{eth}, map[primitive.ObjectID]float64{eth: 2}},
		{"quitar uno que no esta no cambia nada", sol, nil, []primitive.ObjectID{btc, eth}, map[primitive.ObjectID]float64{btc: 1, eth: 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := services.SetPortfolioAssetPositionService(assets, tt.idAsset, tt.position)

			ids := make([]primitive.ObjectID, len(got))
			for i, asset := range got {
				ids[i] = asset.IDAsset
				if asset.Quantity != tt.wantQuantity[asset.IDAsset] {
					t.Errorf("quantity de %s = %v, se esperaba %v", asset.IDAsset.Hex(), asset.Quantity, tt.wantQuantity[asset.IDAsset])
				}
			}
			if !slices.Equal(ids, tt.wantAssets) {
				t.Errorf("assets = %v, se esperaba %v", ids, tt.wantAssets)
			}
		})
	}

	// La cartera recibida no se modifica y una cartera vacia nunca queda en nil
	if assets[0].Quantity != 1 || len(assets) != 2 {
		t.Errorf("la cartera original cambio: %+v", assets)
	}
	if empty := services.SetPortfolioAssetPositionService(nil, btc, nil); empty == nil {
		t.Error("se esperaba [] y no nil para que se guarde como arreglo vacio")
	}
}

// TestBacktestDatasetFileService no es paralela porque t.Setenv cambia la carpeta de los backtests
func TestBacktestDatasetFileService(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(cmm.BacktestDirEnv, dir)

	first := []byte("time,open,high,low,close\n1,1,1,1,1\n")
	second := []byte("time,open,high,low,close\n2,2,2,2,2\n")

	firstSha, err := services.SaveBacktestDatasetFileService(first)
	testutil.RequireEmptyError(t, err)
	if len(firstSha) != 64 {
		t.Fatalf("sha256 = %q, se esperaban 64 caracteres hexadecimales", firstSha)
	}

	// El mismo contenido da el mismo archivo y otro contenido no reemplaza al anterior
	again, err := services.SaveBacktestDatasetFileService(first)
	testutil.RequireEmptyError(t, err)
	secondSha, err := services.SaveBacktestDatasetFileService(second)
	testutil.RequireEmptyError(t, err)
	if again != firstSha || secondSha == firstSha {
		t.Fatalf("hashes = %s, %s, %s", firstSha, again, secondSha)
	}

	content, err := services.ReadBacktestDatasetFileService(firstSha)
	testutil.RequireEmptyError(t, err)
	if string(content) != string(first) {
		t.Errorf("contenido = %q, se esperaba el primer CSV", content)
	}

	entries, errDir := os.ReadDir(dir)
	if errDir != nil || len(entries) != 2 {
		t.Errorf("archivos en la carpeta = %d (%v), se esperaban 2 sin temporales", len(entries), errDir)
	}

	_, err = services.ReadBacktestDatasetFileService("../../etc/passwd")
	testutil.RequireTrackingCode(t, err, "SOPRE104")

	_, err = services.ReadBacktestDatasetFileService(strings.Repeat("a", 64))
	testutil.RequireTrackingCode(t, err, "SOPRE105")

	if errWrite := os.WriteFile(filepath.Join(dir, secondSha+".csv"), []byte("cambiado"), 0o600); errWrite != nil {
		t.Fatal(errWrite)
	}
	_, err = services.ReadBacktestDatasetFileService(secondSha)
	testutil.RequireTrackingCode(t, err, "SOPRE107")
}

func TestPineCodeSha256Service(t *testing.T) {
	t.Parallel()

	if got := services.PineCodeSha256Service(nil); got != nil {
		t.Errorf("hash sin pine_code = %v, se esperaba nil", *got)
	}

	code := "abc"
	got := services.PineCodeSha256Service(&code)
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got == nil || *got != want {
		t.Errorf("hash = %v, se esperaba %s", got, want)
	}
}
