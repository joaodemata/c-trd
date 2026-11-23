package backtest_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"c_trd/backtest"
	"c_trd/tests/testutil"
)

// Export de TradingView con BOM, Volume y dos triggers en el mismo grafico (4 horas)
const chartCSV = "\ufefftime,open,high,low,close,Volume,CTRD_SIGNAL_BTC-EMA,CTRD_SL_BTC-EMA,CTRD_TP_BTC-EMA,CTRD_SIGNAL_btc-rsi,CTRD_SL_btc-rsi,CTRD_TP_btc-rsi,RSI\n" +
	"1700000000,100,105,95,102,1000,NaN,NaN,NaN,0,NaN,NaN,55\n" +
	"1700014400,102,108,101,107,1200,1,98,116,0,NaN,NaN,61\n" +
	"1700028800,107,109,104,105,900,0,NaN,NaN,1,101,113,58\n"

func TestParseChartCSV(t *testing.T) {
	t.Parallel()

	chart, err := backtest.ParseChartCSV([]byte(chartCSV))
	testutil.RequireEmptyError(t, err)

	if len(chart.Bars) != 3 {
		t.Fatalf("velas = %d, se esperaban 3", len(chart.Bars))
	}
	if got := chart.Bars[1]; got.Time != time.Unix(1700014400, 0).UTC() || got.Open != 102 || got.High != 108 || got.Low != 101 || got.Close != 107 {
		t.Errorf("vela 1 = %+v", got)
	}
	if tags := backtest.SignalTags(chart); !slices.Equal(tags, []string{"BTC-EMA", "BTC-RSI"}) {
		t.Errorf("tags = %v, se esperaban [BTC-EMA BTC-RSI]", tags)
	}

	ema := chart.Signals["BTC-EMA"]
	if ema[0].Active || !ema[1].Active || ema[1].StopLoss != 98 || ema[1].TakeProfit != 116 || ema[2].Active {
		t.Errorf("señales BTC-EMA = %+v", ema)
	}
	rsi := chart.Signals["BTC-RSI"]
	if rsi[1].Active || !rsi[2].Active || rsi[2].StopLoss != 101 || rsi[2].TakeProfit != 113 {
		t.Errorf("señales BTC-RSI = %+v", rsi)
	}
}

func TestParseChartCSVErrors(t *testing.T) {
	t.Parallel()

	header := "time,open,high,low,close,CTRD_SIGNAL_A,CTRD_SL_A,CTRD_TP_A\n"
	row1 := "1700000000,100,105,95,102,0,NaN,NaN\n"
	row2 := "1700003600,102,108,101,107,0,NaN,NaN\n"

	tests := []struct {
		name string
		csv  string
		want string
	}{
		{"columnas de distinto largo", header + row1 + row2 + "1700007200,1,2\n", "BTCHRE001"},
		{"muy pocas velas", header + row1 + row2, "BTCHRE002"},
		{"velas desordenadas", header + row2 + row1 + "1700007200,107,109,104,105,0,NaN,NaN\n", "BTCHRE003"},
		{"velas repetidas", header + row1 + row1 + row2, "BTCHRE003"},
		{"columna repetida", "time,open,high,low,close,CTRD_SIGNAL_A,CTRD_SIGNAL_a,CTRD_SL_A,CTRD_TP_A\n", "BTCHRE004"},
		{"falta close", "time,open,high,low,CTRD_SIGNAL_A,CTRD_SL_A,CTRD_TP_A\n", "BTCHRE005"},
		{"falta el stop loss del trigger", "time,open,high,low,close,CTRD_SIGNAL_A,CTRD_TP_A\n", "BTCHRE005"},
		{"sin columnas de señal", "time,open,high,low,close\n1,1,1,1,1\n2,1,1,1,1\n3,1,1,1,1\n", "BTCHRE006"},
		{"time en formato ISO", header + "2023-11-14T22:13:20Z,100,105,95,102,0,NaN,NaN\n" + row1 + row2, "BTCHRE007"},
		{"precio en cero", header + "1699996400,0,105,95,102,0,NaN,NaN\n" + row1 + row2, "BTCHRE008"},
		{"precio NaN", header + "1699996400,NaN,105,95,102,0,NaN,NaN\n" + row1 + row2, "BTCHRE008"},
		{"vela incoherente", header + "1699996400,100,99,95,102,0,NaN,NaN\n" + row1 + row2, "BTCHRE009"},
		{"señal distinta de 0 y 1", header + "1699996400,100,105,95,102,2,NaN,NaN\n" + row1 + row2, "BTCHRE010"},
		{"señal sin stop loss", header + "1699996400,100,105,95,102,1,NaN,110\n" + row1 + row2, "BTCHRE011"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Las filas extra aseguran que el error no sea por falta de velas
			// con el mismo ancho que el encabezado
			content := tt.csv
			if strings.Count(content, "\n") < 4 && tt.want != "BTCHRE002" {
				width := strings.Count(strings.SplitN(content, "\n", 2)[0], ",") + 1
				filler := strings.Repeat(",0", width-5)
				for _, seconds := range []string{"1800000000", "1800003600", "1800007200"} {
					content += seconds + ",1,1,1,1" + filler + "\n"
				}
			}
			_, err := backtest.ParseChartCSV([]byte(content))
			testutil.RequireTrackingCode(t, err, tt.want)
		})
	}
}

// bars velas seguidas desde start con la distancia de cada paso
func barsWithSteps(start time.Time, steps ...time.Duration) []backtest.Bar {
	bars := []backtest.Bar{{Time: start, Open: 1, High: 1, Low: 1, Close: 1}}
	at := start
	for _, step := range steps {
		at = at.Add(step)
		bars = append(bars, backtest.Bar{Time: at, Open: 1, High: 1, Low: 1, Close: 1})
	}
	return bars
}

func TestValidateChartTimeframe(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	fourHours := 4 * time.Hour

	t.Run("velas exactas sin huecos", func(t *testing.T) {
		t.Parallel()
		gaps, err := backtest.ValidateChartTimeframe(barsWithSteps(start, fourHours, fourHours, fourHours), fourHours, false)
		testutil.RequireEmptyError(t, err)
		if len(gaps) != 0 {
			t.Errorf("huecos = %v, no se esperaban", gaps)
		}
	})

	t.Run("cambio de horario en velas diarias", func(t *testing.T) {
		t.Parallel()
		_, err := backtest.ValidateChartTimeframe(barsWithSteps(start, day, day-time.Hour, day+time.Hour), day, false)
		testutil.RequireEmptyError(t, err)
	})

	t.Run("fin de semana aceptado con allowSessionGaps", func(t *testing.T) {
		t.Parallel()
		gaps, err := backtest.ValidateChartTimeframe(barsWithSteps(start, 3*day, day, day, day, day), day, true)
		testutil.RequireEmptyError(t, err)
		if len(gaps) != 1 || gaps[0].MissingBars != 2 || !gaps[0].From.Equal(start) || !gaps[0].To.Equal(start.Add(3*day)) {
			t.Errorf("huecos = %+v, se esperaba uno de 2 velas", gaps)
		}
	})

	tests := []struct {
		name      string
		bars      []backtest.Bar
		timeframe time.Duration
		allowGaps bool
		want      string
	}{
		{"temporalidad en cero", barsWithSteps(start, day), 0, false, "BTCHRE012"},
		{"velas mas cercanas que la temporalidad", barsWithSteps(start, time.Hour, time.Hour), fourHours, false, "BTCHRE013"},
		{"hueco sin allowSessionGaps", barsWithSteps(start, fourHours, 3*fourHours, fourHours), fourHours, false, "BTCHRE014"},
		{"la mayoria son huecos", barsWithSteps(start, 2*day, 2*day, day), day, true, "BTCHRE015"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := backtest.ValidateChartTimeframe(tt.bars, tt.timeframe, tt.allowGaps)
			testutil.RequireTrackingCode(t, err, tt.want)
		})
	}
}

func TestValidateSignalLevels(t *testing.T) {
	t.Parallel()

	bars := []backtest.Bar{{Close: 100}, {Close: 100}}
	long := []backtest.TriggerSignal{{}, {Active: true, StopLoss: 90, TakeProfit: 110}}
	short := []backtest.TriggerSignal{{}, {Active: true, StopLoss: 110, TakeProfit: 90}}

	testutil.RequireEmptyError(t, backtest.ValidateSignalLevels(bars, long, true))
	testutil.RequireEmptyError(t, backtest.ValidateSignalLevels(bars, short, false))
	testutil.RequireTrackingCode(t, backtest.ValidateSignalLevels(bars, short, true), "BTCHRE017")
	testutil.RequireTrackingCode(t, backtest.ValidateSignalLevels(bars, long, false), "BTCHRE017")
	testutil.RequireTrackingCode(t, backtest.ValidateSignalLevels(bars, long[:1], true), "BTCHRE016")
}
