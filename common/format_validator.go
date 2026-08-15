package common

import (
	"context"
	"encoding/json" // Importamos el paquete de errores estándar
	"errors"
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// Inicializamos el validador a nivel global (es thread-safe y optimiza el rendimiento)
var validate *validator.Validate

func init() {
	validate = validator.New()
}

// Validador de si hubo errores y que tipo fue para devolverlo
func FormatValidateMiddleware[T any](next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Instaciamos copia del datos para no realizar race conditions
		var payloadFormat T
		
		err := json.NewDecoder(r.Body).Decode(&payloadFormat)

		if err != nil {
			// Verificar si el error es de tipo de dato erroneo
			var typeError *json.UnmarshalTypeError
			
			if errors.As(err, &typeError) {
				// Mismatch data type
				message := fmt.Sprintf("Error de tipo en el campo '%s': se esperaba %s pero se recibió un %s", 
					typeError.Field, typeError.Type.String(), typeError.Value)
				
				http.Error(w, message, http.StatusBadRequest)
				return
			}

			// No es formato JSON
			http.Error(w, "El JSON está mal formateado o es inválido estructuralmente", http.StatusBadRequest)
			return
		}
		// Validammos la estructura del JSON en base a las reglas enviadas
		err = validate.Struct(payloadFormat)

		if err != nil {
			// Si la validación falla, extraemos los errores para que el cliente sepa qué pasó
			var errors []string
			for _, err := range err.(validator.ValidationErrors) {
				errorMessage := fmt.Sprintf("El campo '%s' falló en la regla: '%s'", err.Field(), err.Tag())
				errors = append(errors, errorMessage)
			}

			// Devolvemos un HTTP 400 (Bad Request) con el array de errores
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":    "Datos de entrada inválidos",
				"details": errors,
			})
			return
		}

		// 3. Inyectar en el contexto
		ctx := context.WithValue(r.Context(), "payload", &payloadFormat)

		// 4. Continuar al controlador
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
