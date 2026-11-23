package backtest_test

import (
	"testing"
	"time"

	"c_trd/backtest"
	cmm "c_trd/common"
	"c_trd/models"
)

// closedTrade operacion cerrada con R y resultado %, que sale hours horas despues de simulationStart
func closedTrade(reason string, r float64, returnPct float64, mae float64, mfe float64, hours int) models.BacktestTrade {
	exitAt := simulationStart.Add(time.Duration(hours) * time.Hour)
	return models.BacktestTrade{ExitReason: reason, ExitAt: &exitAt, RMultiple: &r, ReturnPct: &returnPct, MaePct: &mae, MfePct: &mfe}
}

func TestComputeMetrics(t *testing.T) {
	t.Parallel()

	// Por orden de salida: +2, -1, -1, +1 -> R acumulado 2, 1, 0, 1: caida maxima 2. Llegan desordenadas
	result := backtest.SimulationResult{
		Signals:        7,
		SkippedSignals: 1,
		Trades: []models.BacktestTrade{
			closedTrade(cmm.BacktestExitStopLoss, -1, -1, 1, 0.5, 3),
			closedTrade(cmm.BacktestExitTakeProfit, 2, 4, 0.5, 4, 1),
			closedTrade(cmm.BacktestExitExpired, 1, 2, 2.5, 3, 4),
			closedTrade(cmm.BacktestExitStopLoss, -1, -2, 1.5, 0.2, 2),
			{ExitReason: cmm.BacktestExitCanceled},
			{ExitReason: cmm.BacktestExitEndOfData},
		},
	}

	metrics := backtest.ComputeMetrics(result)
	if metrics.Signals != 7 || metrics.SkippedSignals != 1 || metrics.TradeCount != 4 || metrics.Wins != 2 || metrics.Losses != 2 {
		t.Fatalf("conteos = %+v", metrics)
	}
	if metrics.TakeProfits != 1 || metrics.StopLosses != 2 || metrics.Expired != 1 || metrics.Canceled != 1 || metrics.EndOfData != 1 {
		t.Errorf("salidas = %+v", metrics)
	}
	requireFloat(t, "hit rate", metrics.HitRate, 0.5)
	requireFloat(t, "R promedio", metrics.AverageR, 0.25)
	requireFloat(t, "esperanza %", metrics.ExpectancyPct, 0.75)
	requireFloat(t, "R ganadoras", metrics.AverageWinR, 1.5)
	requireFloat(t, "R perdedoras", metrics.AverageLossR, -1)
	requireFloat(t, "R total", metrics.TotalR, 1)
	requireFloat(t, "caida maxima", metrics.MaxDrawdownR, 2)
	requireFloat(t, "MAE maximo", metrics.MaxAdverseMovePct, 2.5)
	requireFloat(t, "MFE maximo", metrics.MaxFavourableMovePct, 4)
}

func TestComputeMetricsWithoutClosedTrades(t *testing.T) {
	t.Parallel()

	metrics := backtest.ComputeMetrics(backtest.SimulationResult{Signals: 1, Trades: []models.BacktestTrade{{ExitReason: cmm.BacktestExitNotActivated}}})
	if metrics.TradeCount != 0 || metrics.NotActivated != 1 || metrics.HitRate != nil || metrics.AverageR != nil || metrics.MaxDrawdownR != nil {
		t.Errorf("metricas = %+v, se esperaban conteos sin metricas de resultado", metrics)
	}
}
