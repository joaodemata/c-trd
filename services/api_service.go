package services

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)


func FetchDataService(fromCurrency string, toCurrency string) (string, error) {


	// 1. API URL
	baseURL := os.Getenv("JM_CTRD_STOCK_API_URL")

	// 2. Parsear la URL base
	urlParsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("error parsing the URL: %v", err)
	}

	// 3. Agregar los query params de forma segura
	// Esto codifica automáticamente espacios y caracteres especiales
	q := urlParsed.Query()
	q.Add("function", "CURRENCY_EXCHANGE_RATE")   // Ejemplo: ?ticker=AAPL
	q.Add("from_currency", fromCurrency) // Ejemplo: &category=stock
	q.Add("to_currency", toCurrency) // Ejemplo: &category=stock
	q.Add("apikey", os.Getenv("JM_CTRD_API_KEY"))

	// Reasignar los parámetros ya codificados a la URL
	urlParsed.RawQuery = q.Encode()

	urlFinal := urlParsed.String()
	fmt.Println("Doing request to:", urlFinal)

	// 4. Ejecutar la petición GET
	resp, err := http.Get(urlFinal)
	if err != nil {
		return "", fmt.Errorf("error requesting HTTP: %v", err)
	}
	

	// IMPORTANTE: Siempre debes cerrar el body de la respuesta para evitar fugas de memoria
	defer resp.Body.Close()

	// 5. Leer el cuerpo de la respuesta
	bodyBytes, err := io.ReadAll(resp.Body)

	fmt.Println(string(bodyBytes))

	if err != nil {
		return "", fmt.Errorf("error reading response: %v", err)
	}

	// Retornamos el JSON como un string (o podrías retornar los bytes directamente)
	return string(bodyBytes), nil
}
