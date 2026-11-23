package models_test

import (
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNewBacktestRunModel(t *testing.T) {
	t.Parallel()

	dataset := models.BacktestDatasetsModelType{
		ID: primitive.NewObjectID(), FileSha256: "abc",
		FirstBarAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), LastBarAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	trigger := models.TriggersModelType{
		ID: primitive.NewObjectID(), Tag: "BTC-EMA", Name: "EMA", TagAsset: "BTC", TagAction: cmm.ActionTagBuy,
		TagCandlestickTimeframe: "4-HOURS", MaxCandlestickQty: 30,
	}
	pineSha := "def"
	settings := models.BacktestSettings{FeeBps: 10, SlippageBps: 5, ConditionSource: cmm.BacktestConditionSourceAuto}

	run, err := models.NewBacktestRunModel(dataset, trigger, &pineSha, settings, models.RecordOwnerData{})
	testutil.RequireEmptyError(t, err)

	if run.TagStatus != cmm.BacktestRunStatusQueued.Tag || run.IDStatus != cmm.BacktestRunStatusQueued.ID {
		t.Errorf("status = %s, se esperaba %s", run.TagStatus, cmm.BacktestRunStatusQueued.Tag)
	}
	if run.IDDataset != dataset.ID || run.DatasetSha256 != "abc" || !run.DatasetLastBarAt.Equal(dataset.LastBarAt) {
		t.Errorf("copia del dataset = %s %s %s", run.IDDataset.Hex(), run.DatasetSha256, run.DatasetLastBarAt)
	}
	if run.IDTrigger != trigger.ID || run.TagTrigger != "BTC-EMA" || run.MaxCandlestickQty != 30 || *run.PineCodeSha256 != "def" {
		t.Errorf("copia del trigger = %+v", run)
	}
	if run.Conditions == nil || run.Trades == nil || run.Results != nil || run.StartedAt != nil || run.Error != nil {
		t.Errorf("resultado inicial = conditions %v trades %v results %v", run.Conditions, run.Trades, run.Results)
	}
}

func TestNewBacktestDatasetModel(t *testing.T) {
	t.Parallel()

	asset := models.AssetsModelType{ID: primitive.NewObjectID(), Tag: "BTC", Name: "Bitcoin"}
	timeframe := models.CatalogsModelType{ID: primitive.NewObjectID(), Tag: "4-HOURS", Name: "4 horas"}
	file := models.BacktestDatasetFileData{FileSha256: "abc", RowCount: 10}

	dataset, err := models.NewBacktestDatasetModel(asset, timeframe, file, models.RecordOwnerData{})
	testutil.RequireEmptyError(t, err)

	if dataset.IDAsset != asset.ID || dataset.Asset != "Bitcoin" || dataset.TagCandlestickTimeframe != "4-HOURS" || dataset.RowCount != 10 {
		t.Errorf("dataset = %+v", dataset)
	}
	if dataset.Gaps == nil || dataset.SignalTags == nil {
		t.Errorf("gaps = %v y signalTags = %v, se esperaban [] en vez de nil", dataset.Gaps, dataset.SignalTags)
	}
}
