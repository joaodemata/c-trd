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
)

// mustParseObjectId convierte un hexadecimal hardcodeado en ObjectID, detiene la app si es invalido
func mustParseObjectId(hex string) primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(hex)
	if err != nil {
		log.Fatalf("ObjectID hardcodeado invalido %s: %v", hex, err)
	}
	return id
}
