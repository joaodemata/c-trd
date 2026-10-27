package providers_test

import (
	"encoding/json"
	"testing"

	cmm "c_trd/common"
	"c_trd/providers"
	"c_trd/tests/testutil"
)

// fomcMarket mercado de una reunion de la FED con el formato de la Gamma API (outcomes y precios como JSON en string)
type fomcMarket struct {
	GroupItemTitle  string `json:"groupItemTitle"`
	Outcomes        string `json:"outcomes"`
	OutcomePrices   string `json:"outcomePrices"`
	Closed          bool   `json:"closed"`
	AcceptingOrders bool   `json:"acceptingOrders"`
}

type fomcEvent struct {
	Slug    string       `json:"slug"`
	EndDate string       `json:"endDate"`
	Closed  bool         `json:"closed"`
	Markets []fomcMarket `json:"markets"`
}

// openMarket mercado binario abierto con el precio Yes indicado
func openMarket(title, yes string) fomcMarket {
	return fomcMarket{GroupItemTitle: title, Outcomes: `["Yes", "No"]`, OutcomePrices: `["` + yes + `", "0"]`, AcceptingOrders: true}
}

// meeting reunion abierta con los precios Yes de bajada 25, sin cambio y subida 25
func meeting(slug, endDate, cut, hold, hike string) fomcEvent {
	return fomcEvent{Slug: slug, EndDate: endDate, Markets: []fomcMarket{
		openMarket("25 bps decrease", cut),
		openMarket("No change", hold),
		openMarket("25 bps increase", hike),
	}}
}

func fomcBody(t *testing.T, events ...fomcEvent) []byte {
	t.Helper()

	if events == nil {
		events = []fomcEvent{}
	}
	body, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// fedDecisions ejecuta los tres callbacks sobre la misma respuesta
func fedDecisions(t *testing.T, body []byte) (cut, hold, hike bool) {
	t.Helper()

	cut, err := providers.FedNextMeetingCutLikely(body, nil)
	testutil.RequireEmptyError(t, err)
	hold, err = providers.FedNextMeetingHoldLikely(body, nil)
	testutil.RequireEmptyError(t, err)
	hike, err = providers.FedNextMeetingHikeLikely(body, nil)
	testutil.RequireEmptyError(t, err)
	return cut, hold, hike
}

func TestFedNextMeetingCallbacks_Fixture(t *testing.T) {
	t.Parallel()

	// Reunion de octubre 2026: bajada 0.005, sin cambio 0.865, subida 0.1385
	cut, hold, hike := fedDecisions(t, testutil.ReadFixture(t, "polymarket_fomc_events.json"))
	if cut || !hold || hike {
		t.Errorf("cut, hold, hike = %v, %v, %v, want false, true, false", cut, hold, hike)
	}
}

func TestFedNextMeetingCallbacks_Decisions(t *testing.T) {
	t.Parallel()

	october := meeting("october", "2026-10-29T03:59:00Z", "0.05", "0.8", "0.15")
	december := meeting("december", "2026-12-10T04:59:00Z", "0.05", "0.15", "0.8")

	pastOctober := october
	pastOctober.Markets = []fomcMarket{openMarket("No change", "0.99")}
	pastOctober.Markets[0].AcceptingOrders = false

	closedOctober := october
	closedOctober.Closed = true

	cutSplit := meeting("cut", "2026-10-29T03:59:00Z", "0.35", "0.4", "0")
	cutSplit.Markets = append(cutSplit.Markets, openMarket("50+ bps decrease", "0.25"))
	cutBelow := meeting("cut", "2026-10-29T03:59:00Z", "0.35", "0.41", "0")
	cutBelow.Markets = append(cutBelow.Markets, openMarket("50+ bps decrease", "0.24"))

	dissent := fomcEvent{Slug: "dissent", EndDate: "2026-10-01T00:00:00Z", Markets: []fomcMarket{openMarket("No one dissents", "0.9")}}

	tests := []struct {
		name                        string
		events                      []fomcEvent
		wantCut, wantHold, wantHike bool
	}{
		{"proxima reunion", []fomcEvent{october, december}, false, true, false},
		{"orden de la respuesta no importa", []fomcEvent{december, october}, false, true, false},
		{"reunion que ya no acepta ordenes pasa a la siguiente", []fomcEvent{pastOctober, december}, false, false, true},
		{"reunion cerrada pasa a la siguiente", []fomcEvent{closedOctober, december}, false, false, true},
		{"evento sin decisiones conocidas se ignora", []fomcEvent{dissent, december}, false, false, true},
		{"bajadas se suman en el limite 0.35 + 0.25", []fomcEvent{cutSplit}, true, false, false},
		{"bajadas debajo del limite 0.35 + 0.24", []fomcEvent{cutBelow}, false, false, false},
		{"ninguna decision llega al limite", []fomcEvent{meeting("split", "2026-10-29T03:59:00Z", "0.3", "0.4", "0.3")}, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cut, hold, hike := fedDecisions(t, fomcBody(t, tt.events...))
			if cut != tt.wantCut || hold != tt.wantHold || hike != tt.wantHike {
				t.Errorf("cut, hold, hike = %v, %v, %v, want %v, %v, %v", cut, hold, hike, tt.wantCut, tt.wantHold, tt.wantHike)
			}
		})
	}
}

func TestFedNextMeetingCallbacks_Errors(t *testing.T) {
	t.Parallel()

	withMarket := func(market fomcMarket) []byte {
		return fomcBody(t, fomcEvent{Slug: "october", EndDate: "2026-10-29T03:59:00Z", Markets: []fomcMarket{market}})
	}
	market := openMarket("No change", "0.8")

	badOutcomes, badPrices, upDown, threeOutcomes, textPrice, highPrice := market, market, market, market, market, market
	badOutcomes.Outcomes = "Yes,No"
	badPrices.OutcomePrices = "0.8,0.2"
	upDown.Outcomes = `["Up", "Down"]`
	threeOutcomes.Outcomes, threeOutcomes.OutcomePrices = `["Yes", "No", "Maybe"]`, `["0.8", "0.1", "0.1"]`
	textPrice.OutcomePrices = `["abc", "0.2"]`
	highPrice.OutcomePrices = `["1.5", "0"]`

	closedMeeting := meeting("october", "2026-10-29T03:59:00Z", "0", "1", "0")
	closedMeeting.Closed = true

	tests := []struct {
		name string
		body []byte
		code string
	}{
		{"json invalido", []byte(`{"error":"bad request"}`), "PCPOLYE001"},
		{"endDate invalido", fomcBody(t, meeting("october", "29/10/2026", "0", "1", "0")), "PCPOLYE002"},
		{"sin eventos", fomcBody(t), "PCPOLYE003"},
		{"solo reuniones cerradas", fomcBody(t, closedMeeting), "PCPOLYE003"},
		{"outcomes invalidos", withMarket(badOutcomes), "PCPOLYE004"},
		{"precios invalidos", withMarket(badPrices), "PCPOLYE005"},
		{"mercado Up/Down", withMarket(upDown), "PCPOLYE006"},
		{"mercado con tres outcomes", withMarket(threeOutcomes), "PCPOLYE006"},
		{"precio no numerico", withMarket(textPrice), "PCPOLYE007"},
		{"precio fuera de rango", withMarket(highPrice), "PCPOLYE007"},
	}

	callbacks := map[string]func([]byte, map[string]any) (bool, *cmm.ErrorHandler){
		"cut":  providers.FedNextMeetingCutLikely,
		"hold": providers.FedNextMeetingHoldLikely,
		"hike": providers.FedNextMeetingHikeLikely,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for name, callback := range callbacks {
				got, err := callback(tt.body, nil)
				testutil.RequireTrackingCode(t, err, tt.code)
				if got {
					t.Errorf("%s: se esperaba false con error", name)
				}
			}
		})
	}
}
