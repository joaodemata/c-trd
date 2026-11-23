package providers

import (
	"math"
	"time"
)

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

// optionalTime parsea la fecha del dato que trae el proveedor con el layout indicado y la retorna en UTC.
// Retorna nil si viene vacia o no se puede parsear: la fecha es informativa y no debe hacer fallar la evaluacion
func optionalTime(layout, text string) *time.Time {
	if text == "" {
		return nil
	}

	parsed, err := time.Parse(layout, text)
	if err != nil {
		return nil
	}

	utc := parsed.UTC()
	return &utc
}

// linearRegression pendiente y coeficiente de determinacion (R², de 0 a 1) de la regresion lineal por minimos
// cuadrados. Si todos los puntos tienen el mismo X o el mismo Y no hay ajuste que medir y retorna R² 0
func linearRegression(points []seriesPoint) (float64, float64) {
	n := float64(len(points))
	var meanX, meanY float64
	for _, point := range points {
		meanX += point.X / n
		meanY += point.Y / n
	}

	var covariance, varianceX, varianceY float64
	for _, point := range points {
		dx := point.X - meanX
		dy := point.Y - meanY
		covariance += dx * dy
		varianceX += dx * dx
		varianceY += dy * dy
	}

	if varianceX == 0 {
		return 0, 0
	}
	slope := covariance / varianceX
	if varianceY == 0 {
		return slope, 0
	}
	rSquared := covariance * covariance / (varianceX * varianceY)
	return slope, rSquared
}

// percentileRank porcentaje (0 a 100) de valores de reference que estan por debajo de value; los iguales cuentan la
// mitad. Un valor mayor a todos es 100 y uno menor a todos es 0. Recibe al menos 1 valor de referencia
func percentileRank(reference []float64, value float64) float64 {
	var below, equal float64
	for _, item := range reference {
		if item < value {
			below++
		}
		if item == value {
			equal++
		}
	}
	return (below + equal/2) / float64(len(reference)) * 100
}

// meanAndStdDev media y desviacion estandar muestral (n - 1) de los valores. Recibe al menos 2 valores
func meanAndStdDev(values []float64) (float64, float64) {
	n := float64(len(values))
	var mean float64
	for _, value := range values {
		mean += value / n
	}

	var squares float64
	for _, value := range values {
		squares += (value - mean) * (value - mean)
	}
	return mean, math.Sqrt(squares / (n - 1))
}
