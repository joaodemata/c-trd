package common

import (
	"context"
	"fmt"
	"net/http"
)

// Para evitar colisiones con otros paquetes que toquen el context
type ContextKey string

// SetInRequestContext recibe T (puede ser un struct normal o un puntero)
func SetInRequestContext(r *http.Request, key string, value any) *http.Request {
    ctx := context.WithValue(r.Context(), ContextKey(key), value)
    return r.WithContext(ctx)
}

// GetFromRequestContext devuelve T (puede ser un struct normal o un puntero)
func GetFromRequestContext[T any](r *http.Request, key string) (T, *ErrorHandler) {
    ctxKey := ContextKey(key)
    
    val, ok := r.Context().Value(ctxKey).(T)

    if !ok {
        var zero T 
        var err error =  fmt.Errorf("no se encontro la data o el tipo es incorrecto para la llave: %s", key)
        
        return zero,  NewErrorHandler(err, err.Error(), LevelFatal, "CMMREH001")
    }

    return val, NewEmptyErrorHandler()
}