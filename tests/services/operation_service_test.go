package services_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/providers"
	"c_trd/services"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func pointValues(points []providers.GoogleTrendsFilePoint) []float64 {
	values := make([]float64, len(points))
	for i, point := range points {
		values[i] = point.Value
	}
	return values
}

func TestParseGoogleTrendsCSVService_WeeklyExport(t *testing.T) {
	t.Parallel()

	content := testutil.ReadFixture(t, "google_trends_weekly.csv")
	// La ultima semana (27 sep) termina el 4 de octubre, antes de la subida
	uploadedAt := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

	series, err := services.ParseGoogleTrendsCSVService(content, uploadedAt)

	testutil.RequireEmptyError(t, err)
	if series.Keyword != "bitcoin" {
		t.Errorf("keyword = %q, se esperaba bitcoin", series.Keyword)
	}
	if series.PeriodSeconds != int64(7*24*time.Hour/time.Second) {
		t.Errorf("periodSeconds = %d, se esperaba una semana", series.PeriodSeconds)
	}
	// "<1" se interpreta como 0
	if want := []float64{40, 0, 75, 100}; !slices.Equal(pointValues(series.Points), want) {
		t.Errorf("valores = %v, se esperaba %v", pointValues(series.Points), want)
	}
}

func TestParseGoogleTrendsCSVService_DropsIncompleteLastPeriod(t *testing.T) {
	t.Parallel()

	content := testutil.ReadFixture(t, "google_trends_weekly.csv")
	// El 1 de octubre la semana del 27 de septiembre aun no termina
	uploadedAt := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

	series, err := services.ParseGoogleTrendsCSVService(content, uploadedAt)

	testutil.RequireEmptyError(t, err)
	if want := []float64{40, 0, 75}; !slices.Equal(pointValues(series.Points), want) {
		t.Errorf("valores = %v, se esperaba %v", pointValues(series.Points), want)
	}
}

func TestParseGoogleTrendsCSVService_Formats(t *testing.T) {
	t.Parallel()

	uploadedAt := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		content    string
		wantPeriod time.Duration
		wantPoints int
	}{
		{"exportacion con BOM", "\xef\xbb\xbfDay,eth: (Worldwide)\n2026-09-01,1\n2026-09-02,2\n2026-09-03,3\n", 24 * time.Hour, 3},
		{"exportacion por hora", "Time,eth: (Worldwide)\n2026-09-01T10,1\n2026-09-01T11,2\n", time.Hour, 2},
		{"exportacion mensual", "Month,eth: (Worldwide)\n2026-07,1\n2026-08,2\n2026-09,3\n", 31 * 24 * time.Hour, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			series, err := services.ParseGoogleTrendsCSVService([]byte(tt.content), uploadedAt)

			testutil.RequireEmptyError(t, err)
			if series.Keyword != "eth" {
				t.Errorf("keyword = %q, se esperaba eth", series.Keyword)
			}
			if got := time.Duration(series.PeriodSeconds) * time.Second; got != tt.wantPeriod {
				t.Errorf("periodo = %v, se esperaba %v", got, tt.wantPeriod)
			}
			if len(series.Points) != tt.wantPoints {
				t.Errorf("puntos = %d, se esperaban %d", len(series.Points), tt.wantPoints)
			}
		})
	}
}

func TestParseGoogleTrendsCSVService_Errors(t *testing.T) {
	t.Parallel()

	uploadedAt := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		content      string
		wantTracking string
	}{
		{"filas despues de los datos", "Week,a: (W)\n2026-09-06,1\n2026-09-13,2\ntotal,3\n", "SOPRE007"},
		{"comparacion de varias busquedas", "Week,a: (W),b: (W)\n2026-09-06,1,2\n", "SOPRE008"},
		{"fechas desordenadas", "Week,a: (W)\n2026-09-13,1\n2026-09-06,2\n", "SOPRE009"},
		{"fechas repetidas", "Week,a: (W)\n2026-09-06,1\n2026-09-06,2\n", "SOPRE009"},
		{"una sola fecha", "Week,a: (W)\n2026-09-06,1\n", "SOPRE010"},
		{"sin datos", "Category: All categories\n", "SOPRE010"},
		{"valor no numerico", "Week,a: (W)\n2026-09-06,abc\n", "SOPRE019"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := services.ParseGoogleTrendsCSVService([]byte(tt.content), uploadedAt)
			testutil.RequireTrackingCode(t, err, tt.wantTracking)
		})
	}

	t.Run("menos de 2 periodos completos", func(t *testing.T) {
		t.Parallel()

		// La segunda semana aun no termina, queda un solo periodo completo
		content := "Week,a: (W)\n2026-09-06,1\n2026-09-13,2\n"
		_, err := services.ParseGoogleTrendsCSVService([]byte(content), time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
		testutil.RequireTrackingCode(t, err, "SOPRE011")
	})
}

// TestGoogleTrendsFileService no es paralela porque t.Setenv cambia la carpeta de los CSV
func TestGoogleTrendsFileService(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(cmm.GoogleTrendsDirEnv, dir)

	t.Run("guarda y lee el CSV del asset", func(t *testing.T) {
		idAsset := primitive.NewObjectID()

		testutil.RequireEmptyError(t, services.SaveGoogleTrendsFileService(idAsset, testutil.ReadFixture(t, "google_trends_weekly.csv")))

		series, err := services.GetGoogleTrendsFileService(idAsset)
		testutil.RequireEmptyError(t, err)
		if series.IDAsset != idAsset.Hex() {
			t.Errorf("idAsset = %s, se esperaba %s", series.IDAsset, idAsset.Hex())
		}
		if len(series.Points) != 4 {
			t.Errorf("puntos = %d, se esperaban 4", len(series.Points))
		}
	})

	t.Run("reemplaza el CSV anterior sin dejar temporales", func(t *testing.T) {
		idAsset := primitive.NewObjectID()
		newContent := []byte("Day,eth: (Worldwide)\n2026-09-01,1\n2026-09-02,2\n2026-09-03,3\n")

		testutil.RequireEmptyError(t, services.SaveGoogleTrendsFileService(idAsset, testutil.ReadFixture(t, "google_trends_weekly.csv")))
		testutil.RequireEmptyError(t, services.SaveGoogleTrendsFileService(idAsset, newContent))

		series, err := services.GetGoogleTrendsFileService(idAsset)
		testutil.RequireEmptyError(t, err)
		if series.Keyword != "eth" {
			t.Errorf("keyword = %q, se esperaba el CSV nuevo (eth)", series.Keyword)
		}

		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			t.Fatalf("ReadDir: %v", readErr)
		}
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) != ".csv" {
				t.Errorf("archivo inesperado en la carpeta: %s", entry.Name())
			}
		}
	})

	t.Run("asset sin CSV", func(t *testing.T) {
		_, err := services.GetGoogleTrendsFileService(primitive.NewObjectID())
		testutil.RequireTrackingCode(t, err, "SOPRE016")
	})
}
