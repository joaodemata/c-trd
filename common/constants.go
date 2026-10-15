package common

import (
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Valores hardcodeados de la aplicacion. Los status son una copia de la coleccion catalogs,
// si cambian en base de datos deben actualizarse aqui.

// CatalogStatus representa un status del catalogo (id, tag y nombre)
type CatalogStatus struct {
	ID   primitive.ObjectID
	Tag  string
	Name string
}

// GENERIC_STATUS (utilizados por las condiciones)
var (
	GenericStatusActive   = CatalogStatus{ID: mustParseObjectId("6a8423ce9687e916f29f233a"), Tag: "STATUS-ACTIVE", Name: "Activo"}
	GenericStatusInactive = CatalogStatus{ID: mustParseObjectId("6a84242c9687e916f29f233d"), Tag: "STATUS-INACTIVE", Name: "Inactivo"}
)

// OPPORTUNITY_STATUS
var (
	OpportunityStatusOngoing  = CatalogStatus{ID: mustParseObjectId("6a84257d9687e916f29f234e"), Tag: "OPPORTUNITY-ONGOING", Name: "En Curso"}
	OpportunityStatusActive   = CatalogStatus{ID: mustParseObjectId("6ab64e0904584b67c7c0ac59"), Tag: "OPPORTUNITY-ACTIVE", Name: "Activa"}
	OpportunityStatusFinished = CatalogStatus{ID: mustParseObjectId("6ab2c54804584b67c7c0ac33"), Tag: "OPPORTUNITY-FINISHED", Name: "Finalizada"}
	OpportunityStatusExpired  = CatalogStatus{ID: mustParseObjectId("6a84257d9687e916f29f234d"), Tag: "OPPORTUNITY-EXPIRED", Name: "Expirada"}
	OpportunityStatusCanceled = CatalogStatus{ID: mustParseObjectId("6a84257d9687e916f29f234c"), Tag: "OPPORTUNITY-CANCELED", Name: "Cancelada"}
)

// Base de datos
const (
	// Timeout de cada llamada a la base de datos
	DBTimeout = 5 * time.Second
	// Cantidad de operaciones por cada BulkWrite
	BulkWriteBatchSize = 500
	// Maximo de BulkWrites simultaneos, basado en el maxPoolSize de MongoDB y no en la CPU
	BulkWriteMaxWorkers = 20
)

// Cronjob de condiciones
const (
	// Maximo de condiciones evaluadas simultaneamente (llamadas a APIs externas)
	ConditionEvaluationWorkers = 10
	// Intervalo por defecto si la condicion no tiene interval_seconds
	DefaultConditionIntervalSeconds = 60
	// Tiempo maximo para evaluar todas las condiciones de un ciclo (el ticker corre cada minuto)
	ConditionEvaluationTimeout = 50 * time.Second
)

// Peticiones HTTP a proveedores externos
const (
	// Prefijo de las variables de entorno con secretos de proveedores, ej: JM_CTRD_SECRET_ALPHAVANTAGE_APIKEY
	ProviderSecretEnvPrefix = "JM_CTRD_SECRET_"
	// Timeout por defecto si la peticion no tiene timeout_seconds
	ProviderDefaultTimeout = 10 * time.Second
	// Maximo de reintentos permitidos por peticion
	ProviderMaxRetries = 5
	// Maxima espera entre reintentos
	ProviderMaxRetryDelay = 30 * time.Second
	// Tamaño maximo de la respuesta de un proveedor (10 MB)
	ProviderMaxResponseBytes = 10 << 20
)

// Archivos CSV de Google Trends subidos por asset
const (
	// Variable de entorno con la carpeta donde se guardan los CSV
	GoogleTrendsDirEnv = "JM_CTRD_GOOGLE_TRENDS_DIR"
	// Carpeta por defecto si la variable no existe (relativa al directorio de ejecucion)
	GoogleTrendsDefaultDir = "data/google_trends"
)

// mustParseObjectId convierte un hexadecimal hardcodeado en ObjectID, detiene la app si es invalido
func mustParseObjectId(hex string) primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(hex)
	if err != nil {
		log.Fatalf("ObjectID hardcodeado invalido %s: %v", hex, err)
	}
	return id
}
