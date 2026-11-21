package cronjobs_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/cronjobs"
	"c_trd/models"
	"c_trd/providers"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNewConditionStatus(t *testing.T) {
	t.Parallel()

	active := cmm.GenericStatusActive
	inactive := cmm.GenericStatusInactive

	tests := []struct {
		name          string
		currentStatus string
		evaluated     bool
		accepted      bool
		want          *cmm.CatalogStatus // nil si el status no debe cambiar
	}{
		{name: "no evaluada conserva el status", currentStatus: active.Tag, evaluated: false, accepted: false, want: nil},
		{name: "inactiva que se cumple pasa a activa", currentStatus: inactive.Tag, evaluated: true, accepted: true, want: &active},
		{name: "activa que se cumple no cambia", currentStatus: active.Tag, evaluated: true, accepted: true, want: nil},
		{name: "activa que deja de cumplirse pasa a inactiva", currentStatus: active.Tag, evaluated: true, accepted: false, want: &inactive},
		{name: "inactiva que no se cumple no cambia", currentStatus: inactive.Tag, evaluated: true, accepted: false, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := cronjobs.ConditionResult{
				Condition: models.ConditionsModelType{TagStatus: tt.currentStatus},
				Evaluated: tt.evaluated,
				Accepted:  tt.accepted,
			}

			got := cronjobs.NewConditionStatus(result)
			switch {
			case tt.want == nil && got != nil:
				t.Errorf("status = %s, se esperaba que no cambiara", got.Tag)
			case tt.want != nil && got == nil:
				t.Errorf("el status no cambio, se esperaba %s", tt.want.Tag)
			case tt.want != nil && *got != *tt.want:
				t.Errorf("status = %s, se esperaba %s", got.Tag, tt.want.Tag)
			}
		})
	}
}

func TestConditionInterval(t *testing.T) {
	t.Parallel()

	seconds := func(value int) *int { return &value }
	defaultInterval := time.Duration(cmm.DefaultConditionIntervalSeconds) * time.Second

	tests := []struct {
		name     string
		interval *int
		want     time.Duration
	}{
		{"sin interval_seconds usa el default", nil, defaultInterval},
		{"cero usa el default", seconds(0), defaultInterval},
		{"negativo usa el default", seconds(-30), defaultInterval},
		{"intervalo configurado", seconds(300), 300 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := cronjobs.ConditionInterval(models.ConditionsModelType{IntervalSeconds: tt.interval})
			if got != tt.want {
				t.Errorf("ConditionInterval() = %v, se esperaba %v", got, tt.want)
			}
		})
	}
}

// Las pruebas de EvaluateCondition modifican providers.FunctionRegistry (estado global),
// por eso no usan t.Parallel

func TestEvaluateCondition(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		io.WriteString(w, "bullish")
	}))
	t.Cleanup(server.Close)

	asOf := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	registerCallback(t, "TestIsBullish", func(body []byte, parameters map[string]any) (providers.CallbackResult, *cmm.ErrorHandler) {
		isBullish := string(body) == "bullish"
		result := providers.CallbackResult{Accepted: isBullish, Value: 1.27, AsOf: &asOf}
		return result, cmm.NewEmptyErrorHandler()
	})
	registerCallback(t, "TestCallbackError", func(body []byte, parameters map[string]any) (providers.CallbackResult, *cmm.ErrorHandler) {
		// Aunque el callback devuelva datos junto con el error, el resultado no debe usarlos
		result := providers.CallbackResult{Accepted: true, Value: 99, AsOf: &asOf}
		return result, cmm.NewErrorHandler(errors.New("respuesta inesperada"), "callback fallido", cmm.LevelError, "TESTE005")
	})
	registerCallback(t, "TestCallbackPanic", func(body []byte, parameters map[string]any) (providers.CallbackResult, *cmm.ErrorHandler) {
		panic("bug en el callback")
	})

	tests := []struct {
		name          string
		url           string
		callback      string
		wantEvaluated bool
		wantAccepted  bool
		wantTracking  string
	}{
		{name: "condicion que se cumple", url: server.URL, callback: "TestIsBullish", wantEvaluated: true, wantAccepted: true},
		{name: "sin URL no se evalua", url: "", callback: "TestIsBullish", wantTracking: "CJCHCE001"},
		{name: "callback no registrado", url: server.URL, callback: "NoExiste", wantTracking: "CJCHCE002"},
		{name: "la API responde con error", url: server.URL + "/fail", callback: "TestIsBullish", wantTracking: "PRHTTPE004"},
		{name: "el callback retorna error", url: server.URL, callback: "TestCallbackError", wantTracking: "TESTE005"},
		{name: "un panic en el callback no tumba el cronjob", url: server.URL, callback: "TestCallbackPanic", wantTracking: "CJCHCE003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			condition := newTestCondition(tt.url, tt.callback)

			result := cronjobs.EvaluateCondition(context.Background(), condition, nil)

			if result.Evaluated != tt.wantEvaluated || result.Accepted != tt.wantAccepted {
				t.Errorf("evaluated = %v, accepted = %v; se esperaba evaluated = %v, accepted = %v",
					result.Evaluated, result.Accepted, tt.wantEvaluated, tt.wantAccepted)
			}
			if result.Condition.ID != condition.ID {
				t.Error("el resultado debe conservar la condicion evaluada")
			}
			if result.ExecutedAt.IsZero() {
				t.Error("ExecutedAt debe registrarse aunque la evaluacion falle")
			}
			if result.ErrorTrackingCode != tt.wantTracking {
				t.Errorf("errorTrackingCode = %q, se esperaba %q", result.ErrorTrackingCode, tt.wantTracking)
			}

			// Solo una evaluacion exitosa guarda el valor y la fecha del dato
			if tt.wantEvaluated {
				if result.Value != 1.27 || result.AsOf == nil || !result.AsOf.Equal(asOf) {
					t.Errorf("value = %v, asOf = %v; se esperaba 1.27, %s", result.Value, result.AsOf, asOf)
				}
			} else if result.Value != 0 || result.AsOf != nil {
				t.Errorf("value = %v, asOf = %v; una evaluacion fallida no debe guardar datos", result.Value, result.AsOf)
			}
		})
	}
}

func TestEvaluateConditions_KeepsOrderWithMoreConditionsThanWorkers(t *testing.T) {
	// El servidor responde el numero recibido y el callback acepta los pares
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, r.URL.Query().Get("n"))
	}))
	t.Cleanup(server.Close)

	registerCallback(t, "TestIsEven", func(body []byte, parameters map[string]any) (providers.CallbackResult, *cmm.ErrorHandler) {
		n, err := strconv.Atoi(string(body))
		if err != nil {
			return providers.CallbackResult{}, cmm.NewErrorHandler(err, "numero invalido", cmm.LevelError, "TESTE006")
		}
		isEven := n%2 == 0
		result := providers.CallbackResult{Accepted: isEven, Value: float64(n)}
		return result, cmm.NewEmptyErrorHandler()
	})

	conditions := make([]models.ConditionsModelType, cmm.ConditionEvaluationWorkers*2+3)
	for i := range conditions {
		conditions[i] = newTestCondition(server.URL, "TestIsEven")
		conditions[i].HttpRequest.QueryData = map[string]string{"n": strconv.Itoa(i)}
	}

	results := cronjobs.EvaluateConditions(context.Background(), conditions, nil)

	if len(results) != len(conditions) {
		t.Fatalf("resultados = %d, se esperaban %d", len(results), len(conditions))
	}
	for i, result := range results {
		if result.Condition.ID != conditions[i].ID {
			t.Errorf("resultado %d corresponde a otra condicion", i)
		}
		if !result.Evaluated || result.Accepted != (i%2 == 0) || result.Value != float64(i) {
			t.Errorf("resultado %d: evaluated = %v, accepted = %v, value = %v", i, result.Evaluated, result.Accepted, result.Value)
		}
	}
}

func TestEvaluateCondition_PassesParametersCallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "70")
	}))
	t.Cleanup(server.Close)

	// El callback acepta si el valor de la respuesta supera el umbral de la condicion
	registerCallback(t, "TestAboveThreshold", func(body []byte, parameters map[string]any) (providers.CallbackResult, *cmm.ErrorHandler) {
		value, err := strconv.ParseFloat(string(body), 64)
		if err != nil {
			return providers.CallbackResult{}, cmm.NewErrorHandler(err, "numero invalido", cmm.LevelError, "TESTE007")
		}
		threshold, ok := parameters["threshold"].(float64)
		if !ok {
			return providers.CallbackResult{}, cmm.NewErrorHandler(nil, "umbral invalido", cmm.LevelError, "TESTE008")
		}
		isAbove := value > threshold
		result := providers.CallbackResult{Accepted: isAbove, Value: value}
		return result, cmm.NewEmptyErrorHandler()
	})

	tests := []struct {
		name          string
		parameters    map[string]any
		wantEvaluated bool
		wantAccepted  bool
	}{
		{name: "supera el umbral", parameters: map[string]any{"threshold": 60.0}, wantEvaluated: true, wantAccepted: true},
		{name: "no supera el umbral", parameters: map[string]any{"threshold": 80.0}, wantEvaluated: true, wantAccepted: false},
		{name: "sin parametros el callback falla", parameters: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			condition := newTestCondition(server.URL, "TestAboveThreshold")
			condition.ParametersCallback = tt.parameters

			result := cronjobs.EvaluateCondition(context.Background(), condition, nil)

			if result.Evaluated != tt.wantEvaluated || result.Accepted != tt.wantAccepted {
				t.Errorf("evaluated = %v, accepted = %v; se esperaba evaluated = %v, accepted = %v",
					result.Evaluated, result.Accepted, tt.wantEvaluated, tt.wantAccepted)
			}
		})
	}
}

func TestNewConditionEvaluation(t *testing.T) {
	t.Parallel()

	executedAt := time.Date(2026, 11, 21, 10, 30, 0, 0, time.UTC)
	asOf := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	condition := newTestCondition("https://api.test", "FearAndGreedBelow")
	condition.ParametersCallback = map[string]any{"threshold": 25.0}

	tests := []struct {
		name         string
		result       cronjobs.ConditionResult
		wantAccepted bool
		wantValue    *float64
		wantAsOf     *time.Time
		wantSource   *string
		wantTracking *string
	}{
		{
			name:         "con fecha del proveedor guarda esa fecha con origen PROVIDER",
			result:       cronjobs.ConditionResult{Evaluated: true, Accepted: true, Value: 18, AsOf: &asOf},
			wantAccepted: true, wantValue: floatPointer(18), wantAsOf: &asOf, wantSource: stringPointer(cmm.AsOfSourceProvider),
		},
		{
			// Un valor 0 medido es un dato, no debe confundirse con "sin valor"
			name:         "sin fecha del proveedor usa executed_at con origen SERVER",
			result:       cronjobs.ConditionResult{Evaluated: true, Accepted: false, Value: 0},
			wantAccepted: false, wantValue: floatPointer(0), wantAsOf: &executedAt, wantSource: stringPointer(cmm.AsOfSourceServer),
		},
		{
			name:         "fallida guarda el tracking code y no guarda valor",
			result:       cronjobs.ConditionResult{Evaluated: false, ErrorTrackingCode: "PRHTTPE004"},
			wantTracking: stringPointer("PRHTTPE004"),
		},
		{
			// Si llegara un resultado no evaluado con datos (no deberia), no se guardan
			name:         "fallida ignora valor y aceptacion",
			result:       cronjobs.ConditionResult{Evaluated: false, Accepted: true, Value: 50, AsOf: &asOf, ErrorTrackingCode: "CJCHCE003"},
			wantTracking: stringPointer("CJCHCE003"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.result.Condition = condition
			tt.result.ExecutedAt = executedAt

			got, err := cronjobs.NewConditionEvaluation(tt.result)

			testutil.RequireEmptyError(t, err)
			if got.ID.IsZero() || got.IDCondition != condition.ID {
				t.Errorf("id = %s, idCondition = %s; se esperaba un id nuevo y la condicion %s", got.ID.Hex(), got.IDCondition.Hex(), condition.ID.Hex())
			}
			if got.CallbackFunction != "FearAndGreedBelow" || got.ParametersCallback["threshold"] != 25.0 {
				t.Errorf("callback = %s %v; se esperaba la copia del callback y sus parametros", got.CallbackFunction, got.ParametersCallback)
			}
			if got.Evaluated != tt.result.Evaluated || got.Accepted != tt.wantAccepted {
				t.Errorf("evaluated, accepted = %v, %v; se esperaba %v, %v", got.Evaluated, got.Accepted, tt.result.Evaluated, tt.wantAccepted)
			}
			if !got.ExecutedAt.Equal(executedAt) {
				t.Errorf("executedAt = %s, se esperaba %s", got.ExecutedAt, executedAt)
			}
			if !equalPointer(got.Value, tt.wantValue) {
				t.Errorf("value = %v, se esperaba %v", printablePointer(got.Value), printablePointer(tt.wantValue))
			}
			if (got.AsOf == nil) != (tt.wantAsOf == nil) || (got.AsOf != nil && !got.AsOf.Equal(*tt.wantAsOf)) {
				t.Errorf("asOf = %v, se esperaba %v", got.AsOf, tt.wantAsOf)
			}
			if !equalPointer(got.AsOfSource, tt.wantSource) {
				t.Errorf("asOfSource = %v, se esperaba %v", printablePointer(got.AsOfSource), printablePointer(tt.wantSource))
			}
			if !equalPointer(got.ErrorTrackingCode, tt.wantTracking) {
				t.Errorf("errorTrackingCode = %v, se esperaba %v", printablePointer(got.ErrorTrackingCode), printablePointer(tt.wantTracking))
			}
		})
	}
}

func floatPointer(value float64) *float64 { return &value }

func stringPointer(value string) *string { return &value }

// equalPointer compara dos punteros opcionales por su valor
func equalPointer[T comparable](got, want *T) bool {
	if got == nil || want == nil {
		return got == want
	}
	return *got == *want
}

// printablePointer valor del puntero para los mensajes de error, o "nil"
func printablePointer[T any](value *T) any {
	if value == nil {
		return "nil"
	}
	return *value
}

// registerCallback agrega un callback de prueba al registro y lo elimina al terminar la prueba
func registerCallback(t *testing.T, name string, callback providers.CallbackFunc) {
	t.Helper()

	if _, exists := providers.FunctionRegistry[name]; exists {
		t.Fatalf("el callback %s ya existe en FunctionRegistry", name)
	}
	providers.FunctionRegistry[name] = providers.CallbackDefinition{Fn: callback}
	t.Cleanup(func() { delete(providers.FunctionRegistry, name) })
}

// newTestCondition condicion de tipo cronjob sin autenticacion
func newTestCondition(url string, callback string) models.ConditionsModelType {
	return models.ConditionsModelType{
		ID:               primitive.NewObjectID(),
		HttpRequest:      providers.HTTPRequestConfig{URL: url, Method: http.MethodGet},
		CallbackFunction: callback,
		TagStatus:        cmm.GenericStatusInactive.Tag,
	}
}

func TestOutOfSyncConditions(t *testing.T) {
	t.Parallel()

	active := cmm.GenericStatusActive
	inactive := cmm.GenericStatusInactive
	idA := primitive.NewObjectID()
	idB := primitive.NewObjectID()

	// condition status real en la coleccion de condiciones
	condition := func(id primitive.ObjectID, status cmm.CatalogStatus) models.ConditionsModelType {
		return models.ConditionsModelType{ID: id, IDStatus: status.ID, TagStatus: status.Tag, Status: status.Name}
	}
	// opportunity oportunidad con las copias de las condiciones y su status
	opportunity := func(copies map[primitive.ObjectID]cmm.CatalogStatus) models.OpportunitiesModelType {
		opp := models.OpportunitiesModelType{ID: primitive.NewObjectID()}
		for id, status := range copies {
			opp.Conditions = append(opp.Conditions, models.Condition{IDCondition: id, IDStatus: status.ID, TagStatus: status.Tag, Status: status.Name})
		}
		return opp
	}

	tests := []struct {
		name          string
		opportunities []models.OpportunitiesModelType
		conditions    []models.ConditionsModelType
		want          map[primitive.ObjectID]cmm.CatalogStatus
	}{
		{
			name:          "copias al dia no generan cambios",
			opportunities: []models.OpportunitiesModelType{opportunity(map[primitive.ObjectID]cmm.CatalogStatus{idA: active, idB: inactive})},
			conditions:    []models.ConditionsModelType{condition(idA, active), condition(idB, inactive)},
			want:          map[primitive.ObjectID]cmm.CatalogStatus{},
		},
		{
			// Oportunidad creada por el hook con el status leido antes de que el cronjob guardara el lote
			name:          "copia vieja se corrige con el status real",
			opportunities: []models.OpportunitiesModelType{opportunity(map[primitive.ObjectID]cmm.CatalogStatus{idA: inactive, idB: active})},
			conditions:    []models.ConditionsModelType{condition(idA, active), condition(idB, active)},
			want:          map[primitive.ObjectID]cmm.CatalogStatus{idA: active},
		},
		{
			name: "condicion desincronizada en varias oportunidades aparece una vez",
			opportunities: []models.OpportunitiesModelType{
				opportunity(map[primitive.ObjectID]cmm.CatalogStatus{idA: active}),
				opportunity(map[primitive.ObjectID]cmm.CatalogStatus{idA: active}),
				opportunity(map[primitive.ObjectID]cmm.CatalogStatus{idA: inactive}),
			},
			conditions: []models.ConditionsModelType{condition(idA, inactive)},
			want:       map[primitive.ObjectID]cmm.CatalogStatus{idA: inactive},
		},
		{
			name:          "condicion borrada se ignora",
			opportunities: []models.OpportunitiesModelType{opportunity(map[primitive.ObjectID]cmm.CatalogStatus{idA: inactive})},
			conditions:    []models.ConditionsModelType{},
			want:          map[primitive.ObjectID]cmm.CatalogStatus{},
		},
		{
			name:          "sin oportunidades",
			opportunities: nil,
			conditions:    []models.ConditionsModelType{condition(idA, active)},
			want:          map[primitive.ObjectID]cmm.CatalogStatus{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			changes := cronjobs.OutOfSyncConditions(tt.opportunities, tt.conditions)
			if changes == nil {
				t.Fatal("se esperaba [] y no nil")
			}
			if len(changes) != len(tt.want) {
				t.Fatalf("changes = %+v, want %+v", changes, tt.want)
			}
			for _, change := range changes {
				want, exists := tt.want[change.IDCondition]
				if !exists || change.NewStatus != want {
					t.Errorf("change = %+v, se esperaba el status real %+v", change, want)
				}
			}
		})
	}
}
