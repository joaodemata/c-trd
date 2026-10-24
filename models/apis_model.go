package models

import (
	"c_trd/providers"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// ApiParameterOption valor permitido de un parametro y el texto que muestra el frontend
type ApiParameterOption struct {
	Value string `bson:"value" json:"value" description:"Valor que se envia a la API"`
	Label string `bson:"label" json:"label" description:"Texto que se muestra al usuario"`
}

// ApiParameterOptions lista de valores permitidos de un parametro. Pueden estar guardados en la base de datos
// (STATIC) o salir de la respuesta de otra API de la coleccion apis (API)
type ApiParameterOptions struct {
	Source string `bson:"source" json:"source" description:"Origen de las opciones: STATIC o API"`
	// Solo STATIC
	Values []ApiParameterOption `bson:"values" json:"values" description:"Opciones guardadas en base de datos"`
	// Solo API: las rutas usan notacion de puntos sobre la respuesta (ej: data.symbols)
	IDApi     *primitive.ObjectID `bson:"id_api" json:"idApi" description:"API que lista las opciones"`
	ItemsPath string              `bson:"items_path" json:"itemsPath" description:"Ruta del arreglo de opciones en la respuesta"`
	ValuePath string              `bson:"value_path" json:"valuePath" description:"Ruta del valor dentro de cada elemento"`
	LabelPath string              `bson:"label_path" json:"labelPath" description:"Ruta del texto dentro de cada elemento"`
}

// ApiParameter parametro que acepta la API. La API solo define el parametro, el valor lo guarda cada condicion
// en su http_request, en el lugar indicado por el data_location de la API
type ApiParameter struct {
	Name         string               `bson:"name" json:"name" description:"Nombre con el que se envia a la API"`
	Label        string               `bson:"label" json:"label" description:"Nombre que se muestra al usuario"`
	Description  string               `bson:"description" json:"description" description:"Descripcion del parametro"`
	DataType     string               `bson:"data_type" json:"dataType" description:"STRING, INTEGER, FLOAT, BOOLEAN o DATE"`
	IsRequired   bool                 `bson:"is_required" json:"isRequired" description:"Si la condicion debe enviar el parametro"`
	DefaultValue *string              `bson:"default_value" json:"defaultValue" description:"Valor por defecto, null si no tiene"`
	Options      *ApiParameterOptions `bson:"options" json:"options" description:"Valores permitidos, null si el valor es libre"`
}

type ApisModelType struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	IDProvider   primitive.ObjectID `bson:"id_provider,omitempty" json:"idProvider,omitempty"`
	TagProvider  string             `bson:"tag_provider" json:"tagProvider" description:"Tag del proveedor de la API."`
	NameProvider string             `bson:"name_provider" json:"nameProvider" description:"Nombre del proveedor de la API."`

	Tag          string            `bson:"tag" json:"tag" description:"Tag de la API."`
	Name         string            `bson:"name" json:"name" description:"Nombre de la API."`
	Description  string            `bson:"description" json:"description" description:"Descripcion de la API."`
	IsActive     bool              `bson:"is_active" json:"isActive" description:"Si esta activo la API."`
	Type         string            `bson:"type" json:"type" description:"Tipo de api (interna o externa)"`
	Method       string            `bson:"method" json:"method" description:"metodo de la peticion a Llamar"`
	URL          string            `bson:"url" json:"url" description:"url a Llamar"`
	Headers      map[string]string `bson:"headers" json:"headers" description:"cabeceras fijas de la peticion"`
	DataLocation string            `bson:"data_location" json:"dataLocation" description:"Por donde se envian los parametros: QUERY o BODY"`
	Data         interface{}       `bson:"data" json:"data" description:"data que siempre se envia por esta api (Ej: function, id, etc)"`

	// Configuracion de la peticion, se copia al http_request de cada condicion
	Auth           *providers.HTTPAuth    `bson:"auth" json:"auth" description:"Autorizacion, el secreto sale del secret_ref del proveedor. null si no requiere"`
	RetryPolicy    *providers.RetryPolicy `bson:"retry_policy" json:"retryPolicy" description:"Reintentos en caso de fallo, null si no reintenta"`
	TimeoutSeconds int                    `bson:"timeout_seconds" json:"timeoutSeconds" description:"Tiempo maximo de espera en segundos, 0 usa el valor por defecto"`

	// Formato de la respuesta, define que callbacks pueden procesarla
	ResponseFormat string `bson:"response_format" json:"responseFormat" description:"Formato de la respuesta (ej: GOOGLE_TRENDS_SERIES)"`
	// Parametros que acepta la API, se guarda [] si no tiene
	Parameters []ApiParameter `bson:"parameters" json:"parameters" description:"Parametros que la condicion puede personalizar"`

	// Comunes
	LastUpdatedAt time.Time `bson:"last_updated_at" json:"lastUpdatedAt" description:"Ultima Actualizacion"`
	CreatedDate   time.Time `bson:"created_date" json:"createdDate" description:"Fecha de Creacion"`
	LogicalDelete bool      `bson:"logical_delete" json:"-" description:"Borrado Logico"`
	CoreDB        string    `bson:"core_db" json:"-" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string    `bson:"core_version" json:"-" description:"Version del Core"`
	UserDB        string    `bson:"user_db" json:"-" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string    `bson:"schema_db" json:"-" description:"Versionado de la Base de datos."`
}

// Model pointer
var ApisModel *mongo.Collection
