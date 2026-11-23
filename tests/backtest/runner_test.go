package backtest_test

import (
	"maps"
	"testing"

	"c_trd/backtest"
	cmm "c_trd/common"
	"c_trd/providers"
	"c_trd/tests/testutil"
)

func TestIsLongAction(t *testing.T) {
	t.Parallel()

	isLong, err := backtest.IsLongAction(cmm.ActionTagBuy)
	testutil.RequireEmptyError(t, err)
	if !isLong {
		t.Error("BUY debe ser compra")
	}

	isLong, err = backtest.IsLongAction(cmm.ActionTagSell)
	testutil.RequireEmptyError(t, err)
	if isLong {
		t.Error("SELL debe ser venta")
	}

	_, err = backtest.IsLongAction("HOLD")
	testutil.RequireTrackingCode(t, err, "BTRUNE006")
}

func TestHistoryRequest(t *testing.T) {
	t.Parallel()

	original := providers.HTTPRequestConfig{
		URL:       "https://api.stlouisfed.org/fred/series/observations",
		QueryData: map[string]string{"series_id": "UNRATE", "limit": "6", "sort_order": "desc", "observation_start": "2024-01-01"},
	}
	before := maps.Clone(original.QueryData)

	request := backtest.HistoryRequest(original, []string{"limit", "observation_start"})
	want := map[string]string{"series_id": "UNRATE", "sort_order": "desc"}
	if !maps.Equal(request.QueryData, want) {
		t.Errorf("query = %v, se esperaba %v", request.QueryData, want)
	}
	if !maps.Equal(original.QueryData, before) {
		t.Errorf("la peticion original cambio: %v", original.QueryData)
	}

	withoutQuery := backtest.HistoryRequest(providers.HTTPRequestConfig{URL: "http://localhost:8080/x"}, []string{"limit"})
	if withoutQuery.QueryData != nil {
		t.Errorf("query = %v, se esperaba nil", withoutQuery.QueryData)
	}
}
