package backtest

import (
	"fmt"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
)

// Simulacion de las oportunidades de un trigger sobre las velas del CSV, como el flujo en vivo:
//   - Cada vela con señal crea una oportunidad. Mientras la oportunidad de la vela i este abierta (hasta la vela
//     i + max_candlestick_qty, como expiration_date en vivo) las nuevas señales se omiten, igual que el hook
//   - TRIGGER_ONLY entra a la apertura de la vela siguiente a la señal
//   - WITH_CONDITIONS consulta las condiciones al cierre de cada vela desde la señal; cuando todas se cumplen
//     (ACTIVE) entra a la apertura de la vela siguiente. Si antes toca el stop loss o el take profit se cancela (setup
//     invalidado) y si no se cumplen dentro de la ventana expira sin operacion
//   - Con la operacion abierta: si una vela abre mas alla de un nivel se sale a la apertura (gap); si una vela toca el
//     stop loss y el take profit se asume que primero toco el stop loss. Al terminar la vela i + max_candlestick_qty
//     la operacion se cierra al cierre de esa vela
//   - El deslizamiento va en contra en las ordenes a mercado (entrada, stop loss y expiracion); el take profit es una
//     orden limite sin deslizamiento. La comision se cobra en la entrada y en la salida

// ConditionCheck estado de las condiciones del trigger en un instante y si todas se cumplen
type ConditionCheck func(at time.Time) ([]models.BacktestConditionState, bool)

// SimulationInput velas y señales de un trigger y la configuracion de una variante
type SimulationInput struct {
	Bars    []Bar
	Signals []TriggerSignal
	// Duracion de una vela: el cierre de la vela es su apertura + Timeframe
	Timeframe   time.Duration
	IsLong      bool
	MaxBars     int
	FeeBps      float64
	SlippageBps float64
	Variant     string
	// nil si el trigger no tiene condiciones: siempre se cumplen
	CheckConditions ConditionCheck
}

// SimulationResult oportunidades simuladas de una variante y cuantas señales hubo y se omitieron
type SimulationResult struct {
	Trades         []models.BacktestTrade
	Signals        int
	SkippedSignals int
}

// activation resultado de esperar las condiciones de una oportunidad en curso
type activation struct {
	// Indice de la vela de entrada, solo si exitReason es ""
	entryIndex  int
	activatedAt *time.Time
	conditions  []models.BacktestConditionState
	// Razon y vela si la oportunidad termino sin operacion (CANCELED, NOT_ACTIVATED o END_OF_DATA)
	exitReason string
	exitIndex  int
}

// Simulate simula las oportunidades de una variante
func Simulate(input SimulationInput) (SimulationResult, *cmm.ErrorHandler) {
	result := SimulationResult{Trades: []models.BacktestTrade{}}
	if len(input.Bars) != len(input.Signals) {
		return result, cmm.NewErrorHandler(fmt.Errorf("bars %d signals %d", len(input.Bars), len(input.Signals)), "Las señales no corresponden a las velas", cmm.LevelFatal, "BTSIME001")
	}
	if input.MaxBars < 1 {
		return result, cmm.NewErrorHandler(fmt.Errorf("max bars %d", input.MaxBars), "max_candlestick_qty debe ser al menos 1", cmm.LevelError, "BTSIME002")
	}
	if input.Variant != cmm.BacktestVariantTriggerOnly && input.Variant != cmm.BacktestVariantWithConditions {
		return result, cmm.NewErrorHandler(fmt.Errorf("variant %q", input.Variant), "Variante de backtest desconocida", cmm.LevelFatal, "BTSIME003")
	}
	if input.FeeBps < 0 || input.SlippageBps < 0 {
		return result, cmm.NewErrorHandler(fmt.Errorf("fee %v slippage %v", input.FeeBps, input.SlippageBps), "La comision y el deslizamiento no pueden ser negativos", cmm.LevelError, "BTSIME004")
	}

	// Primera vela en la que una señal puede abrir una oportunidad
	nextFree := 0
	for i, signal := range input.Signals {
		if !signal.Active {
			continue
		}
		result.Signals++
		if i < nextFree {
			result.SkippedSignals++
			continue
		}

		trade, lastBusy := simulateOpportunity(input, i)
		result.Trades = append(result.Trades, trade)
		nextFree = lastBusy + 1
	}

	return result, cmm.NewEmptyErrorHandler()
}

// simulateOpportunity simula la oportunidad de la señal de la vela signalIndex. Retorna la operacion y la ultima vela
// en la que la oportunidad sigue abierta (las señales hasta esa vela se omiten)
func simulateOpportunity(input SimulationInput, signalIndex int) (models.BacktestTrade, int) {
	signal := input.Signals[signalIndex]
	lastIndex := signalIndex + input.MaxBars
	trade := models.BacktestTrade{
		Variant:    input.Variant,
		SignalAt:   input.Bars[signalIndex].Time,
		StopLoss:   signal.StopLoss,
		TakeProfit: signal.TakeProfit,
		Conditions: []models.BacktestConditionState{},
	}

	waited := activation{entryIndex: signalIndex + 1}
	if input.Variant == cmm.BacktestVariantTriggerOnly {
		states, _ := checkConditions(input, signalIndex)
		waited.conditions = states
	}
	if input.Variant == cmm.BacktestVariantWithConditions {
		waited = waitForConditions(input, signalIndex, lastIndex)
	}
	trade.ActivatedAt = waited.activatedAt
	trade.Conditions = waited.conditions

	// Termino sin operacion. Una cancelacion libera el trigger como la cancelacion del hook en vivo
	if waited.exitReason != "" {
		trade.ExitReason = waited.exitReason
		if waited.exitIndex < len(input.Bars) {
			exitAt := input.Bars[waited.exitIndex].Time
			trade.ExitAt = &exitAt
		}
		if waited.exitReason == cmm.BacktestExitCanceled {
			return trade, waited.exitIndex - 1
		}
		return trade, lastIndex
	}

	trade = runTrade(input, trade, waited.entryIndex, lastIndex)
	return trade, lastIndex
}

// checkConditions consulta las condiciones al cierre de la vela index. Sin condiciones siempre se cumplen
func checkConditions(input SimulationInput, index int) ([]models.BacktestConditionState, bool) {
	if input.CheckConditions == nil {
		return []models.BacktestConditionState{}, true
	}

	closeAt := input.Bars[index].Time.Add(input.Timeframe)
	states, allMet := input.CheckConditions(closeAt)
	return states, allMet
}

// waitForConditions recorre la oportunidad en curso desde la vela de la señal hasta que todas las condiciones se
// cumplen al cierre de una vela (entra en la siguiente), toca un nivel (CANCELED) o llega a la ultima vela de la
// ventana (NOT_ACTIVATED: activarse al cierre de la ultima vela ya no deja vela para entrar)
func waitForConditions(input SimulationInput, signalIndex int, lastIndex int) activation {
	signal := input.Signals[signalIndex]
	conditions := []models.BacktestConditionState{}

	for index := signalIndex; index <= lastIndex; index++ {
		if index >= len(input.Bars) {
			return activation{conditions: conditions, exitReason: cmm.BacktestExitEndOfData, exitIndex: index}
		}
		// La vela de la señal ya cerro cuando llega la alerta, sus maximos y minimos no cuentan
		if index > signalIndex && touchesLevels(input.Bars[index], signal, input.IsLong) {
			return activation{conditions: conditions, exitReason: cmm.BacktestExitCanceled, exitIndex: index}
		}
		if index == lastIndex {
			break
		}

		states, allMet := checkConditions(input, index)
		conditions = states
		if allMet {
			activatedAt := input.Bars[index].Time.Add(input.Timeframe)
			return activation{entryIndex: index + 1, activatedAt: &activatedAt, conditions: states}
		}
	}

	return activation{conditions: conditions, exitReason: cmm.BacktestExitNotActivated, exitIndex: lastIndex}
}

// touchesLevels la vela toca el stop loss o el take profit
func touchesLevels(bar Bar, signal TriggerSignal, isLong bool) bool {
	if isLong {
		return bar.Low <= signal.StopLoss || bar.High >= signal.TakeProfit
	}
	return bar.High >= signal.StopLoss || bar.Low <= signal.TakeProfit
}

// runTrade entra a la apertura de la vela entryIndex y recorre la operacion hasta un nivel o la ultima vela de la ventana
func runTrade(input SimulationInput, trade models.BacktestTrade, entryIndex int, lastIndex int) models.BacktestTrade {
	if entryIndex >= len(input.Bars) {
		trade.ExitReason = cmm.BacktestExitEndOfData
		return trade
	}

	signal := TriggerSignal{Active: true, StopLoss: trade.StopLoss, TakeProfit: trade.TakeProfit}
	entryBar := input.Bars[entryIndex]
	entryAt := entryBar.Time
	trade.EntryAt = &entryAt

	// Un gap en la apertura ya paso un nivel: la orden no tiene sentido
	if !isBetweenLevels(entryBar.Open, signal, input.IsLong) {
		trade.EntryAt = nil
		trade.ExitAt = &entryAt
		trade.ExitReason = cmm.BacktestExitInvalidEntry
		return trade
	}

	slippage := input.SlippageBps / 10000
	entryPrice := adversePrice(entryBar.Open, slippage, input.IsLong, true)
	trade.EntryPrice = &entryPrice

	excursion := newExcursion(entryPrice)
	for index := entryIndex; index <= lastIndex; index++ {
		if index >= len(input.Bars) {
			trade.ExitReason = cmm.BacktestExitEndOfData
			trade.BarsHeld = index - entryIndex
			return trade
		}

		bar := input.Bars[index]
		exit := barExit(bar, signal, index > entryIndex, slippage, input.IsLong)
		excursion = excursion.update(bar, exit, input.IsLong)
		if exit.reason == "" && index < lastIndex {
			continue
		}
		if exit.reason == "" {
			exit = tradeExit{price: adversePrice(bar.Close, slippage, input.IsLong, false), reason: cmm.BacktestExitExpired}
		}

		return closeTrade(input, trade, excursion, exit, bar.Time, index-entryIndex+1)
	}

	return trade
}

// isBetweenLevels el precio esta entre el stop loss y el take profit
func isBetweenLevels(price float64, signal TriggerSignal, isLong bool) bool {
	if isLong {
		return signal.StopLoss < price && price < signal.TakeProfit
	}
	return signal.TakeProfit < price && price < signal.StopLoss
}

// adversePrice aplica el deslizamiento en contra: comprar mas caro y vender mas barato. isEntry indica si la orden
// abre la operacion (compra en un long) o la cierra (venta en un long)
func adversePrice(price float64, slippage float64, isLong bool, isEntry bool) float64 {
	buys := isLong == isEntry
	if buys {
		return price * (1 + slippage)
	}
	return price * (1 - slippage)
}

// tradeExit salida en una vela. level es el precio sin deslizamiento (para el MAE y el MFE), reason "" si no salio
type tradeExit struct {
	price  float64
	level  float64
	reason string
}

// barExit salida de la operacion en la vela. Primero los gaps de la apertura (salvo en la vela de entrada, que abre
// en el precio de entrada) y luego los niveles, el stop loss antes que el take profit
func barExit(bar Bar, signal TriggerSignal, canGap bool, slippage float64, isLong bool) tradeExit {
	stopGapped := (isLong && bar.Open <= signal.StopLoss) || (!isLong && bar.Open >= signal.StopLoss)
	profitGapped := (isLong && bar.Open >= signal.TakeProfit) || (!isLong && bar.Open <= signal.TakeProfit)
	stopTouched := (isLong && bar.Low <= signal.StopLoss) || (!isLong && bar.High >= signal.StopLoss)
	profitTouched := (isLong && bar.High >= signal.TakeProfit) || (!isLong && bar.Low <= signal.TakeProfit)

	switch {
	case canGap && stopGapped:
		return tradeExit{price: adversePrice(bar.Open, slippage, isLong, false), level: bar.Open, reason: cmm.BacktestExitStopLoss}
	case canGap && profitGapped:
		return tradeExit{price: bar.Open, level: bar.Open, reason: cmm.BacktestExitTakeProfit}
	case stopTouched:
		return tradeExit{price: adversePrice(signal.StopLoss, slippage, isLong, false), level: signal.StopLoss, reason: cmm.BacktestExitStopLoss}
	case profitTouched:
		return tradeExit{price: signal.TakeProfit, level: signal.TakeProfit, reason: cmm.BacktestExitTakeProfit}
	}
	return tradeExit{}
}

// excursion peor y mejor precio que vio la operacion
type excursion struct {
	entry      float64
	adverse    float64
	favourable float64
}

func newExcursion(entry float64) excursion {
	return excursion{entry: entry, adverse: entry, favourable: entry}
}

// update suma la vela. En la vela de salida solo cuenta hasta la salida: con stop loss no se mira el maximo (el stop
// loss fue primero) y con take profit el favorable llega hasta el take profit
func (e excursion) update(bar Bar, exit tradeExit, isLong bool) excursion {
	adverseExtreme, favourableExtreme := bar.Low, bar.High
	if !isLong {
		adverseExtreme, favourableExtreme = bar.High, bar.Low
	}

	switch exit.reason {
	case cmm.BacktestExitStopLoss:
		adverseExtreme, favourableExtreme = exit.level, e.favourable
		if exit.level == bar.Open {
			favourableExtreme = bar.Open
		}
	case cmm.BacktestExitTakeProfit:
		favourableExtreme = exit.level
		if exit.level == bar.Open {
			adverseExtreme = bar.Open
		}
	}

	if isLong {
		e.adverse = min(e.adverse, adverseExtreme)
		e.favourable = max(e.favourable, favourableExtreme)
	} else {
		e.adverse = max(e.adverse, adverseExtreme)
		e.favourable = min(e.favourable, favourableExtreme)
	}
	return e
}

// closeTrade calcula el resultado neto, el R y las excursiones en %
func closeTrade(input SimulationInput, trade models.BacktestTrade, e excursion, exit tradeExit, exitAt time.Time, barsHeld int) models.BacktestTrade {
	direction := 1.0
	if !input.IsLong {
		direction = -1.0
	}
	fee := input.FeeBps / 10000
	entry := *trade.EntryPrice

	net := (exit.price-entry)*direction - (entry+exit.price)*fee
	risk := (entry - trade.StopLoss) * direction
	returnPct := net / entry * 100
	rMultiple := net / risk
	maePct := max((e.entry-e.adverse)*direction/e.entry*100, 0)
	mfePct := max((e.favourable-e.entry)*direction/e.entry*100, 0)

	exitPrice := exit.price
	trade.ExitAt = &exitAt
	trade.ExitPrice = &exitPrice
	trade.ExitReason = exit.reason
	trade.BarsHeld = barsHeld
	trade.ReturnPct = &returnPct
	trade.RMultiple = &rMultiple
	trade.MaePct = &maePct
	trade.MfePct = &mfePct
	return trade
}
