package common

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TODO: MEJORAR NOMBRE Y PROPOSITO DE ESTE ARCHIVO

// Duracion de cada vela segun su tag. Son los tags de type_catalog TYPE_CANDLE_STICK en la coleccion catalogs,
// si cambian en base de datos deben actualizarse aqui
var timeframeDurations = map[string]time.Duration{
	"1-MINUTE":   1 * time.Minute,
	"5-MINUTES":  5 * time.Minute,
	"15-MINUTES": 15 * time.Minute,
	"30-MINUTES": 30 * time.Minute,
	"1-HOUR":     1 * time.Hour,
	"2-HOURS":    2 * time.Hour,
	"4-HOURS":    4 * time.Hour,
	"1-DAY":      24 * time.Hour,
	"1-WEEK":     7 * 24 * time.Hour,
	// Un mes no tiene duracion fija, se aproxima a 30 dias
	"1-MONTH": 30 * 24 * time.Hour,
}

// parseTimeframeTag convierte el tag de la vela (ej: "1-HOUR", "15-MINUTES", "1-DAY") a un time.Duration
func ParseTimeframeTag(tag string) time.Duration {
	if duration, ok := timeframeDurations[tag]; ok {
		return duration
	}
	// Valor por defecto en caso de un tag desconocido (puedes ajustarlo o retornar error)
	return 1 * time.Hour
}

// IsKnownTimeframeTag indica si el tag de la vela tiene una duracion conocida
func IsKnownTimeframeTag(tag string) bool {
	_, ok := timeframeDurations[tag]
	return ok
}

// Convierte un hexadecimal en un objectId
func ParseHexToObjectId(id string) (primitive.ObjectID, *ErrorHandler) {
	// Parse
	objectId, err := primitive.ObjectIDFromHex(id)
	// Check error
	if err != nil {
		return objectId, NewErrorHandler(err, err.Error(), LevelFatal, "CMMPE001")
	}

	return objectId, NewEmptyErrorHandler()
}
