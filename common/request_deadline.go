package common

import (
	"errors"
	"net/http"
	"time"
)

// RequestDeadlineMiddleware amplia los tiempos de lectura y escritura de la conexion solo para la ruta donde se usa,
// sin cambiar los 10s del servidor para el resto. Se usa en las subidas de archivos grandes (CSV de backtest): el
// ReadTimeout del servidor corta el body a mitad de la subida y el WriteTimeout cuenta desde que se leyeron los
// headers, asi que la subida tambien lo consume. Va despues del rate limit y antes de leer el body.
// Si la conexion no permite cambiar los tiempos (ej: httptest.ResponseRecorder) sigue con los del servidor
func RequestDeadlineMiddleware(read time.Duration, write time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			controller := http.NewResponseController(w)

			errRead := controller.SetReadDeadline(now.Add(read))
			errWrite := controller.SetWriteDeadline(now.Add(write))
			if err := errors.Join(errRead, errWrite); err != nil {
				NewErrorHandler(err, "No se pudieron ampliar los tiempos de la conexion, se usan los del servidor", LevelWarning, "CMMRDE001")
			}

			next.ServeHTTP(w, r)
		})
	}
}
