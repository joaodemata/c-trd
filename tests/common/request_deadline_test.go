package common_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/tests/testutil"
)

// slowUploadStatus envia un body en dos partes separadas por pause a un servidor con ReadTimeout de 150ms. Retorna el
// status y los bytes que leyo el handler, o 0 si el servidor corto la conexion
func slowUploadStatus(t *testing.T, handler http.Handler, pause time.Duration) (int, string) {
	t.Helper()

	server := httptest.NewUnstartedServer(handler)
	server.Config.ReadTimeout = 150 * time.Millisecond
	server.Config.WriteTimeout = 150 * time.Millisecond
	server.Start()
	t.Cleanup(server.Close)

	body, writer := io.Pipe()
	go func() {
		writer.Write([]byte("primera parte,"))
		time.Sleep(pause)
		writer.Write([]byte("segunda parte"))
		writer.Close()
	}()

	response, err := http.Post(server.URL, "text/csv", body)
	if err != nil {
		return 0, ""
	}
	defer response.Body.Close()
	read, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(read)
}

// readAllHandler lee todo el body y responde cuantos bytes leyo
var readAllHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	content, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusRequestTimeout)
		return
	}
	w.Write([]byte(strconv.Itoa(len(content))))
})

// TestRequestDeadlineMiddleware no es paralela para no competir por tiempo con otras pruebas: mide tiempos reales
func TestRequestDeadlineMiddleware(t *testing.T) {
	pause := 400 * time.Millisecond

	// Sin el middleware el ReadTimeout del servidor corta la subida lenta
	status, _ := slowUploadStatus(t, readAllHandler, pause)
	if status == http.StatusOK {
		t.Fatal("sin el middleware la subida lenta no deberia terminar")
	}

	// Con el middleware la misma ruta tiene mas tiempo y lee el body completo
	extended := cmm.RequestDeadlineMiddleware(2*time.Second, 2*time.Second)(readAllHandler)
	status, body := slowUploadStatus(t, extended, pause)
	if status != http.StatusOK || body != "27" {
		t.Errorf("status = %d, body = %q; se esperaba 200 con los 27 bytes", status, body)
	}
}

// Si la conexion no permite cambiar los tiempos sigue con la siguiente ruta
func TestRequestDeadlineMiddlewareWithoutDeadlines(t *testing.T) {
	t.Parallel()

	spy := &testutil.NextSpy{}
	recorder := httptest.NewRecorder()
	cmm.RequestDeadlineMiddleware(time.Minute, time.Minute)(spy).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/", nil))

	if !spy.Called {
		t.Error("el middleware debe llamar a la siguiente ruta aunque no pueda ampliar los tiempos")
	}
}
