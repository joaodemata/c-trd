package services_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"c_trd/services"
	"c_trd/tests/testutil"
)

func TestHashPasswordService_VerifyRoundTrip(t *testing.T) {
	t.Parallel()

	encoded, err := services.HashPasswordService("una-clave-segura")
	testutil.RequireEmptyError(t, err)

	if !strings.HasPrefix(encoded, "pbkdf2_sha256$") {
		t.Fatalf("hash = %s, se esperaba el prefijo pbkdf2_sha256$", encoded)
	}

	match, err := services.VerifyPasswordService("una-clave-segura", encoded)
	testutil.RequireEmptyError(t, err)
	if !match {
		t.Error("la contraseña correcta no coincide con su hash")
	}

	match, err = services.VerifyPasswordService("otra-clave", encoded)
	testutil.RequireEmptyError(t, err)
	if match {
		t.Error("una contraseña incorrecta coincide con el hash")
	}
}

// TestVerifyPasswordService_MongoshScriptHash el hash lo genero scripts/create_super_user.js (crypto de Node) para la
// contraseña "una-clave-segura". Si falla, el super usuario creado con el script no podria iniciar sesion
func TestVerifyPasswordService_MongoshScriptHash(t *testing.T) {
	t.Parallel()

	const scriptHash = "pbkdf2_sha256$600000$QTkDTb2ewGWfZq5nSKgEBw$SWn3p7C4uAfW/7Jqi+FDo1rVVQwU0tZS275Y4S87DvU"

	match, err := services.VerifyPasswordService("una-clave-segura", scriptHash)
	testutil.RequireEmptyError(t, err)
	if !match {
		t.Error("el hash generado por el script de mongosh no coincide con su contraseña")
	}
}

func TestHashPasswordService_UsesRandomSalt(t *testing.T) {
	t.Parallel()

	first, err := services.HashPasswordService("misma-clave")
	testutil.RequireEmptyError(t, err)
	second, err := services.HashPasswordService("misma-clave")
	testutil.RequireEmptyError(t, err)

	if first == second {
		t.Error("dos hashes de la misma contraseña son iguales, el salt no es aleatorio")
	}
}

func TestVerifyPasswordService_InvalidHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		encoded      string
		wantTracking string
	}{
		{"vacio", "", "SAUTE003"},
		{"otro algoritmo", "bcrypt$10$salt$hash", "SAUTE003"},
		{"partes de mas", "pbkdf2_sha256$1$a$b$c", "SAUTE003"},
		{"iteraciones no numericas", "pbkdf2_sha256$mil$QUFB$QUFB", "SAUTE004"},
		{"iteraciones en cero", "pbkdf2_sha256$0$QUFB$QUFB", "SAUTE004"},
		{"salt invalido", "pbkdf2_sha256$1$***$QUFB", "SAUTE005"},
		{"hash invalido", "pbkdf2_sha256$1$QUFB$***", "SAUTE006"},
		{"hash vacio", "pbkdf2_sha256$1$QUFB$", "SAUTE006"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := services.VerifyPasswordService("clave", tt.encoded)
			testutil.RequireTrackingCode(t, err, tt.wantTracking)
		})
	}
}

func TestGenerateSessionTokenService(t *testing.T) {
	t.Parallel()

	token, tokenHash, err := services.GenerateSessionTokenService()
	testutil.RequireEmptyError(t, err)

	raw, decodeErr := base64.RawURLEncoding.DecodeString(token)
	if decodeErr != nil || len(raw) != 32 {
		t.Fatalf("token = %q, se esperaban 32 bytes en base64url (error: %v)", token, decodeErr)
	}
	if tokenHash != services.HashSessionTokenService(token) {
		t.Error("el hash retornado no es el hash del token")
	}
	if tokenHash == token {
		t.Error("el hash no puede ser el mismo token")
	}

	other, _, err := services.GenerateSessionTokenService()
	testutil.RequireEmptyError(t, err)
	if other == token {
		t.Error("dos tokens generados son iguales")
	}
}

func TestHashSessionTokenService(t *testing.T) {
	t.Parallel()

	sum := sha256.Sum256([]byte("token-de-prueba"))
	want := hex.EncodeToString(sum[:])

	if got := services.HashSessionTokenService("token-de-prueba"); got != want {
		t.Errorf("hash = %s, se esperaba %s", got, want)
	}
	if services.HashSessionTokenService("token-de-prueba") == services.HashSessionTokenService("otro-token") {
		t.Error("dos tokens distintos tienen el mismo hash")
	}
}
