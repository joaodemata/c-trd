package providers

// Algoritmos genericos que usan los callbacks. No conocen ningun proveedor: reciben series ya parseadas y
// retornan numeros, la decision (comparar contra un umbral, el signo de la pendiente, etc.) la toma cada callback

// seriesPoint punto (x, y) de una serie. X es la posicion en el eje (ej: dias desde la primera observacion)
type seriesPoint struct {
	X float64
	Y float64
}

// trendSlope pendiente de la regresion lineal por minimos cuadrados de los puntos. Positiva es tendencia al alza y
// negativa a la baja. Recibe al menos 2 puntos; si todos tienen el mismo X retorna 0
func trendSlope(points []seriesPoint) float64 {
	n := float64(len(points))
	var meanX, meanY float64
	for _, point := range points {
		meanX += point.X / n
		meanY += point.Y / n
	}

	var covariance, variance float64
	for _, point := range points {
		dx := point.X - meanX
		covariance += dx * (point.Y - meanY)
		variance += dx * dx
	}

	if variance == 0 {
		return 0
	}
	return covariance / variance
}

// seriesRange retorna el minimo y el maximo de la serie. Recibe al menos 1 valor
func seriesRange(values []float64) (float64, float64) {
	low, high := values[0], values[0]
	for _, value := range values {
		low = min(low, value)
		high = max(high, value)
	}
	return low, high
}

// rangeLevel retorna el valor que esta en la proporcion ratio del rango [low, high] (0 es low y 1 es high)
func rangeLevel(low, high, ratio float64) float64 {
	return low + (high-low)*ratio
}

// sharePercentage retorna el porcentaje que representa part sobre total. Recibe total mayor a 0
func sharePercentage(part, total float64) float64 {
	return part / total * 100
}
