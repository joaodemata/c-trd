package providers_test

import (
	"encoding/json"
	"testing"
	"time"

	"c_trd/providers"
	"c_trd/tests/testutil"
)

const week = 7 * 24 * time.Hour

// El archivo vence un periodo mas este margen despues de subirse (googleTrendsFileStaleMargin)
const staleMargin = 24 * time.Hour

// fileResponse arma la respuesta de la ruta local /operation/google_trends/series con los valores dados
func fileResponse(t *testing.T, uploadedAt time.Time, period time.Duration, values ...float64) []byte {
	t.Helper()

	points := make([]providers.GoogleTrendsFilePoint, len(values))
	for i, value := range values {
		points[i] = providers.GoogleTrendsFilePoint{Date: uploadedAt.Add(-period * time.Duration(len(values)-i)), Value: value}
	}

	body, err := json.Marshal(struct {
		Status string                           `json:"status"`
		Data   providers.GoogleTrendsFileSeries `json:"data"`
	}{
		Status: "SUCCESS",
		Data: providers.GoogleTrendsFileSeries{
			IDAsset:       "asset",
			Keyword:       "bitcoin",
			UploadedAt:    uploadedAt,
			PeriodSeconds: int64(period / time.Second),
			Points:        points,
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return body
}

// Las zonas y los nuevos maximos/minimos se prueban con los callbacks del CSV porque reciben la serie directa
func TestGoogleTrendsFileCallbacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		callback providers.CallbackFunc
		values   []float64
		want     bool
	}{
		{"alta: ultimo en el 10% superior", providers.GoogleTrendsFileHighCallback, []float64{0, 100, 95}, true},
		{"alta: justo en el umbral", providers.GoogleTrendsFileHighCallback, []float64{0, 100, 90}, true},
		{"alta: debajo del umbral", providers.GoogleTrendsFileHighCallback, []float64{0, 100, 89}, false},
		{"alta: serie plana", providers.GoogleTrendsFileHighCallback, []float64{50, 50, 50}, false},
		{"baja: ultimo en el 10% inferior", providers.GoogleTrendsFileLowCallback, []float64{0, 100, 5}, true},
		{"baja: justo en el umbral", providers.GoogleTrendsFileLowCallback, []float64{0, 100, 10}, true},
		{"baja: encima del umbral", providers.GoogleTrendsFileLowCallback, []float64{0, 100, 11}, false},
		{"baja: serie plana", providers.GoogleTrendsFileLowCallback, []float64{50, 50, 50}, false},
		{"nuevo maximo", providers.GoogleTrendsFileNewMaxCallback, []float64{1, 5, 6}, true},
		{"igualar el maximo no es nuevo maximo", providers.GoogleTrendsFileNewMaxCallback, []float64{1, 5, 5}, false},
		{"nuevo minimo", providers.GoogleTrendsFileNewMinCallback, []float64{5, 3, 2}, true},
		{"igualar el minimo no es nuevo minimo", providers.GoogleTrendsFileNewMinCallback, []float64{5, 3, 3}, false},
	}

	now := time.Now().UTC()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accepted, err := tt.callback(fileResponse(t, now, week, tt.values...))

			testutil.RequireEmptyError(t, err)
			if accepted != tt.want {
				t.Errorf("valores %v: accepted = %v, se esperaba %v", tt.values, accepted, tt.want)
			}
		})
	}
}

func TestGoogleTrendsFileCallbacks_Errors(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	staleUpload := now.Add(-week - staleMargin - time.Hour)

	tests := []struct {
		name         string
		body         []byte
		wantTracking string
	}{
		{"JSON invalido", []byte(`{`), "PCGTFE001"},
		{"menos de 2 puntos", fileResponse(t, now, week, 10), "PCGTFE002"},
		{"archivo desactualizado", fileResponse(t, staleUpload, week, 10, 20, 30), "PCGTFE003"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accepted, err := providers.GoogleTrendsFileHighCallback(tt.body)

			testutil.RequireTrackingCode(t, err, tt.wantTracking)
			if accepted {
				t.Error("un error nunca debe aceptar la condicion")
			}
		})
	}
}
