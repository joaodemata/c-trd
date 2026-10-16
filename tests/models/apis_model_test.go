package models_test

import (
	"testing"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/providers"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestApisModel_BsonRoundTrip el modelo debe poder guardarse y leerse de Mongo. Antes tag_provider,
// name_provider y tag compartian la llave "tag" y el driver fallaba con llave duplicada
func TestApisModel_BsonRoundTrip(t *testing.T) {
	t.Parallel()

	idOptionsApi := primitive.NewObjectID()
	interval := "daily"
	api := models.ApisModelType{
		TagProvider:    "ALPHA-VANTAGE",
		NameProvider:   "Alpha Vantage",
		Tag:            "AV-PUT-CALL-RATIO",
		Name:           "Put/Call ratio",
		DataLocation:   cmm.ApiDataLocationQuery,
		Auth:           &providers.HTTPAuth{Type: "API_KEY", Key: "apikey", In: "QUERY"},
		TimeoutSeconds: 15,
		ResponseFormat: cmm.ResponseFormatAlphaVantagePutCallRatio,
		Parameters: []models.ApiParameter{
			{
				Name: "symbol", DataType: cmm.ApiParameterTypeString, IsRequired: true,
				Options: &models.ApiParameterOptions{Source: cmm.ApiParameterOptionsApi, IDApi: &idOptionsApi, ItemsPath: "data", ValuePath: "symbol", LabelPath: "name"},
			},
			{
				Name: "interval", DataType: cmm.ApiParameterTypeString, DefaultValue: &interval,
				Options: &models.ApiParameterOptions{Source: cmm.ApiParameterOptionsStatic, Values: []models.ApiParameterOption{{Value: "daily", Label: "Diario"}}},
			},
			{Name: "limit", DataType: cmm.ApiParameterTypeInteger},
		},
	}

	doc, err := bson.Marshal(api)
	if err != nil {
		t.Fatalf("bson.Marshal: %v", err)
	}
	raw := bson.Raw(doc)

	for key, want := range map[string]string{"tag": api.Tag, "tag_provider": api.TagProvider, "name_provider": api.NameProvider, "response_format": api.ResponseFormat} {
		if got := raw.Lookup(key).StringValue(); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}

	// Sin politica de reintentos se guarda null
	if got := raw.Lookup("retry_policy").Type; got != bsontype.Null {
		t.Errorf("retry_policy es %v, se esperaba null", got)
	}

	// Un parametro libre y sin valor por defecto guarda options y default_value como null
	values, _ := raw.Lookup("parameters").Array().Values()
	if len(values) != 3 {
		t.Fatalf("parameters tiene %d elementos, se esperaban 3", len(values))
	}
	free := values[2].Document()
	for _, key := range []string{"options", "default_value"} {
		if got := free.Lookup(key).Type; got != bsontype.Null {
			t.Errorf("parameters[2].%s es %v, se esperaba null", key, got)
		}
	}

	var decoded models.ApisModelType
	if err := bson.Unmarshal(doc, &decoded); err != nil {
		t.Fatalf("bson.Unmarshal: %v", err)
	}
	if decoded.Auth == nil || decoded.Auth.Key != "apikey" || decoded.TimeoutSeconds != 15 {
		t.Errorf("auth = %+v, timeout_seconds = %d, se esperaba la configuracion de la peticion", decoded.Auth, decoded.TimeoutSeconds)
	}
	symbol := decoded.Parameters[0]
	if symbol.Options == nil || symbol.Options.IDApi == nil || *symbol.Options.IDApi != idOptionsApi || symbol.Options.ValuePath != "symbol" {
		t.Errorf("parameters[0].options = %+v, se esperaban las opciones por API", symbol.Options)
	}
	daily := decoded.Parameters[1]
	if daily.DefaultValue == nil || *daily.DefaultValue != interval || len(daily.Options.Values) != 1 {
		t.Errorf("parameters[1] = %+v, se esperaban las opciones STATIC y el valor por defecto", daily)
	}
}
