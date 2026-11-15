package services_test

import (
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/services"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// linkedTx transaccion ligada a una oportunidad (usa portfolioTx de operation_service_test.go)
func linkedTx(transaction models.PortfolioTransactionsModelType, idOpportunity primitive.ObjectID) models.PortfolioTransactionsModelType {
	transaction.IDOpportunity = &idOpportunity
	return transaction
}

// cashTx deposito o retiro, sin asset
func cashTx(status cmm.CatalogStatus, day int, value float64) models.PortfolioTransactionsModelType {
	executedAt := time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC)
	return models.PortfolioTransactionsModelType{ID: primitive.NewObjectID(), TagTypeTransaction: status.Tag, Value: value, ExecutedAt: executedAt, CreatedDate: executedAt}
}

// requirePointer compara un monto opcional: want nil exige nil
func requirePointer(t *testing.T, field string, got *float64, want *float64) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Errorf("%s = %v, se esperaba null", field, *got)
		}
		return
	}
	if got == nil {
		t.Errorf("%s = null, se esperaba %v", field, *want)
		return
	}
	requireAmount(t, field, *got, *want)
}

func amount(value float64) *float64 { return &value }

func TestBuildPortfolioRoiService(t *testing.T) {
	t.Parallel()

	btc, eth := primitive.NewObjectID(), primitive.NewObjectID()
	idOpportunity := primitive.NewObjectID()
	now := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)

	// Compra 2 BTC a 100 y vende 1 a 150 (comision 1) ligadas a la oportunidad; compra ETH sin oportunidad
	transactions := []models.PortfolioTransactionsModelType{
		cashTx(cmm.TransactionTypeDeposit, 1, 1000),
		linkedTx(portfolioTx(btc, cmm.TransactionTypeBuy, 2, 2, 200, 0), idOpportunity),
		linkedTx(portfolioTx(btc, cmm.TransactionTypeSell, 3, 1, 150, 1), idOpportunity),
		portfolioTx(eth, cmm.TransactionTypeBuy, 4, 2, 400, 0),
	}
	portfolio := models.PortfolioModelType{
		ID: primitive.NewObjectID(),
		Assets: []models.PortfolioAsset{
			{IDAsset: btc, TagAsset: "ASSET-BTC", Quantity: 1, AverageCost: 100, CostBasis: 100, RealizedPnL: 49},
			{IDAsset: eth, TagAsset: "ASSET-ETH", Quantity: 2, AverageCost: 200, CostBasis: 400},
		},
	}
	// ETH no tiene precio: se valora a su costo
	prices := map[primitive.ObjectID]float64{btc: 300}
	opportunities := map[primitive.ObjectID]models.OpportunitiesModelType{
		idOpportunity: {ID: idOpportunity, TagTrigger: "TRIGGER-RSI", TagAction: "ACTION-BUY", TagStatus: cmm.OpportunityStatusFinished.Tag,
			PriceActiveOportunity: amount(100), PriceAchieved: amount(160)},
	}

	roi := services.BuildPortfolioRoiService(portfolio, transactions, prices, opportunities, now)

	if !roi.Partial || len(roi.Warnings) != 1 {
		t.Errorf("partial = %v, warnings = %v; falta el precio de ETH", roi.Partial, roi.Warnings)
	}
	if roi.IDPortfolio != portfolio.ID || !roi.CalculatedAt.Equal(now) {
		t.Errorf("portfolio = %s, calculado = %v", roi.IDPortfolio.Hex(), roi.CalculatedAt)
	}

	// Assets
	if len(roi.Assets) != 2 {
		t.Fatalf("assets = %d, se esperaban 2", len(roi.Assets))
	}
	requirePointer(t, "BTC marketPrice", roi.Assets[0].MarketPrice, amount(300))
	requirePointer(t, "BTC marketValue", roi.Assets[0].MarketValue, amount(300))
	requirePointer(t, "BTC unrealizedPnl", roi.Assets[0].UnrealizedPnL, amount(200))
	requirePointer(t, "ETH marketPrice", roi.Assets[1].MarketPrice, nil)
	requirePointer(t, "ETH unrealizedPnl", roi.Assets[1].UnrealizedPnL, nil)

	// Totales
	requireAmount(t, "investedCost", roi.Totals.InvestedCost, 600)
	requireAmount(t, "costBasis", roi.Totals.CostBasis, 500)
	requireAmount(t, "marketValue", roi.Totals.MarketValue, 700)
	requireAmount(t, "realizedPnl", roi.Totals.RealizedPnL, 49)
	requireAmount(t, "unrealizedPnl", roi.Totals.UnrealizedPnL, 200)
	requirePointer(t, "roiOnInvested", roi.Totals.RoiOnInvested, amount(249.0/600))
	requireAmount(t, "netDeposits", roi.Totals.NetDeposits, 1000)
	requireAmount(t, "cash", roi.Totals.Cash, 549)
	requireAmount(t, "equity", roi.Totals.Equity, 1249)
	requirePointer(t, "roiOnEquity", roi.Totals.RoiOnEquity, amount(0.249))

	// Oportunidad: su propio costo promedio, no el del portfolio
	if len(roi.Opportunities) != 1 {
		t.Fatalf("oportunidades = %d, se esperaba 1", len(roi.Opportunities))
	}
	opportunity := roi.Opportunities[0]
	if opportunity.TagTrigger != "TRIGGER-RSI" || opportunity.IDAsset != btc || opportunity.Transactions != 2 || opportunity.Warning != "" {
		t.Errorf("oportunidad = %+v", opportunity)
	}
	requireAmount(t, "quantityBought", opportunity.QuantityBought, 2)
	requireAmount(t, "quantitySold", opportunity.QuantitySold, 1)
	requireAmount(t, "openQuantity", opportunity.OpenQuantity, 1)
	requireAmount(t, "investedCost", opportunity.InvestedCost, 200)
	requireAmount(t, "proceeds", opportunity.Proceeds, 149)
	requireAmount(t, "averageCost", opportunity.AverageCost, 100)
	requireAmount(t, "realizedPnl", opportunity.RealizedPnL, 49)
	requirePointer(t, "unrealizedPnl", opportunity.UnrealizedPnL, amount(200))
	requirePointer(t, "roi", opportunity.Roi, amount(249.0/200))
	requirePointer(t, "plannedPriceChange", opportunity.PlannedPriceChange, amount(0.6))
}

func TestBuildPortfolioRoiService_RoiOnEquity(t *testing.T) {
	t.Parallel()

	btc := primitive.NewObjectID()
	portfolio := models.PortfolioModelType{Assets: []models.PortfolioAsset{{IDAsset: btc, Quantity: 2, AverageCost: 100, CostBasis: 200}}}
	prices := map[primitive.ObjectID]float64{btc: 150}

	tests := []struct {
		name         string
		transactions []models.PortfolioTransactionsModelType
		want         *float64
	}{
		{"sin depositos", []models.PortfolioTransactionsModelType{
			portfolioTx(btc, cmm.TransactionTypeBuy, 1, 2, 200, 0),
		}, nil},
		{"compras mayores a los depositos", []models.PortfolioTransactionsModelType{
			cashTx(cmm.TransactionTypeDeposit, 1, 100),
			portfolioTx(btc, cmm.TransactionTypeBuy, 2, 2, 200, 0),
		}, nil},
		{"retiro descuenta de los depositos", []models.PortfolioTransactionsModelType{
			cashTx(cmm.TransactionTypeDeposit, 1, 500),
			portfolioTx(btc, cmm.TransactionTypeBuy, 2, 2, 200, 0),
			cashTx(cmm.TransactionTypeWithdrawal, 3, 100),
		}, amount(100.0 / 400)}, // equity = cash 200 + mercado 300 = 500 sobre depositos netos 400
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			roi := services.BuildPortfolioRoiService(portfolio, tt.transactions, prices, nil, time.Now().UTC())

			requirePointer(t, "roiOnEquity", roi.Totals.RoiOnEquity, tt.want)
			requirePointer(t, "roiOnInvested", roi.Totals.RoiOnInvested, amount(0.5))
			if tt.want == nil && len(roi.Warnings) != 1 {
				t.Errorf("warnings = %v, se esperaba el motivo por el que no se calcula", roi.Warnings)
			}
		})
	}
}

func TestBuildPortfolioRoiService_OpportunityWithoutRoi(t *testing.T) {
	t.Parallel()

	btc := primitive.NewObjectID()
	idOpportunity := primitive.NewObjectID()

	tests := []struct {
		name         string
		transactions []models.PortfolioTransactionsModelType
		prices       map[primitive.ObjectID]float64
	}{
		{"ventas mayores a sus compras", []models.PortfolioTransactionsModelType{
			portfolioTx(btc, cmm.TransactionTypeBuy, 1, 2, 200, 0),
			linkedTx(portfolioTx(btc, cmm.TransactionTypeSell, 2, 1, 150, 0), idOpportunity),
		}, map[primitive.ObjectID]float64{btc: 300}},
		{"cantidad abierta sin precio", []models.PortfolioTransactionsModelType{
			linkedTx(portfolioTx(btc, cmm.TransactionTypeBuy, 1, 2, 200, 0), idOpportunity),
		}, map[primitive.ObjectID]float64{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			roi := services.BuildPortfolioRoiService(models.PortfolioModelType{}, tt.transactions, tt.prices, nil, time.Now().UTC())

			if len(roi.Opportunities) != 1 {
				t.Fatalf("oportunidades = %d, se esperaba 1", len(roi.Opportunities))
			}
			opportunity := roi.Opportunities[0]
			requirePointer(t, "roi", opportunity.Roi, nil)
			if opportunity.Warning == "" {
				t.Error("se esperaba el motivo por el que no se calcula el ROI")
			}
			// La oportunidad ya no es visible: sin sus datos pero con sus transacciones
			if opportunity.TagTrigger != "" || opportunity.PlannedPriceChange != nil {
				t.Errorf("oportunidad no visible = %+v, no debe tener sus datos", opportunity)
			}
		})
	}
}

func TestBuildPortfolioRoiService_EmptyPortfolio(t *testing.T) {
	t.Parallel()

	roi := services.BuildPortfolioRoiService(models.PortfolioModelType{}, nil, nil, nil, time.Now().UTC())

	if roi.Assets == nil || roi.Opportunities == nil || roi.Warnings == nil {
		t.Error("las listas deben ser [] y no null")
	}
	if roi.Partial || roi.Totals.RoiOnInvested != nil || roi.Totals.RoiOnEquity != nil {
		t.Errorf("roi = %+v, un portfolio vacio no tiene ROI", roi.Totals)
	}
}
