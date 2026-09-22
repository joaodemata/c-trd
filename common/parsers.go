package common

import "time"

//TODO: MEJORAR NOMBRE Y PROPOSITO DE ESTE ARCHIVO

// parseTimeframeTag convierte el tag de la vela (ej: "1H", "15M", "1D") a un time.Duration
func ParseTimeframeTag(tag string) time.Duration {
	switch tag {
	case "1m", "1M":
		return 1 * time.Minute
	case "5m", "5M":
		return 5 * time.Minute
	case "15m", "15M":
		return 15 * time.Minute
	case "30m", "30M":
		return 30 * time.Minute
	case "1h", "1H":
		return 1 * time.Hour
	case "4h", "4H":
		return 4 * time.Hour
	case "1d", "1D":
		return 24 * time.Hour
	case "1w", "1W":
		return 7 * 24 * time.Hour
	default:
		// Valor por defecto en caso de un tag desconocido (puedes ajustarlo o retornar error)
		return 1 * time.Hour 
	}
}