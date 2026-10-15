package common_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	cmm "c_trd/common"
	"c_trd/tests/testutil"
)

type contextSample struct {
	Name string
}

func TestRequestContext(t *testing.T) {
	t.Parallel()

	newRequest := func() *http.Request {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		return cmm.SetInRequestContext(req, "sample", &contextSample{Name: "ctrd"})
	}

	t.Run("recupera el valor con el tipo correcto", func(t *testing.T) {
		t.Parallel()

		got, err := cmm.GetFromRequestContext[*contextSample](newRequest(), "sample")

		testutil.RequireEmptyError(t, err)
		if got.Name != "ctrd" {
			t.Errorf("Name = %q, se esperaba %q", got.Name, "ctrd")
		}
	})

	t.Run("llave inexistente", func(t *testing.T) {
		t.Parallel()

		_, err := cmm.GetFromRequestContext[*contextSample](newRequest(), "otra")
		testutil.RequireTrackingCode(t, err, "CMMREH001")
	})

	t.Run("tipo incorrecto", func(t *testing.T) {
		t.Parallel()

		// Se guardo un puntero, pedirlo por valor debe fallar
		_, err := cmm.GetFromRequestContext[contextSample](newRequest(), "sample")
		testutil.RequireTrackingCode(t, err, "CMMREH001")
	})
}
