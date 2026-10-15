package common_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cmm "c_trd/common"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type uploadForm struct {
	IdAsset string `form:"idAsset" validate:"required,mongodb"`
}

var uploadConfig = cmm.FileUploadConfig{
	Field:      "file",
	MaxBytes:   16,
	Extensions: []string{".csv"},
}

// newMultipartRequest arma una peticion multipart/form-data. Si fileName es vacio no se adjunta archivo
func newMultipartRequest(t *testing.T, fields map[string]string, fileName string, content []byte) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("WriteField: %v", err)
		}
	}

	if fileName != "" {
		part, err := writer.CreateFormFile(uploadConfig.Field, fileName)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestFileFormatValidateMiddleware(t *testing.T) {
	t.Parallel()

	validID := primitive.NewObjectID().Hex()
	validFields := map[string]string{"idAsset": validID}

	tests := []struct {
		name         string
		request      func(t *testing.T) *http.Request
		wantNext     bool
		wantTracking string
	}{
		{
			name: "archivo y campos validos",
			request: func(t *testing.T) *http.Request {
				return newMultipartRequest(t, validFields, "trends.csv", []byte("a,b"))
			},
			wantNext: true,
		},
		{
			name: "la extension no distingue mayusculas",
			request: func(t *testing.T) *http.Request {
				return newMultipartRequest(t, validFields, "TRENDS.CSV", []byte("a,b"))
			},
			wantNext: true,
		},
		{
			name: "no es multipart",
			request: func(t *testing.T) *http.Request {
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				return req
			},
			wantTracking: "CMMFUE001",
		},
		{
			name:         "sin archivo",
			request:      func(t *testing.T) *http.Request { return newMultipartRequest(t, validFields, "", nil) },
			wantTracking: "CMMFUE002",
		},
		{
			name: "extension no permitida",
			request: func(t *testing.T) *http.Request {
				return newMultipartRequest(t, validFields, "trends.txt", []byte("a,b"))
			},
			wantTracking: "CMMFUE003",
		},
		{
			name:         "archivo vacio",
			request:      func(t *testing.T) *http.Request { return newMultipartRequest(t, validFields, "trends.csv", nil) },
			wantTracking: "CMMFUE005",
		},
		{
			name: "archivo mas grande que MaxBytes",
			request: func(t *testing.T) *http.Request {
				return newMultipartRequest(t, validFields, "trends.csv", bytes.Repeat([]byte("x"), int(uploadConfig.MaxBytes)+1))
			},
			wantTracking: "CMMFUE005",
		},
		{
			name: "campo de texto invalido",
			request: func(t *testing.T) *http.Request {
				return newMultipartRequest(t, map[string]string{"idAsset": "abc"}, "trends.csv", []byte("a,b"))
			},
			wantTracking: "CMMFUE006",
		},
	}

	middleware := cmm.FileFormatValidateMiddleware[uploadForm](uploadConfig)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			spy := &testutil.NextSpy{}

			middleware(spy).ServeHTTP(rec, tt.request(t))

			if spy.Called != tt.wantNext {
				t.Fatalf("next llamado = %v, se esperaba %v (body: %s)", spy.Called, tt.wantNext, rec.Body.String())
			}

			if !tt.wantNext {
				if res := testutil.DecodeResponse(t, rec); res.Tracking != tt.wantTracking {
					t.Errorf("tracking = %s, se esperaba %s", res.Tracking, tt.wantTracking)
				}
				return
			}

			payload, err := cmm.GetFromRequestContext[*uploadForm](spy.Request, "payload")
			testutil.RequireEmptyError(t, err)
			if payload.IdAsset != validID {
				t.Errorf("idAsset = %q, se esperaba %q", payload.IdAsset, validID)
			}

			file, err := cmm.GetFromRequestContext[*cmm.UploadedFile](spy.Request, "file")
			testutil.RequireEmptyError(t, err)
			if string(file.Content) != "a,b" {
				t.Errorf("contenido = %q, se esperaba %q", file.Content, "a,b")
			}
		})
	}
}

func TestFileFormatValidateMiddleware_PanicsOnInvalidFormStruct(t *testing.T) {
	t.Parallel()

	// Un campo con tag form que no es string debe detener la app al registrar la ruta
	type invalidForm struct {
		Qty int `form:"qty"`
	}

	defer func() {
		if recover() == nil {
			t.Fatal("se esperaba un panic al registrar un formulario con un campo no string")
		}
	}()

	cmm.FileFormatValidateMiddleware[invalidForm](uploadConfig)
}
