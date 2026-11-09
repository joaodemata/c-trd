package providers_test

import (
	"encoding/json"
	"testing"
	"time"

	"c_trd/providers"
	"c_trd/tests/testutil"
)

// calendarEvent evento del calendario de biquote para armar respuestas en los tests
type calendarEvent struct {
	Time        string   `json:"time"`
	CountryCode string   `json:"countryCode"`
	Name        string   `json:"name"`
	Importance  string   `json:"importance"`
	Type        string   `json:"type"`
	TimeMode    string   `json:"timeMode"`
	Actual      *float64 `json:"actual"`
	Forecast    *float64 `json:"forecast"`
}

// calendarBody respuesta de /api/calendar con los eventos indicados
func calendarBody(t *testing.T, events ...calendarEvent) []byte {
	t.Helper()

	body, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return body
}

// highEvent indicador de importancia alta con hora exacta a offset de ahora
func highEvent(country string, offset time.Duration, actual, forecast *float64) calendarEvent {
	return calendarEvent{
		Time:        time.Now().UTC().Add(offset).Format(time.RFC3339),
		CountryCode: country,
		Name:        "CPI y/y",
		Importance:  "high",
		Type:        "indicator",
		TimeMode:    "exact",
		Actual:      actual,
		Forecast:    forecast,
	}
}

func number(value float64) *float64 { return &value }

func TestEconomicCalendarHighImpactUpcomingCallback(t *testing.T) {
	t.Parallel()

	medium := highEvent("US", 2*time.Hour, nil, number(1))
	medium.Importance = "medium"
	holiday := highEvent("US", 2*time.Hour, nil, nil)
	holiday.Type = "holiday"
	// Sin hora exacta: la hora es un marcador del dia, cuenta si el dia se cruza con la ventana
	inexactToday := highEvent("US", -3*time.Hour, nil, nil)
	inexactToday.TimeMode = "date"
	inexactOld := highEvent("US", -30*time.Hour, nil, nil)
	inexactOld.TimeMode = "tentative"

	tests := []struct {
		name   string
		events []calendarEvent
		hours  any
		want   bool
	}{
		{"alta en 2 horas con ventana de 24", []calendarEvent{highEvent("US", 2*time.Hour, nil, number(4.2))}, int64(24), true},
		{"alta en 30 horas fuera de la ventana", []calendarEvent{highEvent("US", 30*time.Hour, nil, nil)}, int64(24), false},
		{"alta que ya paso", []calendarEvent{highEvent("US", -2*time.Hour, nil, nil)}, int64(24), false},
		{"alta con dato real ya publicado", []calendarEvent{highEvent("US", time.Hour, number(4), nil)}, int64(24), false},
		{"importancia media no cuenta", []calendarEvent{medium}, int64(24), false},
		{"feriado no cuenta", []calendarEvent{holiday}, int64(24), false},
		{"sin hora exacta del dia de hoy", []calendarEvent{inexactToday}, int64(1), true},
		{"sin hora exacta de un dia que ya paso", []calendarEvent{inexactOld}, int64(24), false},
		{"hours int32 leido de Mongo", []calendarEvent{highEvent("US", 2*time.Hour, nil, nil)}, int32(3), true},
		{"calendario vacio", []calendarEvent{}, int64(24), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.EconomicCalendarHighImpactUpcomingCallback(calendarBody(t, tt.events...), map[string]any{"hours": tt.hours})
			testutil.RequireEmptyError(t, err)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEconomicCalendarHighImpactClusterCallback(t *testing.T) {
	t.Parallel()

	// CPI m/m y CPI y/y de US a la misma hora son una sola publicacion
	sameRelease := highEvent("US", 2*time.Hour, nil, nil)

	tests := []struct {
		name        string
		events      []calendarEvent
		minReleases any
		want        bool
	}{
		{"tres publicaciones distintas", []calendarEvent{highEvent("US", time.Hour, nil, nil), highEvent("EU", 2*time.Hour, nil, nil), highEvent("JP", 3*time.Hour, nil, nil)}, int64(3), true},
		{"solo dos publicaciones", []calendarEvent{highEvent("US", time.Hour, nil, nil), highEvent("EU", 2*time.Hour, nil, nil)}, int64(3), false},
		{"mismo pais y hora cuenta una vez", []calendarEvent{sameRelease, sameRelease, highEvent("EU", 3*time.Hour, nil, nil)}, int64(3), false},
		{"mismo pais y hora con minimo 2", []calendarEvent{sameRelease, sameRelease, highEvent("EU", 3*time.Hour, nil, nil)}, int64(2), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.EconomicCalendarHighImpactClusterCallback(calendarBody(t, tt.events...), map[string]any{"hours": int64(24), "minReleases": tt.minReleases})
			testutil.RequireEmptyError(t, err)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEconomicCalendarHighImpactSurpriseCallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		events []calendarEvent
		want   bool
	}{
		{"real 0.7 contra pronostico 0.5 es 40%", []calendarEvent{highEvent("US", -time.Hour, number(0.7), number(0.5))}, true},
		{"real 0.55 contra pronostico 0.5 es 10%", []calendarEvent{highEvent("US", -time.Hour, number(0.55), number(0.5))}, false},
		{"sorpresa negativa tambien cuenta", []calendarEvent{highEvent("US", -time.Hour, number(-0.2), number(0.5))}, true},
		{"justo en el 20% cuenta", []calendarEvent{highEvent("US", -time.Hour, number(6), number(5))}, true},
		{"sin pronostico se ignora", []calendarEvent{highEvent("US", -time.Hour, number(5), nil)}, false},
		{"pronostico 0 se ignora", []calendarEvent{highEvent("US", -time.Hour, number(0.25), number(0))}, false},
		{"publicada fuera de la ventana", []calendarEvent{highEvent("US", -10*time.Hour, number(0.7), number(0.5))}, false},
		{"aun sin dato real", []calendarEvent{highEvent("US", time.Hour, nil, number(0.5))}, false},
		{"toma la mayor sorpresa", []calendarEvent{highEvent("US", -time.Hour, number(0.5), number(0.5)), highEvent("EU", -2*time.Hour, number(3), number(2))}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.EconomicCalendarHighImpactSurpriseCallback(calendarBody(t, tt.events...), map[string]any{"hours": int64(6), "surprisePercent": float64(20)})
			testutil.RequireEmptyError(t, err)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEconomicCalendarCallbacks_Fixture(t *testing.T) {
	t.Parallel()

	// Respuesta real de /api/calendar?countries=US&importance=high. Sus fechas son fijas, solo se valida que se lea
	body := testutil.ReadFixture(t, "biquote_calendar_us_high.json")
	parameters := map[string]any{"hours": int64(24), "minReleases": int64(2), "surprisePercent": float64(20)}

	callbacks := []providers.CallbackFunc{
		providers.EconomicCalendarHighImpactUpcomingCallback,
		providers.EconomicCalendarHighImpactClusterCallback,
		providers.EconomicCalendarHighImpactSurpriseCallback,
	}
	for _, callback := range callbacks {
		_, err := callback(body, parameters)
		testutil.RequireEmptyError(t, err)
	}
}

func TestEconomicCalendarCallbacks_Errors(t *testing.T) {
	t.Parallel()

	validBody := []byte(`[]`)
	upcoming := providers.EconomicCalendarHighImpactUpcomingCallback
	cluster := providers.EconomicCalendarHighImpactClusterCallback
	surprise := providers.EconomicCalendarHighImpactSurpriseCallback

	tests := []struct {
		name       string
		callback   providers.CallbackFunc
		body       []byte
		parameters map[string]any
		code       string
	}{
		{"json invalido", upcoming, []byte(`{not json`), map[string]any{"hours": int64(24)}, "PCBIQE001"},
		{"biquote responde error", upcoming, []byte(`{"error":"Invalid importance 'extreme'","validImportance":["none","low","medium","high"]}`), map[string]any{"hours": int64(24)}, "PCBIQE002"},
		{"hora del evento invalida", upcoming, []byte(`[{"time":"mañana","importance":"high","type":"indicator","timeMode":"exact"}]`), map[string]any{"hours": int64(24)}, "PCBIQE003"},
		{"hours faltante", upcoming, validBody, map[string]any{}, "PCBIQE004"},
		{"hours con decimales", upcoming, validBody, map[string]any{"hours": 1.5}, "PCBIQE004"},
		{"hours mayor a 168", upcoming, validBody, map[string]any{"hours": int64(169)}, "PCBIQE005"},
		{"hours 0", cluster, validBody, map[string]any{"hours": int64(0), "minReleases": int64(2)}, "PCBIQE005"},
		{"hours de sorpresa faltante", surprise, validBody, map[string]any{"surprisePercent": float64(20)}, "PCBIQE006"},
		{"hours de sorpresa mayor a 24", surprise, validBody, map[string]any{"hours": int64(25), "surprisePercent": float64(20)}, "PCBIQE007"},
		{"minReleases texto", cluster, validBody, map[string]any{"hours": int64(24), "minReleases": "3"}, "PCBIQE008"},
		{"minReleases 1", cluster, validBody, map[string]any{"hours": int64(24), "minReleases": int64(1)}, "PCBIQE009"},
		{"surprisePercent faltante", surprise, validBody, map[string]any{"hours": int64(6)}, "PCBIQE010"},
		{"surprisePercent 0", surprise, validBody, map[string]any{"hours": int64(6), "surprisePercent": float64(0)}, "PCBIQE011"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accepted, err := tt.callback(tt.body, tt.parameters)
			testutil.RequireTrackingCode(t, err, tt.code)
			if accepted {
				t.Error("accepted = true, want false on error")
			}
		})
	}
}
