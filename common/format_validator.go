package common

import (
	"bytes"
	"encoding/json" // Importamos el paquete de errores estándar
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Inicializamos el validador a nivel global (es thread-safe y optimiza el rendimiento)
var validate *validator.Validate

// Mensaje general de los errores de formato
const invalidInputMessage = "Datos de entrada inválidos"

// Origen de cada campo de un format validate, se define con el tag `in`. Sin tag el campo viene del body
const (
	FieldInBody  = "body"  // JSON del body
	FieldInQuery = "query" // query params, ej: ?limit=10&page=2. Un arreglo se envia repitiendo el parametro
)

// Formatos aceptados para los campos time.Time que vienen por query
var queryTimeLayouts = []string{time.RFC3339, time.DateOnly}

// Solo mayusculas, numeros y guion bajo (ej: GOOGLE_TRENDS_SERIES)
var upperSnakePattern = regexp.MustCompile(`^[A-Z0-9_]+$`)

// Numero con el formato de JSON (ej: 900, -1.5, 2e3)
var jsonNumberPattern = regexp.MustCompile(`^-?(0|[1-9]\d*)(\.\d+)?([eE][+-]?\d+)?$`)

// Tipos que se decodifican a si mismos (time.Time, ObjectID, json.RawMessage), su valor no se modifica
var jsonUnmarshalerType = reflect.TypeFor[json.Unmarshaler]()

func init() {
	validate = validator.New()

	// Los errores usan el nombre del campo en el JSON (o en el formulario), que es el que conoce el cliente
	validate.RegisterTagNameFunc(clientFieldName)

	// Registramos las etiquetas personalizadas
	_ = validate.RegisterValidation("mongodb", validateMongoDBID)
	_ = validate.RegisterValidation("uppersnake", validateUpperSnake)
}

// validateMongoDBID es la función interna que ejecuta la lógica de validación
func validateMongoDBID(fl validator.FieldLevel) bool {
	idStr := fl.Field().String()

	// primitive.ObjectIDFromHex intenta convertir el string.
	// Si da error, significa que no es un ID válido de Mongo.
	_, err := primitive.ObjectIDFromHex(idStr)
	return err == nil
}

// validateUpperSnake valida textos como los response_format: mayusculas, numeros y guion bajo
func validateUpperSnake(fl validator.FieldLevel) bool {
	return upperSnakePattern.MatchString(fl.Field().String())
}

// clientFieldName nombre del campo en el tag json o form, si no tiene se usa el nombre en Go
func clientFieldName(field reflect.StructField) string {
	for _, tag := range []string{"json", "form"} {
		name := strings.Split(field.Tag.Get(tag), ",")[0]
		if name == "-" {
			return ""
		}
		if name != "" {
			return name
		}
	}
	return field.Name
}

// FormatValidateMiddleware valida el formato de la peticion con las reglas `validate` de T. Cada campo indica su origen
// con el tag `in`: sin tag (o in:"body") se lee del JSON del body y con in:"query" de los query params, usando el nombre
// del tag json. Un campo de query nunca se toma del body. Inyecta *T en "payload".
// Los campos de query solo pueden ser textos, numeros, booleanos, time.Time o arreglos de estos, si no la app se
// detiene al registrar la ruta
func FormatValidateMiddleware[T any](next http.Handler) http.Handler {
	queryFields, hasBodyFields := formatFields[T]()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Creamos el response
		res := NewResponseHandler(w)

		// Instaciamos copia del datos para no realizar race conditions
		var payloadFormat T

		// 1. Body: solo se lee si T tiene campos que vienen del body
		if hasBodyFields {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				res.Error("FAIL", "No se pudo leer el body de la petición", "CC005", nil, NewEmptyErrorHandler())
				return
			}

			// Los numeros y booleanos enviados como texto ("900", "true") se aceptan en los campos de ese tipo
			body = numericTextsToNumbers(body, reflect.TypeFor[T]())
			err = json.NewDecoder(bytes.NewReader(body)).Decode(&payloadFormat)

			// Si el body esta vacio continuamos a la validación del Struct con los valores por defecto (vacíos)
			if err != nil && !errors.Is(err, io.EOF) {
				// Verificar si el error es de tipo de dato erroneo
				var typeError *json.UnmarshalTypeError
				if errors.As(err, &typeError) {
					message := typeErrorMessage(typeError)
					res.Error("FAIL", message, "CC001", formatErrorData(map[string]string{typeError.Field: message}), NewEmptyErrorHandler())
					return
				}

				// No es formato JSON
				res.Error("FAIL", "El JSON está mal formateado o es inválido estructuralmente", "CC002", nil, NewEmptyErrorHandler())
				return
			}
		}

		// 2. Query params: se descarta lo que el body haya enviado en esos campos
		payloadValue := reflect.ValueOf(&payloadFormat).Elem()
		query := r.URL.Query()
		for index, name := range queryFields {
			field := payloadValue.Field(index)
			field.SetZero()

			values, sent := query[name]
			if !sent {
				continue
			}
			if received, ok := setQueryValue(field, values); !ok {
				message := fmt.Sprintf("Error de tipo en el parámetro '%s': se esperaba %s pero se recibió '%s'", name, expectedTypeName(field.Type()), received)
				res.Error("FAIL", message, "CC004", formatErrorData(map[string]string{name: message}), NewEmptyErrorHandler())
				return
			}
		}

		// 3. Validamos la estructura en base a las reglas enviadas
		if err := validate.Struct(payloadFormat); err != nil {
			res.Error("FAIL", "Los datos enviados no cumplen el formato esperado", "CC003", formatErrorData(validationMessages(err)), NewEmptyErrorHandler())
			return
		}

		// 4. Inyectar en el contexto
		r = SetInRequestContext(r, "payload", &payloadFormat)

		// 5. Continuar al data validate
		next.ServeHTTP(w, r)
	})
}

// formatFields retorna los campos de T que vienen por query (indice y nombre) y si T tiene campos que vienen del body
func formatFields[T any]() (map[int]string, bool) {
	structType := reflect.TypeFor[T]()
	if structType.Kind() != reflect.Struct {
		panic(fmt.Errorf("FormatValidateMiddleware: %s must be a struct", structType))
	}

	queryFields := map[int]string{}
	hasBodyFields := false
	for index := 0; index < structType.NumField(); index++ {
		field := structType.Field(index)
		if !field.IsExported() {
			continue
		}

		switch field.Tag.Get("in") {
		case "", FieldInBody:
			hasBodyFields = true
		case FieldInQuery:
			name := clientFieldName(field)
			if name == "" || !isQueryType(field.Type) {
				panic(fmt.Errorf("FormatValidateMiddleware: query field %s.%s needs a json name and a string, number, bool, time.Time or slice type", structType, field.Name))
			}
			queryFields[index] = name
		default:
			panic(fmt.Errorf("FormatValidateMiddleware: field %s.%s has an unsupported in tag %q", structType, field.Name, field.Tag.Get("in")))
		}
	}

	return queryFields, hasBodyFields
}

// isQueryType tipos que se pueden convertir desde el texto de un query param
func isQueryType(fieldType reflect.Type) bool {
	if fieldType.Kind() == reflect.Slice {
		fieldType = fieldType.Elem()
	}
	if fieldType == reflect.TypeFor[time.Time]() {
		return true
	}

	switch fieldType.Kind() {
	case reflect.String, reflect.Bool, reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return false
}

// setQueryValue convierte los valores del query param al tipo del campo. Un arreglo toma todos los valores y los demas
// tipos el primero. Si falla retorna el valor que no se pudo convertir
func setQueryValue(field reflect.Value, values []string) (string, bool) {
	if field.Kind() == reflect.Slice {
		slice := reflect.MakeSlice(field.Type(), len(values), len(values))
		for i, value := range values {
			if !setQueryScalar(slice.Index(i), value) {
				return value, false
			}
		}
		field.Set(slice)
		return "", true
	}

	if !setQueryScalar(field, values[0]) {
		return values[0], false
	}
	return "", true
}

// setQueryScalar convierte un texto al tipo simple del campo
func setQueryScalar(field reflect.Value, value string) bool {
	if field.Type() == reflect.TypeFor[time.Time]() {
		for _, layout := range queryTimeLayouts {
			if parsed, err := time.Parse(layout, value); err == nil {
				field.Set(reflect.ValueOf(parsed))
				return true
			}
		}
		return false
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return false
		}
		field.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(value, 10, field.Type().Bits())
		if err != nil {
			return false
		}
		field.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(value, 10, field.Type().Bits())
		if err != nil {
			return false
		}
		field.SetUint(parsed)
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(value, field.Type().Bits())
		if err != nil {
			return false
		}
		field.SetFloat(parsed)
	default:
		return false
	}
	return true
}

// numericTextsToNumbers cambia los textos con un numero ("900") por el numero (900) y los textos "true" o "false" por
// el booleano cuando el campo de destino en targetType es de ese tipo, incluso dentro de objetos, arreglos y mapas. Si el body no es un JSON valido se retorna sin
// cambios para que el decoder reporte el error
func numericTextsToNumbers(body []byte, targetType reflect.Type) []byte {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber() // Mantiene el texto exacto de los numeros, sin pasarlos por float64

	var value any
	if err := decoder.Decode(&value); err != nil {
		return body
	}

	converted, err := json.Marshal(numericTextsInValue(value, targetType))
	if err != nil {
		return body
	}
	return converted
}

// numericTextsInValue recorre el valor del JSON junto con el tipo de destino y convierte los textos numericos y booleanos
func numericTextsInValue(value any, targetType reflect.Type) any {
	for targetType.Kind() == reflect.Pointer {
		targetType = targetType.Elem()
	}
	if targetType.Implements(jsonUnmarshalerType) || reflect.PointerTo(targetType).Implements(jsonUnmarshalerType) {
		return value
	}

	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if isNumberKind(targetType.Kind()) && jsonNumberPattern.MatchString(text) {
			return json.Number(text)
		}
		if targetType.Kind() == reflect.Bool && (text == "true" || text == "false") {
			return text == "true"
		}
	case []any:
		if targetType.Kind() == reflect.Slice || targetType.Kind() == reflect.Array {
			for index, item := range typed {
				typed[index] = numericTextsInValue(item, targetType.Elem())
			}
		}
	case map[string]any:
		for key, item := range typed {
			switch targetType.Kind() {
			case reflect.Map:
				typed[key] = numericTextsInValue(item, targetType.Elem())
			case reflect.Struct:
				if fieldType, ok := jsonFieldType(targetType, key); ok {
					typed[key] = numericTextsInValue(item, fieldType)
				}
			}
		}
	}
	return value
}

// isNumberKind tipos enteros o decimales
func isNumberKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return false
}

// jsonFieldType tipo del campo del struct que recibe la llave del JSON. Igual que encoding/json, primero busca el
// nombre exacto, luego sin distinguir mayusculas, y revisa los structs embebidos
func jsonFieldType(structType reflect.Type, key string) (reflect.Type, bool) {
	var foldedType reflect.Type
	for index := 0; index < structType.NumField(); index++ {
		field := structType.Field(index)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}

		if field.Anonymous && name == "" {
			embeddedType := field.Type
			if embeddedType.Kind() == reflect.Pointer {
				embeddedType = embeddedType.Elem()
			}
			if embeddedType.Kind() == reflect.Struct {
				if fieldType, ok := jsonFieldType(embeddedType, key); ok {
					return fieldType, true
				}
			}
			continue
		}
		if !field.IsExported() {
			continue
		}

		if name == "" {
			name = field.Name
		}
		if name == key {
			return field.Type, true
		}
		if foldedType == nil && strings.EqualFold(name, key) {
			foldedType = field.Type
		}
	}
	return foldedType, foldedType != nil
}

// formatErrorData arma el data de un error de formato: cada campo con su mensaje ({"campo": {"details": "..."}}),
// la lista de todos los mensajes en "details" y el mensaje general en "error". Los campos sin nombre solo van en la lista
func formatErrorData(fieldMessages map[string]string) map[string]any {
	data := map[string]any{}
	details := make([]string, 0, len(fieldMessages))

	for field, message := range fieldMessages {
		if field != "" {
			data[field] = map[string]string{"details": message}
		}
		details = append(details, message)
	}

	// Mismo orden en cada respuesta, el mapa no garantiza el orden
	slices.Sort(details)
	data["details"] = details
	data["error"] = invalidInputMessage
	return data
}

// validationMessages convierte los errores del validador en un mensaje en español por campo
func validationMessages(err error) map[string]string {
	messages := map[string]string{}

	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) {
		messages[""] = invalidInputMessage
		return messages
	}

	for _, fieldError := range validationErrors {
		field := fieldPath(fieldError)
		messages[field] = validationMessage(field, fieldError)
	}
	return messages
}

// fieldPath ruta del campo sin el nombre del struct raiz, ej: "conditions[0]" o "parameters"
func fieldPath(fieldError validator.FieldError) string {
	namespace := fieldError.Namespace()
	if index := strings.Index(namespace, "."); index >= 0 {
		return namespace[index+1:]
	}
	return fieldError.Field()
}

// validationMessage mensaje especifico de cada regla del validador
func validationMessage(field string, fieldError validator.FieldError) string {
	param := fieldError.Param()

	switch fieldError.Tag() {
	case "required", "required_if", "required_unless", "required_with", "required_without":
		return fmt.Sprintf("El campo '%s' es obligatorio", field)
	case "isdefault", "excluded_if", "excluded_unless", "excluded_with", "excluded_without":
		if fieldError.Kind() == reflect.Bool {
			return fmt.Sprintf("El campo '%s' debe ser false", field)
		}
		return fmt.Sprintf("El campo '%s' no debe enviarse", field)
	case "mongodb":
		return fmt.Sprintf("El campo '%s' debe ser un ObjectID válido (24 caracteres hexadecimales)", field)
	case "uppersnake":
		return fmt.Sprintf("El campo '%s' solo puede tener letras mayúsculas, números y guion bajo", field)
	case "unique":
		return fmt.Sprintf("El campo '%s' no puede tener valores repetidos", field)
	case "oneof":
		return fmt.Sprintf("El campo '%s' debe ser uno de: %s", field, strings.Join(strings.Fields(param), ", "))
	case "email":
		return fmt.Sprintf("El campo '%s' debe ser un correo electrónico válido", field)
	case "url", "http_url":
		return fmt.Sprintf("El campo '%s' debe ser una URL válida", field)
	case "datetime":
		return fmt.Sprintf("El campo '%s' debe tener el formato de fecha %s", field, param)
	case "gt", "gte", "lt", "lte", "min", "max", "len", "eq", "ne":
		return sizeMessage(field, fieldError.Tag(), param, fieldError.Kind())
	}

	return fmt.Sprintf("El campo '%s' no cumple la regla '%s'", field, fieldError.Tag())
}

// sizeMessage mensajes de las reglas de comparacion. En numeros se compara el valor, en textos la cantidad
// de caracteres y en arreglos u objetos la cantidad de elementos
func sizeMessage(field string, tag string, param string, kind reflect.Kind) string {
	comparisons := map[string]string{
		"gt": "mayor a", "gte": "mayor o igual a", "min": "mayor o igual a",
		"lt": "menor a", "lte": "menor o igual a", "max": "menor o igual a",
		"len": "igual a", "eq": "igual a", "ne": "distinto de",
	}

	switch kind {
	case reflect.String:
		if tag == "eq" || tag == "ne" {
			return fmt.Sprintf("El campo '%s' debe ser %s '%s'", field, comparisons[tag], param)
		}
		return fmt.Sprintf("El campo '%s' debe tener una cantidad de caracteres %s %s", field, comparisons[tag], param)
	case reflect.Slice, reflect.Array, reflect.Map:
		return fmt.Sprintf("El campo '%s' debe tener una cantidad de elementos %s %s", field, comparisons[tag], param)
	}

	return fmt.Sprintf("El campo '%s' debe ser %s %s", field, comparisons[tag], param)
}

// expectedTypeName nombre en español del tipo de dato esperado, ej: "un número entero"
func expectedTypeName(fieldType reflect.Type) string {
	if fieldType == reflect.TypeFor[time.Time]() {
		return "una fecha (YYYY-MM-DD o RFC3339)"
	}

	switch fieldType.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "un número entero"
	case reflect.Float32, reflect.Float64:
		return "un número"
	case reflect.String:
		return "un texto"
	case reflect.Bool:
		return "un booleano (true o false)"
	case reflect.Slice, reflect.Array:
		return "un arreglo"
	case reflect.Map, reflect.Struct:
		return "un objeto"
	}
	return "otro tipo de dato"
}

// typeErrorMessage mensaje de un campo con tipo de dato incorrecto, ej: se esperaba un número entero pero se recibió un texto
func typeErrorMessage(typeError *json.UnmarshalTypeError) string {
	expected := expectedTypeName(typeError.Type)

	// El JSON indica lo recibido como "string", "number", "number 1.5", "bool", "array" u "object"
	received := map[string]string{"string": "un texto", "bool": "un booleano", "array": "un arreglo", "object": "un objeto"}[typeError.Value]
	if strings.HasPrefix(typeError.Value, "number") {
		received = "un número"
		if value := strings.TrimSpace(strings.TrimPrefix(typeError.Value, "number")); value != "" {
			received = "el número " + value
		}
	}
	if received == "" {
		received = typeError.Value
	}

	if typeError.Field == "" {
		return fmt.Sprintf("Se esperaba %s pero se recibió %s", expected, received)
	}
	return fmt.Sprintf("Error de tipo en el campo '%s': se esperaba %s pero se recibió %s", typeError.Field, expected, received)
}
