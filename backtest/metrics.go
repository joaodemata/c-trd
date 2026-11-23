package backtest

import (
	"sort"

	cmm "c_trd/common"
	"c_trd/models"
)

// ComputeMetrics metricas de una variante. Los conteos usan todas las oportunidades y las metricas de resultado solo
// las operaciones cerradas (TP, SL y EXPIRED): END_OF_DATA no tiene salida y las demas no tuvieron operacion
func ComputeMetrics(result SimulationResult) models.BacktestMetrics {
	metrics := models.BacktestMetrics{Signals: result.Signals, SkippedSignals: result.SkippedSignals}

	closed := make([]models.BacktestTrade, 0, len(result.Trades))
	for _, trade := range result.Trades {
		switch trade.ExitReason {
		case cmm.BacktestExitTakeProfit:
			metrics.TakeProfits++
			closed = append(closed, trade)
		case cmm.BacktestExitStopLoss:
			metrics.StopLosses++
			closed = append(closed, trade)
		case cmm.BacktestExitExpired:
			metrics.Expired++
			closed = append(closed, trade)
		case cmm.BacktestExitCanceled:
			metrics.Canceled++
		case cmm.BacktestExitNotActivated:
			metrics.NotActivated++
		case cmm.BacktestExitInvalidEntry:
			metrics.InvalidEntries++
		case cmm.BacktestExitEndOfData:
			metrics.EndOfData++
		}
	}

	metrics.TradeCount = len(closed)
	if len(closed) == 0 {
		return metrics
	}

	// La curva de R acumulado se arma en el orden en que se cerraron las operaciones
	sort.SliceStable(closed, func(i, j int) bool { return closed[i].ExitAt.Before(*closed[j].ExitAt) })

	var totalR, totalReturnPct, winR, lossR, equity, peak, maxDrawdown, maxAdverse, maxFavourable float64
	for _, trade := range closed {
		r := *trade.RMultiple
		totalR += r
		totalReturnPct += *trade.ReturnPct
		if r > 0 {
			metrics.Wins++
			winR += r
		} else {
			metrics.Losses++
			lossR += r
		}

		equity += r
		peak = max(peak, equity)
		maxDrawdown = max(maxDrawdown, peak-equity)
		maxAdverse = max(maxAdverse, *trade.MaePct)
		maxFavourable = max(maxFavourable, *trade.MfePct)
	}

	count := float64(len(closed))
	hitRate := float64(metrics.Wins) / count
	expectancyPct := totalReturnPct / count
	averageR := totalR / count
	metrics.HitRate = &hitRate
	metrics.ExpectancyPct = &expectancyPct
	metrics.AverageR = &averageR
	metrics.TotalR = &totalR
	metrics.MaxDrawdownR = &maxDrawdown
	metrics.MaxAdverseMovePct = &maxAdverse
	metrics.MaxFavourableMovePct = &maxFavourable
	if metrics.Wins > 0 {
		averageWinR := winR / float64(metrics.Wins)
		metrics.AverageWinR = &averageWinR
	}
	if metrics.Losses > 0 {
		averageLossR := lossR / float64(metrics.Losses)
		metrics.AverageLossR = &averageLossR
	}

	return metrics
}
