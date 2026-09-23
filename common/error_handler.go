package common

import (
	"fmt"
	"log/slog"
)

// ErrorLevel define la severidad del error.
type ErrorLevel string

const (
	LevelInfo     ErrorLevel = "INFO"
	LevelWarning  ErrorLevel = "WARNING"
	LevelError    ErrorLevel = "ERROR"
	LevelFatal    ErrorLevel = "FATAL"
	LevelDatabase ErrorLevel = "DATABASE"
	LevelEmpty    ErrorLevel = "EMPTY"
)

// AppError es nuestra estructura personalizada.
type ErrorHandler struct {
	Err          error      // El error original (underlying error)
	Message      string     // Mensaje amigable o de contexto
	Level        ErrorLevel // Nivel de severidad
	TrackingCode string     // Código para rastrear dónde ocurrió
}

// Método Error() para implementar la interfaz 'error' de Go.
func (e *ErrorHandler) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s | Reason: %v | Track: %s", e.Level, e.Message, e.Err, e.TrackingCode)
	}
	return fmt.Sprintf("[%s] %s | Track: %s", e.Level, e.Message, e.TrackingCode)
}

// Método Unwrap() para soportar errors.Is y errors.As (Go 1.13+).
func (e *ErrorHandler) Unwrap() error {
	return e.Err
}

// Valida si el error no es vacio
func (e *ErrorHandler) IsEmtpy() bool {
	if e.Level == LevelEmpty {
		return true
	}

	return false
}

// Constructor para facilitar la creación del error.
func NewErrorHandler(err error, msg string, level ErrorLevel, trackCode string) *ErrorHandler {
	//
	slog.Error(msg, "TRACKING CODE: "+trackCode, err)

	return &ErrorHandler{
		Err:          err,
		Message:      msg,
		Level:        level,
		TrackingCode: trackCode,
	}
}

func NewServerErrorHandler(trackingCode string) *ErrorHandler {
	return &ErrorHandler{
		Message:      "Servicio no disponible",
		Level:        LevelEmpty,
		TrackingCode: trackingCode,
	}
}

func NewEmptyErrorHandler() *ErrorHandler {
	return &ErrorHandler{
		Message:      "",
		Level:        LevelEmpty,
		TrackingCode: "",
	}
}

// func main() {
// 	// Simulamos un error de base de datos
// 	dbErr := errors.New("connection timeout")

// 	// Envolvemos el error con nuestro struct personalizado
// 	myErr := NewAppError(
// 		dbErr,
// 		"No se pudo conectar a la base de datos de usuarios",
// 		LevelError,
// 		"DB_CONN_001",
// 	)

// 	fmt.Println(myErr.Error())

// 	// Comprobamos si el error original era 'dbErr' usando Unwrap (automático con errors.Is)
// 	if errors.Is(myErr, dbErr) {
// 		fmt.Println("-> El sistema detectó que la causa raíz fue un timeout de DB.")
// 	}
// }
