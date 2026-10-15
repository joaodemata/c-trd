// Package testutil reune los helpers compartidos por las pruebas unitarias.
// Solo debe importarse desde archivos _test.go
package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	cmm "c_trd/common"
)

// RequireEmptyError detiene la prueba si err no es un ErrorHandler vacio. Un nil tambien falla
// porque la convencion del proyecto es retornar siempre cmm.NewEmptyErrorHandler()
func RequireEmptyError(t testing.TB, err *cmm.ErrorHandler) {
	t.Helper()
	if err == nil {
		t.Fatal("se retorno un *ErrorHandler nil, se esperaba cmm.NewEmptyErrorHandler()")
	}
	if !err.IsEmtpy() {
		t.Fatalf("error inesperado: %v", err)
	}
}

// RequireTrackingCode detiene la prueba si err esta vacio o no tiene el tracking code esperado.
// El tracking code identifica el punto exacto del fallo, por eso es la asercion mas precisa sobre un error
func RequireTrackingCode(t testing.TB, err *cmm.ErrorHandler, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("se retorno un *ErrorHandler nil, se esperaba el tracking code %s", want)
	}
	if err.IsEmtpy() {
		t.Fatalf("se esperaba un error con tracking code %s y no hubo error", want)
	}
	if err.TrackingCode != want {
		t.Fatalf("tracking code = %s, se esperaba %s (error: %v)", err.TrackingCode, want, err)
	}
}

// ReadFixture lee un archivo de la carpeta testdata del paquete que ejecuta la prueba
func ReadFixture(t testing.TB, name string) []byte {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("no se pudo leer el fixture %s: %v", name, err)
	}
	return content
}

// DecodeResponse decodifica la respuesta estandar de la API (cmm.APIResponse) escrita en el recorder
func DecodeResponse(t testing.TB, rec *httptest.ResponseRecorder) cmm.APIResponse {
	t.Helper()
	var res cmm.APIResponse
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("la respuesta no es un APIResponse valido: %v (body: %q)", err, rec.Body.String())
	}
	return res
}

// NextSpy handler que se coloca despues de un middleware para saber si dejo pasar la peticion
// y revisar la request que recibio (por ejemplo los datos inyectados en el contexto)
type NextSpy struct {
	Called  bool
	Request *http.Request
}

func (s *NextSpy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Called = true
	s.Request = r
	w.WriteHeader(http.StatusOK)
}
