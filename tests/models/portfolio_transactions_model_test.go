package models_test

import (
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/models"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// transactionType catalogo TYPE_TRANSACTION con los datos de la constante
func transactionType(status cmm.CatalogStatus) models.CatalogsModelType {
	return models.CatalogsModelType{ID: status.ID, TypeCatalog: cmm.CatalogTypeTransaction, Tag: status.Tag, Name: status.Name, Description: "Tipo de transaccion"}
}

var testAsset = models.AssetsModelType{
	ID:              primitive.NewObjectID(),
	IDTypeAsset:     primitive.NewObjectID(),
	TypeTag:         "TYPE-CRYPTO",
	TypeName:        "Cripto",
	TypeDescription: "Criptomonedas",
	Tag:             "ASSET-BTC",
	Name:            "Bitcoin",
	Description:     "Bitcoin",
}

// transactionInput datos de NewPortfolioTransactionModel, cada caso modifica uno
type transactionInput struct {
	typeTransaction cmm.CatalogStatus
	quantity        float64
	unitPrice       float64
	value           float64
	fee             float64
	executedAt      time.Time
	opts            *models.PortfolioTransactionOptionalData
}

func newTestTransaction(input transactionInput) (models.PortfolioTransactionsModelType, *cmm.ErrorHandler) {
	return models.NewPortfolioTransactionModel(
		primitive.NewObjectID(),
		transactionType(input.typeTransaction),
		input.quantity, input.unitPrice, input.value, input.fee,
		input.executedAt,
		"comentario",
		testOwner,
		input.opts,
	)
}

func TestNewPortfolioTransactionModel_Rules(t *testing.T) {
	t.Parallel()

	yesterday := time.Now().UTC().Add(-24 * time.Hour)
	idOpportunity := primitive.NewObjectID()
	withAsset := func() *models.PortfolioTransactionOptionalData {
		return &models.PortfolioTransactionOptionalData{Asset: &testAsset}
	}
	buy := func() transactionInput {
		return transactionInput{typeTransaction: cmm.TransactionTypeBuy, quantity: 2, unitPrice: 100, fee: 1, executedAt: yesterday, opts: withAsset()}
	}
	deposit := func() transactionInput {
		return transactionInput{typeTransaction: cmm.TransactionTypeDeposit, value: 500, executedAt: yesterday}
	}

	tests := []struct {
		name         string
		input        func() transactionInput
		wantTracking string
	}{
		{"compra valida", buy, ""},
		{"venta ligada a una oportunidad", func() transactionInput {
			in := buy()
			in.typeTransaction = cmm.TransactionTypeSell
			in.opts.IDOpportunity = &idOpportunity
			return in
		}, ""},
		{"dividendo valido", func() transactionInput {
			return transactionInput{typeTransaction: cmm.TransactionTypeDividend, value: 12.5, executedAt: yesterday, opts: withAsset()}
		}, ""},
		{"deposito valido", deposit, ""},
		{"retiro valido", func() transactionInput {
			in := deposit()
			in.typeTransaction = cmm.TransactionTypeWithdrawal
			return in
		}, ""},
		{"tipo desconocido", func() transactionInput {
			in := buy()
			in.typeTransaction = cmm.CatalogStatus{ID: primitive.NewObjectID(), Tag: "TRANSACTION-SWAP"}
			return in
		}, "MPTRE001"},
		{"compra sin asset", func() transactionInput {
			in := buy()
			in.opts = nil
			return in
		}, "MPTRE002"},
		{"dividendo sin asset", func() transactionInput {
			return transactionInput{typeTransaction: cmm.TransactionTypeDividend, value: 10, executedAt: yesterday}
		}, "MPTRE002"},
		{"deposito con asset", func() transactionInput {
			in := deposit()
			in.opts = withAsset()
			return in
		}, "MPTRE003"},
		{"dividendo ligado a una oportunidad", func() transactionInput {
			in := transactionInput{typeTransaction: cmm.TransactionTypeDividend, value: 10, executedAt: yesterday, opts: withAsset()}
			in.opts.IDOpportunity = &idOpportunity
			return in
		}, "MPTRE004"},
		{"ejecutada en el futuro", func() transactionInput {
			in := buy()
			in.executedAt = time.Now().UTC().Add(time.Hour)
			return in
		}, "MPTRE005"},
		{"compra sin cantidad", func() transactionInput {
			in := buy()
			in.quantity = 0
			return in
		}, "MPTRE006"},
		{"venta sin precio", func() transactionInput {
			in := buy()
			in.typeTransaction = cmm.TransactionTypeSell
			in.unitPrice = 0
			return in
		}, "MPTRE006"},
		{"compra con value enviado", func() transactionInput {
			in := buy()
			in.value = 200
			return in
		}, "MPTRE007"},
		{"deposito con cantidad", func() transactionInput {
			in := deposit()
			in.quantity = 1
			return in
		}, "MPTRE008"},
		{"dividendo con precio", func() transactionInput {
			return transactionInput{typeTransaction: cmm.TransactionTypeDividend, unitPrice: 3, value: 10, executedAt: yesterday, opts: withAsset()}
		}, "MPTRE008"},
		{"retiro sin value", func() transactionInput {
			in := deposit()
			in.typeTransaction = cmm.TransactionTypeWithdrawal
			in.value = 0
			return in
		}, "MPTRE009"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := newTestTransaction(tt.input())

			if tt.wantTracking == "" {
				testutil.RequireEmptyError(t, err)
				return
			}
			testutil.RequireTrackingCode(t, err, tt.wantTracking)
		})
	}
}

func TestNewPortfolioTransactionModel_Buy(t *testing.T) {
	t.Parallel()

	madrid := time.FixedZone("CET", 3600)
	executedAt := time.Date(2026, 10, 1, 10, 0, 0, 0, madrid)
	idOpportunity := primitive.NewObjectID()

	transaction, err := newTestTransaction(transactionInput{
		typeTransaction: cmm.TransactionTypeBuy, quantity: 0.5, unitPrice: 60000, fee: 15, executedAt: executedAt,
		opts: &models.PortfolioTransactionOptionalData{Asset: &testAsset, IDOpportunity: &idOpportunity},
	})

	testutil.RequireEmptyError(t, err)
	if transaction.ID.IsZero() || transaction.CreatedDate.IsZero() {
		t.Errorf("transaccion = %+v, debe tener ID y fecha de creacion", transaction)
	}
	if transaction.Value != 30000 {
		t.Errorf("value = %v, se esperaba quantity * unitPrice = 30000", transaction.Value)
	}
	if !transaction.ExecutedAt.Equal(executedAt) || transaction.ExecutedAt.Location() != time.UTC {
		t.Errorf("executedAt = %v, se esperaba %v en UTC", transaction.ExecutedAt, executedAt)
	}
	if transaction.IDAsset == nil || *transaction.IDAsset != testAsset.ID || transaction.TagAsset != "ASSET-BTC" {
		t.Errorf("asset = %v %q, se esperaba el asset enviado", transaction.IDAsset, transaction.TagAsset)
	}
	if transaction.IDTypeAsset == nil || *transaction.IDTypeAsset != testAsset.IDTypeAsset || transaction.TypeTagAsset != "TYPE-CRYPTO" {
		t.Errorf("tipo de asset = %v %q", transaction.IDTypeAsset, transaction.TypeTagAsset)
	}
	if transaction.TagTypeTransaction != cmm.TransactionTypeBuy.Tag || transaction.IDTypeTransaction != cmm.TransactionTypeBuy.ID {
		t.Errorf("tipo = %s", transaction.TagTypeTransaction)
	}
	if transaction.IDOpportunity == nil || *transaction.IDOpportunity != idOpportunity {
		t.Errorf("oportunidad = %v, se esperaba %s", transaction.IDOpportunity, idOpportunity.Hex())
	}
	if transaction.IDUser != testOwner.IDUser {
		t.Errorf("dueño = %s", transaction.IDUser.Hex())
	}
}

func TestNewPortfolioTransactionModel_DepositHasNoAsset(t *testing.T) {
	t.Parallel()

	transaction, err := newTestTransaction(transactionInput{typeTransaction: cmm.TransactionTypeDeposit, value: 1000, executedAt: time.Now().UTC()})

	testutil.RequireEmptyError(t, err)
	if transaction.IDAsset != nil || transaction.IDTypeAsset != nil || transaction.IDOpportunity != nil {
		t.Errorf("asset = %v, tipo = %v, oportunidad = %v; un deposito no los lleva", transaction.IDAsset, transaction.IDTypeAsset, transaction.IDOpportunity)
	}

	// Se guardan como null para poder filtrar los movimientos de efectivo
	keys := jsonKeys(t, transaction)
	if string(keys["idAsset"]) != "null" || string(keys["idOpportunity"]) != "null" {
		t.Errorf("idAsset = %s, idOpportunity = %s, se esperaba null", keys["idAsset"], keys["idOpportunity"])
	}
}

func TestNewPortfolioTransactionModel_UpdateKeepsIdentity(t *testing.T) {
	t.Parallel()

	id := primitive.NewObjectID()
	createdDate := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

	transaction, err := newTestTransaction(transactionInput{
		typeTransaction: cmm.TransactionTypeDeposit, value: 250, executedAt: createdDate,
		opts: &models.PortfolioTransactionOptionalData{ID: id, CreatedDate: createdDate},
	})

	testutil.RequireEmptyError(t, err)
	if transaction.ID != id || !transaction.CreatedDate.Equal(createdDate) {
		t.Errorf("id = %s, creada = %v; al actualizar se conservan", transaction.ID.Hex(), transaction.CreatedDate)
	}
	if !transaction.LastUpdatedAt.After(createdDate) {
		t.Errorf("ultima actualizacion = %v, debe ser la del reemplazo", transaction.LastUpdatedAt)
	}
}
