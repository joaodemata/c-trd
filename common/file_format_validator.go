package common

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
)

// Margen para los campos de texto y separadores del multipart ademas del archivo
const multipartOverheadBytes = 64 << 10

// FileUploadConfig configuracion del archivo que recibe una ruta multipart/form-data
type FileUploadConfig struct {
	Field      string   // Campo del multipart con el archivo, ej: "file"
	MaxBytes   int64    // Tamaño maximo del archivo
	Extensions []string // Extensiones permitidas, ej: []string{".csv"}
}

// UploadedFile archivo recibido, se guarda en el contexto con la llave "file"
type UploadedFile struct {
	Name    string
	Content []byte
}

// FileFormatValidateMiddleware valida una peticion multipart/form-data: el archivo segun cfg y los campos de
// texto de T (tags `form` y `validate`). Inyecta *T en "payload" y *UploadedFile en "file".
// T solo puede tener campos string con tag `form`, si no la app se detiene al registrar la ruta
func FileFormatValidateMiddleware[T any](cfg FileUploadConfig) func(http.Handler) http.Handler {
	formFields := multipartFormFields[T]()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			res := NewResponseHandler(w)
			maxMessage := fmt.Sprintf("Se espera un multipart/form-data con el archivo en el campo '%s' de maximo %d KB", cfg.Field, cfg.MaxBytes>>10)

			// Limitamos el body completo antes de leerlo
			r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxBytes+multipartOverheadBytes)
			if err := r.ParseMultipartForm(cfg.MaxBytes + multipartOverheadBytes); err != nil {
				res.Error("FAIL", maxMessage, "CMMFUE001", nil, NewEmptyErrorHandler())
				return
			}
			defer r.MultipartForm.RemoveAll()

			// 1. Archivo
			file, header, err := r.FormFile(cfg.Field)
			if err != nil {
				res.Error("FAIL", maxMessage, "CMMFUE002", nil, NewEmptyErrorHandler())
				return
			}
			defer file.Close()

			if !hasAllowedExtension(header.Filename, cfg.Extensions) {
				res.Error("FAIL", "Extension de archivo no permitida, se espera: "+strings.Join(cfg.Extensions, ", "), "CMMFUE003", nil, NewEmptyErrorHandler())
				return
			}

			content, err := io.ReadAll(io.LimitReader(file, cfg.MaxBytes+1))
			if err != nil {
				res.Error("FAIL", "Error leyendo el archivo", "CMMFUE004", nil, NewEmptyErrorHandler())
				return
			}
			if len(content) == 0 || int64(len(content)) > cfg.MaxBytes {
				res.Error("FAIL", maxMessage, "CMMFUE005", nil, NewEmptyErrorHandler())
				return
			}

			// 2. Campos de texto
			var payloadFormat T
			payloadValue := reflect.ValueOf(&payloadFormat).Elem()
			for index, name := range formFields {
				payloadValue.Field(index).SetString(r.FormValue(name))
			}

			if err := validate.Struct(payloadFormat); err != nil {
				res.Error("FAIL", "Los campos del formulario son invalidos", "CMMFUE006", formatErrorData(validationMessages(err)), NewEmptyErrorHandler())
				return
			}

			// 3. Inyectar en el contexto
			r = SetInRequestContext(r, "payload", &payloadFormat)
			r = SetInRequestContext(r, "file", &UploadedFile{Name: header.Filename, Content: content})

			next.ServeHTTP(w, r)
		})
	}
}

// multipartFormFields retorna el indice de cada campo de T con su nombre en el formulario (tag `form`)
func multipartFormFields[T any]() map[int]string {
	structType := reflect.TypeFor[T]()
	if structType.Kind() != reflect.Struct {
		panic(fmt.Errorf("FileFormatValidateMiddleware: %s must be a struct", structType))
	}

	fields := map[int]string{}
	for index := 0; index < structType.NumField(); index++ {
		field := structType.Field(index)
		name := field.Tag.Get("form")
		if name == "" {
			continue
		}
		if field.Type.Kind() != reflect.String || !field.IsExported() {
			panic(fmt.Errorf("FileFormatValidateMiddleware: field %s.%s with form tag must be an exported string", structType, field.Name))
		}
		fields[index] = name
	}

	return fields
}

// hasAllowedExtension compara la extension del nombre del archivo sin distinguir mayusculas
func hasAllowedExtension(fileName string, extensions []string) bool {
	extension := filepath.Ext(fileName)
	for _, allowed := range extensions {
		if strings.EqualFold(extension, allowed) {
			return true
		}
	}
	return false
}
