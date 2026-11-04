package agents

import (
	"fmt"
	"math"
	"sort"
)

// ---------- Metricas del backtest (el LLM nunca calcula numeros) ----------

// tradeR resultado del trade en multiplos de R (riesgo inicial = distancia entrada-stop)
func tradeR(t Trade) float64 {
	risk := math.Abs(t.EntryPrice - t.StopLoss)
	if risk == 0 {
		return 0
	}
	move := t.ExitPrice - t.EntryPrice
	if !t.IsLong {
		move = -move
	}
	fees := t.EntryPrice * t.FeesPct / 100
	return (move - fees) / risk
}

// ComputeMetrics calcula las metricas. riskPerTradePct = % del capital arriesgado por trade
// (curva de capital en R), years = duracion del periodo testeado, bhCAGR = CAGR del buy and hold
func ComputeMetrics(trades []Trade, riskPerTradePct, years, bhCAGR float64) BacktestMetrics {
	m := BacktestMetrics{Trades: len(trades), BuyAndHoldCAGRPct: bhCAGR}
	if len(trades) == 0 {
		return m
	}

	var wins, losses int
	var sumWinR, sumLossR, grossWin, grossLoss, sumBars float64
	var exitsByTime int
	rs := make([]float64, 0, len(trades))
	equity, peak, maxDD := 100.0, 100.0, 0.0

	for _, t := range trades {
		r := tradeR(t)
		rs = append(rs, r)
		sumBars += float64(t.BarsHeld)
		if t.ExitByTime {
			exitsByTime++
		}
		if r > 0 {
			wins++
			sumWinR += r
			grossWin += r
		} else {
			losses++
			sumLossR += r
			grossLoss += -r
		}
		equity *= 1 + r*riskPerTradePct/100
		peak = math.Max(peak, equity)
		maxDD = math.Max(maxDD, (peak-equity)/peak*100)
	}

	n := float64(len(trades))
	m.WinRate = float64(wins) / n * 100
	m.ExpectancyR = (sumWinR + sumLossR) / n
	if wins > 0 {
		m.AvgWinR = sumWinR / float64(wins)
	}
	if losses > 0 {
		m.AvgLossR = sumLossR / float64(losses)
	}
	if grossLoss > 0 {
		m.ProfitFactor = grossWin / grossLoss
	} else {
		m.ProfitFactor = math.Inf(1)
	}
	m.MaxDrawdownPct = maxDD
	m.AvgBarsHeld = sumBars / n
	m.PctExitsByTime = float64(exitsByTime) / n * 100
	if years > 0 && equity > 0 {
		m.CAGRPct = (math.Pow(equity/100, 1/years) - 1) * 100
	}
	m.TopTradesProfitPct = topTradesShare(rs, grossWin)
	return m
}

// topTradesShare % del beneficio bruto que aportan el 10% de los mejores trades
func topTradesShare(rs []float64, grossWin float64) float64 {
	if grossWin <= 0 {
		return 0
	}
	sorted := append([]float64(nil), rs...)
	sort.Sort(sort.Reverse(sort.Float64Slice(sorted)))
	top := int(math.Ceil(float64(len(sorted)) * 0.1))
	var sum float64
	for i := 0; i < top; i++ {
		if sorted[i] > 0 {
			sum += sorted[i]
		}
	}
	return sum / grossWin * 100
}

// SplitInOutOfSample separa los trades ordenados por fecha: 70% in-sample, 30% out-of-sample
func SplitInOutOfSample(trades []Trade) (inSample, outOfSample []Trade) {
	cut := int(float64(len(trades)) * 0.7)
	return trades[:cut], trades[cut:]
}

// ---------- Gates duros: si uno falla, el agente 4 no puede aprobar ----------

func EvaluateGates(m, oos BacktestMetrics, th Thresholds) []GateResult {
	return []GateResult{
		{Name: "min_trades", Threshold: float64(th.MinTrades), Value: float64(m.Trades), Passed: m.Trades >= th.MinTrades},
		{Name: "min_profit_factor", Threshold: th.MinProfitFactor, Value: m.ProfitFactor, Passed: m.ProfitFactor >= th.MinProfitFactor},
		{Name: "min_expectancy_r", Threshold: th.MinExpectancyR, Value: m.ExpectancyR, Passed: m.ExpectancyR >= th.MinExpectancyR},
		{Name: "max_drawdown_pct", Threshold: th.MaxDrawdownPct, Value: m.MaxDrawdownPct, Passed: m.MaxDrawdownPct <= th.MaxDrawdownPct},
		{Name: "min_oos_expectancy_r", Threshold: th.MinOOSExpectancyR, Value: oos.ExpectancyR, Passed: oos.ExpectancyR >= th.MinOOSExpectancyR},
		{Name: "max_top_trades_profit_pct", Threshold: th.MaxTopTradesProfit, Value: m.TopTradesProfitPct, Passed: m.TopTradesProfitPct <= th.MaxTopTradesProfit},
	}
}

func AllGatesPassed(gates []GateResult) bool {
	for _, g := range gates {
		if !g.Passed {
			return false
		}
	}
	return true
}

// EnforceJudgeDecision aplica la regla dura sobre la salida del agente 4:
// con un gate fallido nunca puede quedar APPROVED, ni con score < 65
func EnforceJudgeDecision(out FinancialJudgeOutput, gates []GateResult) FinancialJudgeOutput {
	gatesPassed := AllGatesPassed(gates)
	lowScore := out.Score < 65
	if out.Decision == "APPROVED" && (!gatesPassed || lowScore) {
		out.Decision = "REJECTED"
		out.Reasons = append(out.Reasons, JudgeReason{
			Metric: "gates", Value: fmt.Sprintf("gates_ok=%v score=%d", gatesPassed, out.Score),
			Impact: "NEGATIVE", Comment: "Aprobacion anulada por el backend: gate fallido o score < 65",
		})
	}
	return out
}
