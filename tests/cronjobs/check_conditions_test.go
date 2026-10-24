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

	registerCallback(t, "TestIsBullish", func(body []byte) (bool, *cmm.ErrorHandler) {
		return string(body) == "bullish", cmm.NewEmptyErrorHandler()
	})
	registerCallback(t, "TestCallbackError", func(body []byte) (bool, *cmm.ErrorHandler) {
		return true, cmm.NewErrorHandler(errors.New("respuesta inesperada"), "callback fallido", cmm.LevelError, "TESTE005")
	})
	registerCallback(t, "TestCallbackPanic", func(body []byte) (bool, *cmm.ErrorHandler) {
		panic("bug en el callback")
	})

	tests := []struct {
		name          string
		url           string
		callback      string
		wantEvaluated bool
		wantAccepted  bool
	}{
		{name: "condicion que se cumple", url: server.URL, callback: "TestIsBullish", wantEvaluated: true, wantAccepted: true},
		{name: "sin URL no se evalua", url: "", callback: "TestIsBullish"},
		{name: "callback no registrado", url: server.URL, callback: "NoExiste"},
		{name: "la API responde con error", url: server.URL + "/fail", callback: "TestIsBullish"},
		{name: "el callback retorna error", url: server.URL, callback: "TestCallbackError"},
		{name: "un panic en el callback no tumba el cronjob", url: server.URL, callback: "TestCallbackPanic"},
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
		})
	}
}

func TestEvaluateConditions_KeepsOrderWithMoreConditionsThanWorkers(t *testing.T) {
	// El servidor responde el numero recibido y el callback acepta los pares
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, r.URL.Query().Get("n"))
	}))
	t.Cleanup(server.Close)

	registerCallback(t, "TestIsEven", func(body []byte) (bool, *cmm.ErrorHandler) {
		n, err := strconv.Atoi(string(body))
		if err != nil {
			return false, cmm.NewErrorHandler(err, "numero invalido", cmm.LevelError, "TESTE006")
		}
		return n%2 == 0, cmm.NewEmptyErrorHandler()
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
		if !result.Evaluated || result.Accepted != (i%2 == 0) {
			t.Errorf("resultado %d: evaluated = %v, accepted = %v", i, result.Evaluated, result.Accepted)
		}
	}
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
