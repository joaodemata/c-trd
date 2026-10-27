package providers_test

import (
	"testing"

	"c_trd/providers"
	"c_trd/tests/testutil"
)

// observations arma una respuesta de FRED con las observaciones en el orden recibido
func observations(points ...[2]string) []byte {
	body := `{"observations":[`
	for i, point := range points {
		if i > 0 {
			body += ","
		}
		body += `{"date":"` + point[0] + `","value":"` + point[1] + `"}`
	}
	return []byte(body + `]}`)
}

func TestFredSeriesCallbacks_Fixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fixture     string
		wantRising  bool
		wantFalling bool
	}{
		// Desempleo abr-sep: 4.3, 4.3, 4.2, 4.1, 4.1, 4.2. Repunta al final pero la tendencia de la ventana baja
		{"desempleo", "fred_unrate_observations.json", false, true},
		// Balance de la FED may-sep con octubre sin valor ("."), la tendencia sube
		{"balance de la FED", "fred_walcl_observations.json", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			body := testutil.ReadFixture(t, tt.fixture)

			rising, err := providers.FredSeriesRisingCallback(body, nil)
			testutil.RequireEmptyError(t, err)
			if rising != tt.wantRising {
				t.Errorf("rising = %v, want %v", rising, tt.wantRising)
			}

			falling, err := providers.FredSeriesFallingCallback(body, nil)
			testutil.RequireEmptyError(t, err)
			if falling != tt.wantFalling {
				t.Errorf("falling = %v, want %v", falling, tt.wantFalling)
			}
		})
	}
}

func TestFredSeriesCallbacks_Trend(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        []byte
		wantRising  bool
		wantFalling bool
	}{
		{"sube (desc)", observations([2]string{"2026-09-01", "3.1"}, [2]string{"2026-03-01", "2.8"}), true, false},
		{"baja (desc)", observations([2]string{"2026-09-01", "6500000"}, [2]string{"2026-03-01", "6700000"}), false, true},
		{"sube (asc)", observations([2]string{"2026-03-01", "2.8"}, [2]string{"2026-09-01", "3.1"}), true, false},
		{"plana no sube ni baja", observations([2]string{"2026-09-01", "4.2"}, [2]string{"2026-06-01", "4.2"}, [2]string{"2026-03-01", "4.2"}), false, false},
		// El ultimo valor es menor que el primero, pero la regresion de toda la ventana sube
		{"usa toda la ventana y no solo los extremos", observations(
			[2]string{"2026-03-01", "2.0"}, [2]string{"2026-04-01", "3.0"}, [2]string{"2026-05-01", "4.0"},
			[2]string{"2026-06-01", "5.0"}, [2]string{"2026-07-01", "1.9"},
		), true, false},
		// Un pico en medio de una serie que baja no cambia la tendencia
		{"pico aislado", observations(
			[2]string{"2026-03-01", "5.0"}, [2]string{"2026-04-01", "4.8"}, [2]string{"2026-05-01", "6.0"},
			[2]string{"2026-06-01", "4.4"}, [2]string{"2026-07-01", "4.2"},
		), false, true},
		{"ignora valores faltantes", observations([2]string{"2026-09-01", "."}, [2]string{"2026-08-01", "2.5"}, [2]string{"2026-03-01", "2.8"}), false, true},
		{"valores negativos", observations([2]string{"2026-09-01", "-0.5"}, [2]string{"2026-03-01", "-1.2"}), true, false},
		{"fechas repetidas no tienen pendiente", observations([2]string{"2026-09-01", "1.0"}, [2]string{"2026-09-01", "2.0"}), false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rising, err := providers.FredSeriesRisingCallback(tt.body, nil)
			testutil.RequireEmptyError(t, err)
			if rising != tt.wantRising {
				t.Errorf("rising = %v, want %v", rising, tt.wantRising)
			}

			falling, err := providers.FredSeriesFallingCallback(tt.body, nil)
			testutil.RequireEmptyError(t, err)
			if falling != tt.wantFalling {
				t.Errorf("falling = %v, want %v", falling, tt.wantFalling)
			}
		})
	}
}

func TestFredSeriesCallbacks_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		body         []byte
		wantTracking string
	}{
		{"JSON invalido", []byte(`{`), "PCFREDE001"},
		{"mensaje de error", []byte(`{"error_code":400,"error_message":"Bad Request.  The series does not exist."}`), "PCFREDE002"},
		{"valor no numerico", observations([2]string{"2026-09-01", "abc"}, [2]string{"2026-03-01", "2.8"}), "PCFREDE003"},
		{"fecha invalida", observations([2]string{"09/2026", "3.1"}, [2]string{"2026-03-01", "2.8"}), "PCFREDE005"},
		{"sin observaciones", []byte(`{"observations":[]}`), "PCFREDE004"},
		{"una sola observacion", observations([2]string{"2026-09-01", "3.1"}), "PCFREDE004"},
		{"solo un valor sin faltantes", observations([2]string{"2026-09-01", "."}, [2]string{"2026-03-01", "2.8"}), "PCFREDE004"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rising, err := providers.FredSeriesRisingCallback(tt.body, nil)
			testutil.RequireTrackingCode(t, err, tt.wantTracking)
			if rising {
				t.Error("un error nunca debe aceptar la condicion")
			}

			falling, err := providers.FredSeriesFallingCallback(tt.body, nil)
			testutil.RequireTrackingCode(t, err, tt.wantTracking)
			if falling {
				t.Error("un error nunca debe aceptar la condicion")
			}
		})
	}
}
