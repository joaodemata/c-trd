package common

import (
	"encoding/json"
	"net/http"
	"time"
)

type APIResponse struct {
	Status   string    `json:"status"`
	Message  string    `json:"message"`
	Tracking string    `json:"tracking"`
	Date     time.Time `json:"date"`
	Data     any       `json:"data,omitempty"`
}

type ResponseHandler struct {
	w http.ResponseWriter
}

// NewResponseHandler inicializa el ayudante
func NewResponseHandler(w http.ResponseWriter) *ResponseHandler {
	return &ResponseHandler{w: w}
}

// Send es tu método "core" para despachar la respuesta
func (rh *ResponseHandler) Send(statusCode int, statusMsg string, message string, tracking string, data any) {
	// Seteamos el header de que es un JSON antes de escribir nada
	rh.w.Header().Set("Content-Type", "application/json")
	rh.w.WriteHeader(statusCode)

	// Construimos la estructura de la respuesta
	res := APIResponse{
		Status:   statusMsg,
		Message:  message,
		Tracking: tracking,
		Date:     time.Now(),
		Data:     data,
	}

	// Transformamos a JSON y escribimos directamente en el ResponseWriter
	json.NewEncoder(rh.w).Encode(res)
}

func (rh *ResponseHandler) Error(statusMsg string, message string, tracking string, data any, err *ErrorHandler) {
	// Seteamos el header de que es un JSON antes de escribir nada
	rh.w.Header().Set("Content-Type", "application/json")
	rh.w.WriteHeader(http.StatusBadRequest)

	// Por defecto se muestra el mensaje enviado. Los errores fatales y de base de datos se ocultan
	// porque su mensaje es interno y no debe llegar al cliente
	messageAPI := message
	if err.Level == LevelFatal || err.Level == LevelDatabase {
		messageAPI = "Servicio no disponible."
	}

	// Construimos la estructura de la respuesta
	res := APIResponse{
		Status:   statusMsg,
		Message:  messageAPI,
		Tracking: tracking,
		Date:     time.Now(),
		Data:     data,
	}

	// Transformamos a JSON y escribimos directamente en el ResponseWriter
	json.NewEncoder(rh.w).Encode(res)
}
