package backtest_test

import (
	"math"
	"testing"
	"time"

	"c_trd/backtest"
	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/tests/testutil"
)

var simulationStart = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

// ohlc vela de 1 hora con indice i desde simulationStart
func ohlc(i int, open, high, low, close float64) backtest.Bar {
	return backtest.Bar{Time: simulationStart.Add(time.Duration(i) * time.Hour), Open: open, High: high, Low: low, Close: close}
}

// flat vela sin movimiento en price
func flat(i int, price float64) backtest.Bar {
	return ohlc(i, price, price, price, price)
}

// signalsAt señales de compra (SL 90, TP 110) o venta (SL 110, TP 90) en los indices dados
func signalsAt(count int, isLong bool, indexes ...int) []backtest.TriggerSignal {
	signals := make([]backtest.TriggerSignal, count)
	for _, i := range indexes {
		signals[i] = backtest.TriggerSignal{Active: true, StopLoss: 90, TakeProfit: 110}
		if !isLong {
			signals[i] = backtest.TriggerSignal{Active: true, StopLoss: 110, TakeProfit: 90}
		}
	}
	return signals
}

func simulationInput(bars []backtest.Bar, signals []backtest.TriggerSignal, isLong bool, maxBars int) backtest.SimulationInput {
	return backtest.SimulationInput{
		Bars: bars, Signals: signals, Timeframe: time.Hour, IsLong: isLong, MaxBars: maxBars,
		Variant: cmm.BacktestVariantTriggerOnly,
	}
}

func requireFloat(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, se esperaba %v", name, want)
	}
	if math.Abs(*got-want) > 1e-9 {
		t.Errorf("%s = %v, se esperaba %v", name, *got, want)
	}
}

func simulateOne(t *testing.T, input backtest.SimulationInput) models.BacktestTrade {
	t.Helper()
	result, err := backtest.Simulate(input)
	testutil.RequireEmptyError(t, err)
	if len(result.Trades) != 1 {
		t.Fatalf("operaciones = %d, se esperaba 1: %+v", len(result.Trades), result.Trades)
	}
	return result.Trades[0]
}

func TestSimulateTriggerOnlyExits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		bars       []backtest.Bar
		isLong     bool
		maxBars    int
		wantReason string
		wantExit   float64
		wantR      float64
		wantBars   int
	}{
		{"take profit", []backtest.Bar{flat(0, 100), ohlc(1, 100, 111, 99, 108)}, true, 5, cmm.BacktestExitTakeProfit, 110, 1, 1},
		{"stop loss y take profit en la misma vela: primero el stop loss", []backtest.Bar{flat(0, 100), ohlc(1, 100, 111, 89, 100)}, true, 5, cmm.BacktestExitStopLoss, 90, -1, 1},
		{"gap bajo el stop loss sale a la apertura", []backtest.Bar{flat(0, 100), ohlc(1, 100, 101, 99, 100), ohlc(2, 85, 86, 80, 84)}, true, 5, cmm.BacktestExitStopLoss, 85, -1.5, 2},
		{"gap sobre el take profit sale a la apertura", []backtest.Bar{flat(0, 100), ohlc(1, 100, 101, 99, 100), ohlc(2, 115, 116, 114, 115)}, true, 5, cmm.BacktestExitTakeProfit, 115, 1.5, 2},
		{"expira al cierre de la ultima vela de la ventana", []backtest.Bar{flat(0, 100), ohlc(1, 100, 104, 98, 103), ohlc(2, 103, 105, 101, 104), flat(3, 120)}, true, 2, cmm.BacktestExitExpired, 104, 0.4, 2},
		{"venta con take profit", []backtest.Bar{flat(0, 100), ohlc(1, 100, 101, 89, 95)}, false, 5, cmm.BacktestExitTakeProfit, 90, 1, 1},
		{"venta con stop loss", []backtest.Bar{flat(0, 100), ohlc(1, 100, 111, 99, 105)}, false, 5, cmm.BacktestExitStopLoss, 110, -1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			trade := simulateOne(t, simulationInput(tt.bars, signalsAt(len(tt.bars), tt.isLong, 0), tt.isLong, tt.maxBars))
			if trade.ExitReason != tt.wantReason {
				t.Fatalf("salida = %s, se esperaba %s", trade.ExitReason, tt.wantReason)
			}
			requireFloat(t, "entrada", trade.EntryPrice, 100)
			requireFloat(t, "salida", trade.ExitPrice, tt.wantExit)
			requireFloat(t, "R", trade.RMultiple, tt.wantR)
			if trade.BarsHeld != tt.wantBars {
				t.Errorf("velas = %d, se esperaban %d", trade.BarsHeld, tt.wantBars)
			}
			if !trade.EntryAt.Equal(tt.bars[1].Time) {
				t.Errorf("entrada en %s, se esperaba la apertura de la vela siguiente a la señal", trade.EntryAt)
			}
		})
	}
}

func TestSimulateCostsAndExcursions(t *testing.T) {
	t.Parallel()

	// Compra: entra a 100 con 10 bps de deslizamiento (100.1), baja a 95 y sale en el take profit (sin deslizamiento)
	bars := []backtest.Bar{flat(0, 100), ohlc(1, 100, 104, 95, 103), ohlc(2, 103, 112, 101, 109)}
	input := simulationInput(bars, signalsAt(3, true, 0), true, 5)
	input.FeeBps = 10
	input.SlippageBps = 10

	trade := simulateOne(t, input)
	entry := 100.1
	net := (110 - entry) - (entry+110)*0.001
	requireFloat(t, "entrada", trade.EntryPrice, entry)
	requireFloat(t, "salida", trade.ExitPrice, 110)
	requireFloat(t, "R", trade.RMultiple, net/(entry-90))
	requireFloat(t, "resultado %", trade.ReturnPct, net/entry*100)
	requireFloat(t, "MAE %", trade.MaePct, (entry-95)/entry*100)
	requireFloat(t, "MFE %", trade.MfePct, (110-entry)/entry*100)

	// Stop loss con deslizamiento: vende 10 bps bajo el stop loss
	stopBars := []backtest.Bar{flat(0, 100), ohlc(1, 100, 101, 85, 88)}
	stopInput := simulationInput(stopBars, signalsAt(2, true, 0), true, 5)
	stopInput.SlippageBps = 10
	stopTrade := simulateOne(t, stopInput)
	requireFloat(t, "salida stop loss", stopTrade.ExitPrice, 90*0.999)
	requireFloat(t, "MAE % stop loss", stopTrade.MaePct, (entry-90)/entry*100)
}

func TestSimulateWithoutTrade(t *testing.T) {
	t.Parallel()

	t.Run("apertura ya bajo el stop loss", func(t *testing.T) {
		t.Parallel()
		bars := []backtest.Bar{flat(0, 100), ohlc(1, 89, 92, 88, 91)}
		trade := simulateOne(t, simulationInput(bars, signalsAt(2, true, 0), true, 5))
		if trade.ExitReason != cmm.BacktestExitInvalidEntry || trade.EntryPrice != nil || trade.RMultiple != nil {
			t.Errorf("operacion = %+v, se esperaba INVALID_ENTRY sin entrada", trade)
		}
	})

	t.Run("señal en la ultima vela", func(t *testing.T) {
		t.Parallel()
		bars := []backtest.Bar{flat(0, 100), flat(1, 100)}
		trade := simulateOne(t, simulationInput(bars, signalsAt(2, true, 1), true, 5))
		if trade.ExitReason != cmm.BacktestExitEndOfData || trade.EntryAt != nil {
			t.Errorf("operacion = %+v, se esperaba END_OF_DATA sin entrada", trade)
		}
	})

	t.Run("el CSV termina con la operacion abierta", func(t *testing.T) {
		t.Parallel()
		bars := []backtest.Bar{flat(0, 100), flat(1, 101), flat(2, 102)}
		trade := simulateOne(t, simulationInput(bars, signalsAt(3, true, 0), true, 5))
		if trade.ExitReason != cmm.BacktestExitEndOfData || trade.EntryPrice == nil || trade.ExitPrice != nil {
			t.Errorf("operacion = %+v, se esperaba END_OF_DATA con entrada y sin salida", trade)
		}
	})
}

func TestSimulateSkipsSignalsWhileOpen(t *testing.T) {
	t.Parallel()

	// La señal 0 ocupa las velas 1 a 3 aunque toque el take profit en la vela 1 (en vivo expira por fecha); la de la
	// vela 2 se omite y la de la vela 4 abre otra oportunidad
	bars := []backtest.Bar{flat(0, 100), ohlc(1, 100, 111, 99, 108), flat(2, 100), flat(3, 100), flat(4, 100), flat(5, 100)}
	result, err := backtest.Simulate(simulationInput(bars, signalsAt(6, true, 0, 2, 4), true, 3))
	testutil.RequireEmptyError(t, err)

	if result.Signals != 3 || result.SkippedSignals != 1 || len(result.Trades) != 2 {
		t.Fatalf("señales = %d, omitidas = %d, operaciones = %d; se esperaba 3, 1 y 2", result.Signals, result.SkippedSignals, len(result.Trades))
	}
	if !result.Trades[1].SignalAt.Equal(bars[4].Time) {
		t.Errorf("la segunda oportunidad es de %s, se esperaba la vela 4", result.Trades[1].SignalAt)
	}
}

// conditionsMetFrom condiciones que se cumplen desde la fecha from
func conditionsMetFrom(from time.Time) backtest.ConditionCheck {
	return func(at time.Time) ([]models.BacktestConditionState, bool) {
		met := !at.Before(from)
		return []models.BacktestConditionState{{Accepted: met, Source: cmm.BacktestStateSourceEvaluation}}, met
	}
}

func TestSimulateWithConditions(t *testing.T) {
	t.Parallel()

	t.Run("entra en la vela siguiente a la activacion", func(t *testing.T) {
		t.Parallel()
		bars := []backtest.Bar{flat(0, 100), flat(1, 101), flat(2, 102), ohlc(3, 103, 111, 102, 109)}
		input := simulationInput(bars, signalsAt(4, true, 0), true, 5)
		input.Variant = cmm.BacktestVariantWithConditions
		// Se cumplen al cierre de la vela 1 (apertura de la vela 2)
		input.CheckConditions = conditionsMetFrom(bars[2].Time)

		trade := simulateOne(t, input)
		if trade.ActivatedAt == nil || !trade.ActivatedAt.Equal(bars[2].Time) {
			t.Fatalf("activada en %v, se esperaba el cierre de la vela 1", trade.ActivatedAt)
		}
		if !trade.EntryAt.Equal(bars[2].Time) || trade.ExitReason != cmm.BacktestExitTakeProfit {
			t.Errorf("operacion = %+v, se esperaba entrada en la vela 2 y take profit", trade)
		}
		requireFloat(t, "entrada", trade.EntryPrice, 102)
		if len(trade.Conditions) != 1 || !trade.Conditions[0].Accepted {
			t.Errorf("condiciones = %+v, se esperaba la condicion cumplida", trade.Conditions)
		}
	})

	t.Run("toca un nivel en curso y se cancela", func(t *testing.T) {
		t.Parallel()
		// La vela 1 toca el take profit antes de que se cumplan las condiciones; la cancelacion libera el trigger y la
		// señal de la vela 1 abre otra oportunidad
		bars := []backtest.Bar{flat(0, 100), ohlc(1, 100, 111, 99, 100), flat(2, 100), flat(3, 100)}
		input := simulationInput(bars, signalsAt(4, true, 0, 1), true, 5)
		input.Variant = cmm.BacktestVariantWithConditions
		input.CheckConditions = conditionsMetFrom(bars[3].Time.Add(time.Hour))

		result, err := backtest.Simulate(input)
		testutil.RequireEmptyError(t, err)
		if len(result.Trades) != 2 || result.SkippedSignals != 0 {
			t.Fatalf("operaciones = %d, omitidas = %d; se esperaban 2 y 0", len(result.Trades), result.SkippedSignals)
		}
		if result.Trades[0].ExitReason != cmm.BacktestExitCanceled || !result.Trades[0].ExitAt.Equal(bars[1].Time) {
			t.Errorf("primera = %+v, se esperaba CANCELED en la vela 1", result.Trades[0])
		}
	})

	t.Run("no se cumplen dentro de la ventana", func(t *testing.T) {
		t.Parallel()
		bars := []backtest.Bar{flat(0, 100), flat(1, 100), flat(2, 100), flat(3, 100)}
		input := simulationInput(bars, signalsAt(4, true, 0), true, 2)
		input.Variant = cmm.BacktestVariantWithConditions
		// Se cumplirian al cierre de la vela 2, la ultima de la ventana: ya no hay vela para entrar
		input.CheckConditions = conditionsMetFrom(bars[3].Time)

		trade := simulateOne(t, input)
		if trade.ExitReason != cmm.BacktestExitNotActivated || trade.EntryAt != nil || trade.ActivatedAt != nil {
			t.Errorf("operacion = %+v, se esperaba NOT_ACTIVATED sin entrada", trade)
		}
	})

	t.Run("sin condiciones es igual al trigger solo", func(t *testing.T) {
		t.Parallel()
		bars := []backtest.Bar{flat(0, 100), ohlc(1, 100, 111, 99, 108)}
		input := simulationInput(bars, signalsAt(2, true, 0), true, 5)
		input.Variant = cmm.BacktestVariantWithConditions

		trade := simulateOne(t, input)
		if trade.ExitReason != cmm.BacktestExitTakeProfit || !trade.EntryAt.Equal(bars[1].Time) {
			t.Errorf("operacion = %+v, se esperaba entrada en la vela 1 y take profit", trade)
		}
	})
}

func TestSimulateErrors(t *testing.T) {
	t.Parallel()

	bars := []backtest.Bar{flat(0, 100), flat(1, 100)}
	tests := []struct {
		name string
		edit func(input *backtest.SimulationInput)
		want string
	}{
		{"señales de otro largo", func(input *backtest.SimulationInput) { input.Signals = input.Signals[:1] }, "BTSIME001"},
		{"ventana en cero", func(input *backtest.SimulationInput) { input.MaxBars = 0 }, "BTSIME002"},
		{"variante desconocida", func(input *backtest.SimulationInput) { input.Variant = "OTRA" }, "BTSIME003"},
		{"comision negativa", func(input *backtest.SimulationInput) { input.FeeBps = -1 }, "BTSIME004"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := simulationInput(bars, signalsAt(2, true, 0), true, 5)
			tt.edit(&input)
			_, err := backtest.Simulate(input)
			testutil.RequireTrackingCode(t, err, tt.want)
		})
	}
}
