package models_test

import (
	"encoding/json"
	"strings"
	"testing"

	"c_trd/models"
	"c_trd/providers"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// jsonKeys serializa value a JSON y retorna sus llaves de primer nivel
func jsonKeys(t *testing.T, value any) map[string]json.RawMessage {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	keys := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return keys
}

// requireHiddenKeys valida que las llaves sensibles no se envien al frontend y que las visibles si
func requireHiddenKeys(t *testing.T, keys map[string]json.RawMessage, hidden []string, visible []string) {
	t.Helper()

	for _, key := range hidden {
		if _, ok := keys[key]; ok {
			t.Errorf("la llave %q no debe enviarse al frontend", key)
		}
	}
	for _, key := range visible {
		if _, ok := keys[key]; !ok {
			t.Errorf("la llave %q debe enviarse al frontend", key)
		}
	}
}

func TestApisModel_HidesRequestData(t *testing.T) {
	t.Parallel()

	api := models.ApisModelType{
		ID:             primitive.NewObjectID(),
		Tag:            "API-TRENDS",
		Method:         "GET",
		URL:            "https://api.example.com/trends",
		Headers:        map[string]string{"X-Key": "secret"},
		DataLocation:   "QUERY",
		RetryPolicy:    &providers.RetryPolicy{},
		TimeoutSeconds: 10,
		ResponseFormat: "GOOGLE_TRENDS_SERIES",
	}

	requireHiddenKeys(t, jsonKeys(t, api),
		[]string{"method", "url", "headers", "dataLocation", "retryPolicy", "timeoutSeconds"},
		[]string{"id", "tag", "responseFormat", "parameters"},
	)
}

func TestConditionsModel_HidesRequestData(t *testing.T) {
	t.Parallel()

	condition := models.ConditionsModelType{
		ID:          primitive.NewObjectID(),
		Tag:         "CONDITION-RSI",
		Metadata:    map[string]any{"note": "interno"},
		HttpRequest: providers.HTTPRequestConfig{URL: "https://api.example.com", Method: "GET"},
	}

	requireHiddenKeys(t, jsonKeys(t, condition),
		[]string{"metadata", "httpRequest"},
		[]string{"id", "tag", "callbackFunction", "parametersCallback"},
	)
}

func TestTriggersModel_HidesWebhookID(t *testing.T) {
	t.Parallel()

	trigger := newTestTrigger(t, nil)

	requireHiddenKeys(t, jsonKeys(t, trigger),
		[]string{"idWebhook"},
		[]string{"id", "tag", "conditions"},
	)
}

// TestModels_JSONKeysCamelCase las respuestas al frontend usan camelCase, ninguna llave puede tener "_"
func TestModels_JSONKeysCamelCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value any
	}{
		{"apis", models.ApisModelType{}},
		{"assets", models.AssetsModelType{}},
		{"catalogs", models.CatalogsModelType{}},
		{"conditions", models.ConditionsModelType{}},
		{"opportunities", models.OpportunitiesModelType{}},
		{"providers", models.ProvidersModelType{}},
		{"triggers", models.TriggersModelType{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for key := range jsonKeys(t, tt.value) {
				if strings.Contains(key, "_") {
					t.Errorf("la llave %q no esta en camelCase", key)
				}
			}
		})
	}
}
