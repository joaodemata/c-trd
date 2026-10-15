package common_test

import (
	"errors"
	"testing"

	cmm "c_trd/common"
)

func TestNewEmptyErrorHandler_IsEmpty(t *testing.T) {
	t.Parallel()

	if !cmm.NewEmptyErrorHandler().IsEmtpy() {
		t.Fatal("NewEmptyErrorHandler() deberia ser vacio")
	}
}

func TestNewErrorHandler(t *testing.T) {
	t.Parallel()

	cause := errors.New("connection timeout")
	err := cmm.NewErrorHandler(cause, "no se pudo conectar", cmm.LevelDatabase, "TESTE001")

	if err.IsEmtpy() {
		t.Fatal("un error con nivel DATABASE no deberia ser vacio")
	}
	if !errors.Is(err, cause) {
		t.Error("errors.Is deberia encontrar la causa original via Unwrap")
	}

	want := "[DATABASE] no se pudo conectar | Reason: connection timeout | Track: TESTE001"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, se esperaba %q", got, want)
	}
}

func TestNewErrorHandler_WithoutCause(t *testing.T) {
	t.Parallel()

	err := cmm.NewErrorHandler(nil, "sin causa", cmm.LevelWarning, "TESTE002")

	want := "[WARNING] sin causa | Track: TESTE002"
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, se esperaba %q", got, want)
	}
}
