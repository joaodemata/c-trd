package providers

import (
	cmm "c_trd/common"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Las decisiones de la FED en Polymarket son eventos de la serie fomc (uno por reunion). Cada evento agrupa mercados
// binarios Yes/No cuyo groupItemTitle es fijo: "50+ bps decrease", "25 bps decrease", "No change", "25 bps increase",
// "50+ bps increase". Los callbacks siempre leen la proxima reunion abierta, asi la condicion no expira
const (
	// Probabilidad minima (suma de los precios Yes del grupo) para considerar probable la decision
	polymarketLikelyThreshold = 0.6
	// Margen para que la suma de precios en el limite (ej: 0.35 + 0.25) no quede debajo por redondeo
	polymarketEpsilon = 1e-9

	polymarketOutcomeYes = "Yes"

	fedDecisionCut  = "CUT"
	fedDecisionHold = "HOLD"
	fedDecisionHike = "HIKE"
)

// polymarketEvent estructura minima de un evento de /events de la Gamma API
type polymarketEvent struct {
	Slug    string             `json:"slug"`
	EndDate string             `json:"endDate"`
	Closed  bool               `json:"closed"`
	Markets []polymarketMarket `json:"markets"`
}

// polymarketMarket mercado de un evento. outcomes y outcomePrices llegan como JSON dentro de un string
// (ej: "[\"Yes\", \"No\"]" y "[\"0.62\", \"0.38\"]")
type polymarketMarket struct {
	GroupItemTitle  string `json:"groupItemTitle"`
	Outcomes        string `json:"outcomes"`
	OutcomePrices   string `json:"outcomePrices"`
	Closed          bool   `json:"closed"`
	AcceptingOrders bool   `json:"acceptingOrders"`
}

// FedNextMeetingCutLikely retorna true si en la proxima reunion de la FED la probabilidad de bajar la tasa
// (suma de todos los mercados "decrease") es mayor o igual a polymarketLikelyThreshold
func FedNextMeetingCutLikely(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	probabilities, err := parseFedNextMeeting(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	cutProbability := probabilities[fedDecisionCut]
	isCutLikely := cutProbability+polymarketEpsilon >= polymarketLikelyThreshold

	return isCutLikely, cmm.NewEmptyErrorHandler()
}

// FedNextMeetingHoldLikely retorna true si en la proxima reunion de la FED la probabilidad de mantener la tasa
// es mayor o igual a polymarketLikelyThreshold
func FedNextMeetingHoldLikely(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	probabilities, err := parseFedNextMeeting(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	holdProbability := probabilities[fedDecisionHold]
	isHoldLikely := holdProbability+polymarketEpsilon >= polymarketLikelyThreshold

	return isHoldLikely, cmm.NewEmptyErrorHandler()
}

// FedNextMeetingHikeLikely retorna true si en la proxima reunion de la FED la probabilidad de subir la tasa
// (suma de todos los mercados "increase") es mayor o igual a polymarketLikelyThreshold
func FedNextMeetingHikeLikely(responseData []byte, parameters map[string]any) (bool, *cmm.ErrorHandler) {
	probabilities, err := parseFedNextMeeting(responseData)
	if !err.IsEmtpy() {
		return false, err
	}

	hikeProbability := probabilities[fedDecisionHike]
	isHikeLikely := hikeProbability+polymarketEpsilon >= polymarketLikelyThreshold

	return isHikeLikely, cmm.NewEmptyErrorHandler()
}

// parseFedNextMeeting busca la reunion abierta mas cercana (por endDate, sin depender del orden de la peticion) y
// retorna la probabilidad de cada decision. Una reunion que ya no acepta ordenes (ya ocurrio y espera resolucion)
// se salta, asi la condicion pasa sola a la siguiente reunion
func parseFedNextMeeting(responseData []byte) (map[string]float64, *cmm.ErrorHandler) {
	var events []polymarketEvent
	if err := json.Unmarshal(responseData, &events); err != nil {
		return nil, cmm.NewErrorHandler(err, "Error parsing Polymarket events response", cmm.LevelFatal, "PCPOLYE001")
	}

	endDates := make(map[string]time.Time, len(events))
	for _, event := range events {
		endDate, err := time.Parse(time.RFC3339, event.EndDate)
		if err != nil {
			return nil, cmm.NewErrorHandler(fmt.Errorf("event %q: %w", event.Slug, err), "Error parsing Polymarket event endDate", cmm.LevelFatal, "PCPOLYE002")
		}
		endDates[event.Slug] = endDate
	}
	sort.SliceStable(events, func(i, j int) bool { return endDates[events[i].Slug].Before(endDates[events[j].Slug]) })

	for _, event := range events {
		if event.Closed {
			continue
		}

		probabilities, found, err := fedDecisionProbabilities(event)
		if !err.IsEmtpy() {
			return nil, err
		}
		if found {
			return probabilities, cmm.NewEmptyErrorHandler()
		}
	}

	return nil, cmm.NewErrorHandler(fmt.Errorf("got %d events", len(events)), "Polymarket has no open FED meeting with decision markets", cmm.LevelError, "PCPOLYE003")
}

// fedDecisionProbabilities suma el precio Yes de los mercados abiertos de la reunion agrupados por decision.
// found es false si la reunion no tiene mercados abiertos con una decision conocida
func fedDecisionProbabilities(event polymarketEvent) (map[string]float64, bool, *cmm.ErrorHandler) {
	probabilities := map[string]float64{fedDecisionCut: 0, fedDecisionHold: 0, fedDecisionHike: 0}
	found := false

	for _, market := range event.Markets {
		decision := fedDecisionFromTitle(market.GroupItemTitle)
		if decision == "" || market.Closed || !market.AcceptingOrders {
			continue
		}

		price, err := polymarketYesPrice(market)
		if !err.IsEmtpy() {
			return nil, false, err
		}

		probabilities[decision] += price
		found = true
	}

	return probabilities, found, cmm.NewEmptyErrorHandler()
}

// fedDecisionFromTitle clasifica el groupItemTitle del mercado. Retorna "" si no es una decision conocida
func fedDecisionFromTitle(title string) string {
	title = strings.ToLower(title)
	switch {
	case strings.Contains(title, "decrease"):
		return fedDecisionCut
	case strings.Contains(title, "increase"):
		return fedDecisionHike
	case strings.Contains(title, "no change"):
		return fedDecisionHold
	}
	return ""
}

// polymarketYesPrice retorna el precio (probabilidad) del outcome Yes. Solo acepta mercados binarios Yes/No
func polymarketYesPrice(market polymarketMarket) (float64, *cmm.ErrorHandler) {
	var outcomes, prices []string
	if err := json.Unmarshal([]byte(market.Outcomes), &outcomes); err != nil {
		return 0, cmm.NewErrorHandler(fmt.Errorf("market %q: %w", market.GroupItemTitle, err), "Error parsing Polymarket market outcomes", cmm.LevelFatal, "PCPOLYE004")
	}
	if err := json.Unmarshal([]byte(market.OutcomePrices), &prices); err != nil {
		return 0, cmm.NewErrorHandler(fmt.Errorf("market %q: %w", market.GroupItemTitle, err), "Error parsing Polymarket market prices", cmm.LevelFatal, "PCPOLYE005")
	}

	yes := slices.Index(outcomes, polymarketOutcomeYes)
	if len(outcomes) != 2 || len(prices) != len(outcomes) || yes < 0 {
		return 0, cmm.NewErrorHandler(fmt.Errorf("market %q: outcomes %v prices %v", market.GroupItemTitle, outcomes, prices), "Polymarket market is not a binary Yes/No market", cmm.LevelError, "PCPOLYE006")
	}

	price, err := strconv.ParseFloat(prices[yes], 64)
	if err != nil || price < 0 || price > 1 {
		if err == nil {
			err = errors.New("price out of range [0, 1]")
		}
		return 0, cmm.NewErrorHandler(fmt.Errorf("market %q price %q: %w", market.GroupItemTitle, prices[yes], err), "Error parsing Polymarket Yes price", cmm.LevelFatal, "PCPOLYE007")
	}

	return price, cmm.NewEmptyErrorHandler()
}
