package backtest

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
)

// Lectura y validacion del CSV "Export chart data" de TradingView. Columnas: time (unix en segundos, UTC), open, high,
// low, close, opcionalmente Volume, y por cada trigger del grafico CTRD_SIGNAL_<tag>, CTRD_SL_<tag> y CTRD_TP_<tag>.
// Los nombres de columna se comparan sin distinguir mayusculas

// Minimo de velas de un CSV
const chartMinBars = 3

// Maximo de problemas que se listan en un error, el resto solo se cuenta
const chartMaxListedProblems = 5

// Bar vela del CSV. Time es la apertura en UTC
type Bar struct {
	Time  time.Time
	Open  float64
	High  float64
	Low   float64
	Close float64
}

// TriggerSignal valores de las columnas de un trigger en una vela. StopLoss y TakeProfit solo se leen si Active
type TriggerSignal struct {
	Active     bool
	StopLoss   float64
	TakeProfit float64
}

// Chart velas del CSV y las señales de cada trigger. Signals tiene un valor por vela (mismo indice que Bars) y la llave
// es el tag del trigger en mayusculas
type Chart struct {
	Bars    []Bar
	Signals map[string][]TriggerSignal
}

// chartColumns indice de cada columna del CSV. Las de los triggers por tag en mayusculas
type chartColumns struct {
	time       int
	open       int
	high       int
	low        int
	close      int
	signal     map[string]int
	stopLoss   map[string]int
	takeProfit map[string]int
}

// SignalTags tags (en mayusculas) de los triggers con columnas en el CSV, ordenados
func SignalTags(chart Chart) []string {
	tags := make([]string, 0, len(chart.Signals))
	for tag := range chart.Signals {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

// ParseChartCSV lee el CSV: valida las columnas, que las velas sean coherentes y esten ordenadas sin repetirse y que
// cada trigger tenga sus tres columnas con stop loss y take profit en las velas con señal
func ParseChartCSV(content []byte) (Chart, *cmm.ErrorHandler) {
	reader := csv.NewReader(bytes.NewReader(content))
	reader.TrimLeadingSpace = true
	records, err := reader.ReadAll()
	if err != nil {
		return Chart{}, cmm.NewErrorHandler(err, fmt.Sprintf("El CSV no se puede leer: %v", err), cmm.LevelError, "BTCHRE001")
	}
	if len(records)-1 < chartMinBars {
		return Chart{}, cmm.NewErrorHandler(fmt.Errorf("rows %d", len(records)-1), fmt.Sprintf("El CSV debe tener al menos %d velas", chartMinBars), cmm.LevelError, "BTCHRE002")
	}

	columns, errHeader := parseChartHeader(records[0])
	if !errHeader.IsEmtpy() {
		return Chart{}, errHeader
	}

	chart := Chart{Bars: make([]Bar, 0, len(records)-1), Signals: map[string][]TriggerSignal{}}
	for tag := range columns.signal {
		chart.Signals[tag] = make([]TriggerSignal, 0, len(records)-1)
	}

	for index, record := range records[1:] {
		line := index + 2
		bar, errBar := parseChartBar(record, columns, line)
		if !errBar.IsEmtpy() {
			return Chart{}, errBar
		}

		previous := len(chart.Bars) - 1
		if previous >= 0 && !bar.Time.After(chart.Bars[previous].Time) {
			return Chart{}, cmm.NewErrorHandler(fmt.Errorf("line %d time %s", line, bar.Time.Format(time.RFC3339)), fmt.Sprintf("Linea %d: las velas deben estar ordenadas por fecha sin repetirse (%s)", line, bar.Time.Format(time.RFC3339)), cmm.LevelError, "BTCHRE003")
		}
		chart.Bars = append(chart.Bars, bar)

		for tag := range columns.signal {
			signal, errSignal := parseTriggerSignal(record, columns, tag, line)
			if !errSignal.IsEmtpy() {
				return Chart{}, errSignal
			}
			chart.Signals[tag] = append(chart.Signals[tag], signal)
		}
	}

	return chart, cmm.NewEmptyErrorHandler()
}

// parseChartHeader ubica las columnas. Exige time, open, high, low y close y que cada CTRD_SIGNAL_<tag> tenga su
// CTRD_SL_<tag> y CTRD_TP_<tag>. Las demas columnas (Volume, otros indicadores) se ignoran
//
// TODO: confirmar con un export real de TradingView con dos o mas indicadores de triggers en el mismo grafico que cada
// columna se llama exactamente como el titulo del plot (sin prefijo del indicador ni sufijos por titulos repetidos).
// Si TradingView cambia los nombres, ajustar aqui como se reconoce el tag y agregar el export a tests/backtest/testdata
func parseChartHeader(header []string) (chartColumns, *cmm.ErrorHandler) {
	columns := chartColumns{time: -1, open: -1, high: -1, low: -1, close: -1, signal: map[string]int{}, stopLoss: map[string]int{}, takeProfit: map[string]int{}}
	seen := map[string]bool{}

	for index, raw := range header {
		name := strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff")))
		if seen[name] {
			return columns, cmm.NewErrorHandler(fmt.Errorf("column %q", name), fmt.Sprintf("La columna %s esta repetida: cada trigger debe usar su propio tag en los titulos de los plot", name), cmm.LevelError, "BTCHRE004")
		}
		seen[name] = true

		switch {
		case name == "TIME":
			columns.time = index
		case name == "OPEN":
			columns.open = index
		case name == "HIGH":
			columns.high = index
		case name == "LOW":
			columns.low = index
		case name == "CLOSE":
			columns.close = index
		case strings.HasPrefix(name, cmm.BacktestSignalColumnPrefix) && len(name) > len(cmm.BacktestSignalColumnPrefix):
			columns.signal[strings.TrimPrefix(name, cmm.BacktestSignalColumnPrefix)] = index
		case strings.HasPrefix(name, cmm.BacktestStopLossColumnPrefix) && len(name) > len(cmm.BacktestStopLossColumnPrefix):
			columns.stopLoss[strings.TrimPrefix(name, cmm.BacktestStopLossColumnPrefix)] = index
		case strings.HasPrefix(name, cmm.BacktestTakeProfitColumnPrefix) && len(name) > len(cmm.BacktestTakeProfitColumnPrefix):
			columns.takeProfit[strings.TrimPrefix(name, cmm.BacktestTakeProfitColumnPrefix)] = index
		}
	}

	missing := []string{}
	required := []struct {
		name  string
		index int
	}{{"time", columns.time}, {"open", columns.open}, {"high", columns.high}, {"low", columns.low}, {"close", columns.close}}
	for _, column := range required {
		if column.index < 0 {
			missing = append(missing, column.name)
		}
	}
	for tag := range columns.signal {
		if _, has := columns.stopLoss[tag]; !has {
			missing = append(missing, cmm.BacktestStopLossColumnPrefix+tag)
		}
		if _, has := columns.takeProfit[tag]; !has {
			missing = append(missing, cmm.BacktestTakeProfitColumnPrefix+tag)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return columns, cmm.NewErrorHandler(fmt.Errorf("missing %v", missing), "Faltan columnas en el CSV: "+strings.Join(missing, ", "), cmm.LevelError, "BTCHRE005")
	}
	if len(columns.signal) == 0 {
		return columns, cmm.NewErrorHandler(errors.New("no signal columns"), "El CSV no tiene columnas "+cmm.BacktestSignalColumnPrefix+"<tag>: agrega al grafico los indicadores de los triggers antes de exportar", cmm.LevelError, "BTCHRE006")
	}

	return columns, cmm.NewEmptyErrorHandler()
}

// parseChartBar lee la vela de una fila: time en segundos unix y precios positivos y coherentes (low <= open, close <= high)
func parseChartBar(record []string, columns chartColumns, line int) (Bar, *cmm.ErrorHandler) {
	seconds, err := strconv.ParseInt(strings.TrimSpace(record[columns.time]), 10, 64)
	if err != nil {
		return Bar{}, cmm.NewErrorHandler(err, fmt.Sprintf("Linea %d: time debe ser un timestamp UNIX en segundos (exporta con el formato de tiempo UNIX)", line), cmm.LevelError, "BTCHRE007")
	}

	prices := [4]float64{}
	for i, index := range []int{columns.open, columns.high, columns.low, columns.close} {
		price, errPrice := strconv.ParseFloat(strings.TrimSpace(record[index]), 64)
		if errPrice != nil || math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
			return Bar{}, cmm.NewErrorHandler(fmt.Errorf("line %d value %q", line, record[index]), fmt.Sprintf("Linea %d: los precios deben ser numeros mayores a 0 (%q)", line, record[index]), cmm.LevelError, "BTCHRE008")
		}
		prices[i] = price
	}

	bar := Bar{Time: time.Unix(seconds, 0).UTC(), Open: prices[0], High: prices[1], Low: prices[2], Close: prices[3]}
	isCoherent := bar.Low <= bar.Open && bar.Low <= bar.Close && bar.High >= bar.Open && bar.High >= bar.Close
	if !isCoherent {
		return Bar{}, cmm.NewErrorHandler(fmt.Errorf("line %d bar %+v", line, bar), fmt.Sprintf("Linea %d: la vela no es coherente, low debe ser el minimo y high el maximo", line), cmm.LevelError, "BTCHRE009")
	}

	return bar, cmm.NewEmptyErrorHandler()
}

// parseTriggerSignal lee las columnas de un trigger. La señal vacia o NaN (indicador sin datos) es 0; si es 1 el stop
// loss y el take profit deben ser numeros mayores a 0
func parseTriggerSignal(record []string, columns chartColumns, tag string, line int) (TriggerSignal, *cmm.ErrorHandler) {
	raw := strings.TrimSpace(record[columns.signal[tag]])
	if raw == "" || strings.EqualFold(raw, "nan") {
		return TriggerSignal{}, cmm.NewEmptyErrorHandler()
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || (value != 0 && value != 1) {
		return TriggerSignal{}, cmm.NewErrorHandler(fmt.Errorf("line %d value %q", line, raw), fmt.Sprintf("Linea %d: %s%s debe ser 0 o 1 (%q)", line, cmm.BacktestSignalColumnPrefix, tag, raw), cmm.LevelError, "BTCHRE010")
	}
	if value == 0 {
		return TriggerSignal{}, cmm.NewEmptyErrorHandler()
	}

	levels := [2]float64{}
	for i, index := range []int{columns.stopLoss[tag], columns.takeProfit[tag]} {
		level, errLevel := strconv.ParseFloat(strings.TrimSpace(record[index]), 64)
		if errLevel != nil || math.IsNaN(level) || math.IsInf(level, 0) || level <= 0 {
			return TriggerSignal{}, cmm.NewErrorHandler(fmt.Errorf("line %d value %q", line, record[index]), fmt.Sprintf("Linea %d: el trigger %s tiene señal sin stop loss o take profit valido (%q)", line, tag, record[index]), cmm.LevelError, "BTCHRE011")
		}
		levels[i] = level
	}

	signal := TriggerSignal{Active: true, StopLoss: levels[0], TakeProfit: levels[1]}
	return signal, cmm.NewEmptyErrorHandler()
}

// ValidateChartTimeframe valida que la distancia entre velas sea la de la temporalidad. Tolera 1 hora en temporalidades
// de un dia o mas (cambio de horario) y 3 dias en la mensual (meses de 28 a 31 dias). Una distancia mayor es un hueco:
// sin allowSessionGaps es un error y con allowSessionGaps se retorna en la lista. Si mas de la mitad de las distancias
// son huecos la temporalidad no es la del CSV
func ValidateChartTimeframe(bars []Bar, timeframe time.Duration, allowSessionGaps bool) ([]models.BacktestGap, *cmm.ErrorHandler) {
	gaps := []models.BacktestGap{}
	if timeframe <= 0 {
		return gaps, cmm.NewErrorHandler(fmt.Errorf("timeframe %s", timeframe), "La temporalidad debe ser mayor a 0", cmm.LevelError, "BTCHRE012")
	}

	tolerance := timeframeTolerance(timeframe)
	for i := 1; i < len(bars); i++ {
		step := bars[i].Time.Sub(bars[i-1].Time)
		if step < timeframe-tolerance {
			return gaps, cmm.NewErrorHandler(fmt.Errorf("step %s timeframe %s at %s", step, timeframe, bars[i].Time.Format(time.RFC3339)), fmt.Sprintf("Hay velas a %s de distancia (%s), la temporalidad del trigger es %s", step, bars[i].Time.Format(time.RFC3339), timeframe), cmm.LevelError, "BTCHRE013")
		}
		if step <= timeframe+tolerance {
			continue
		}

		missing := int(math.Round(float64(step)/float64(timeframe))) - 1
		gap := models.BacktestGap{From: bars[i-1].Time, To: bars[i].Time, MissingBars: max(missing, 1)}
		if !allowSessionGaps {
			return gaps, cmm.NewErrorHandler(fmt.Errorf("gap %s - %s", gap.From.Format(time.RFC3339), gap.To.Format(time.RFC3339)), fmt.Sprintf("Faltan velas entre %s y %s. Si el mercado tiene sesiones (acciones), sube el CSV con allowSessionGaps", gap.From.Format(time.RFC3339), gap.To.Format(time.RFC3339)), cmm.LevelError, "BTCHRE014")
		}
		gaps = append(gaps, gap)
	}

	steps := len(bars) - 1
	if steps > 0 && len(gaps)*2 > steps {
		return gaps, cmm.NewErrorHandler(fmt.Errorf("gaps %d steps %d", len(gaps), steps), fmt.Sprintf("%d de %d distancias entre velas son huecos: la temporalidad del CSV no es %s", len(gaps), steps, timeframe), cmm.LevelError, "BTCHRE015")
	}

	return gaps, cmm.NewEmptyErrorHandler()
}

// timeframeTolerance margen de la distancia entre velas segun la temporalidad
func timeframeTolerance(timeframe time.Duration) time.Duration {
	if timeframe >= 28*24*time.Hour {
		return 3 * 24 * time.Hour
	}
	if timeframe >= 24*time.Hour {
		return time.Hour
	}
	return 0
}

// ValidateSignalLevels valida que en cada vela con señal el stop loss y el take profit esten del lado correcto del
// cierre segun la accion: compra SL < close < TP y venta TP < close < SL
func ValidateSignalLevels(bars []Bar, signals []TriggerSignal, isLong bool) *cmm.ErrorHandler {
	if len(bars) != len(signals) {
		return cmm.NewErrorHandler(fmt.Errorf("bars %d signals %d", len(bars), len(signals)), "Las señales no corresponden a las velas", cmm.LevelFatal, "BTCHRE016")
	}

	problems := []string{}
	count := 0
	for i, signal := range signals {
		if !signal.Active {
			continue
		}

		closePrice := bars[i].Close
		isValid := signal.StopLoss < closePrice && closePrice < signal.TakeProfit
		if !isLong {
			isValid = signal.TakeProfit < closePrice && closePrice < signal.StopLoss
		}
		if isValid {
			continue
		}

		count++
		if len(problems) < chartMaxListedProblems {
			problems = append(problems, fmt.Sprintf("%s (close %v, SL %v, TP %v)", bars[i].Time.Format(time.RFC3339), closePrice, signal.StopLoss, signal.TakeProfit))
		}
	}

	if count > 0 {
		side := "compra: SL < close < TP"
		if !isLong {
			side = "venta: TP < close < SL"
		}
		return cmm.NewErrorHandler(fmt.Errorf("%d invalid signals", count), fmt.Sprintf("%d señales con stop loss o take profit del lado equivocado (%s): %s", count, side, strings.Join(problems, "; ")), cmm.LevelError, "BTCHRE017")
	}

	return cmm.NewEmptyErrorHandler()
}
