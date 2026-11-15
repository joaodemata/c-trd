package models

import (
	"fmt"
	"os"
	"slices"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type PortfolioTransactionsModelType struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	// Usuario que registro la transaccion
	RecordOwnerData `bson:",inline"`

	Comment string `bson:"comment" json:"comment" description:"Comentario de la transaccion."`
	// Portfolio: el cliente ya tiene sus datos porque la vista va del portfolio a sus transacciones
	IDPortfolio primitive.ObjectID `bson:"id_portfolio" json:"idPortfolio"`
	// Tipo de transaccion (catalogo TYPE_TRANSACTION): compra, venta, deposito, retiro, dividendo...
	IDTypeTransaction          primitive.ObjectID `bson:"id_type_transaction" json:"idTypeTransaction"`
	TagTypeTransaction         string             `bson:"tag_type_transaction" json:"tagTypeTransaction"`
	NameTypeTransaction        string             `bson:"name_type_transaction" json:"nameTypeTransaction"`
	DescriptionTypeTransaction string             `bson:"description_type_transaction" json:"descriptionTypeTransaction"`

	// Asset: null en depositos y retiros (movimientos de efectivo)
	IDAsset              *primitive.ObjectID `bson:"id_asset" json:"idAsset"`
	IDTypeAsset          *primitive.ObjectID `bson:"id_type_asset" json:"idTypeAsset"`
	TypeTagAsset         string              `bson:"type_tag_asset" json:"typeTagAsset" description:"Tag del tipo de asset"`
	TypeNameAsset        string              `bson:"type_name_asset" json:"typeNameAsset" description:"Nombre del tipo de asset"`
	TypeDescriptionAsset string              `bson:"type_description_asset" json:"typeDescriptionAsset" description:"Descripcion del tipo de asset"`
	TagAsset             string              `bson:"tag_asset" json:"tagAsset" description:"Tag o identificador"`
	NameAsset            string              `bson:"name_asset" json:"nameAsset" description:"Nombre"`
	DescriptionAsset     string              `bson:"description_asset" json:"descriptionAsset" description:"Descripcion"`
	// Montos: siempre positivos, la direccion la define el tipo de transaccion
	Quantity  float64 `bson:"quantity" json:"quantity" description:"Cantidad del activo de la transaccion (siempre positiva)"`
	UnitPrice float64 `bson:"unit_price" json:"unitPrice" description:"Precio por unidad en dolares al ejecutar la transaccion"`
	Value     float64 `bson:"value" json:"value" description:"Valor total de la transaccion en dolares (quantity * unit_price), sin comisiones"`
	Fee       float64 `bson:"fee" json:"fee" description:"Comision pagada en dolares"`
	// Fecha real de ejecucion; puede ser anterior a created_date si se registra despues
	ExecutedAt time.Time `bson:"executed_at" json:"executedAt" description:"Fecha y hora UTC en que se ejecuto la transaccion"`

	// Opportunity

	IDOpportunity *primitive.ObjectID `bson:"id_opportunity" json:"idOpportunity" description:"Transacción ligada a una oportunidad"`

	// Comunes
	LastUpdatedAt time.Time `bson:"last_updated_at" json:"lastUpdatedAt" description:"Ultima Actualizacion"`
	CreatedDate   time.Time `bson:"created_date" json:"createdDate" description:"Fecha de Creacion"`
	LogicalDelete bool      `bson:"logical_delete" json:"-" description:"Borrado Logico"`
	CoreDB        string    `bson:"core_db" json:"-" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string    `bson:"core_version" json:"-" description:"Version del Core"`
	UserDB        string    `bson:"user_db" json:"-" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string    `bson:"schema_db" json:"-" description:"Versionado de la Base de datos."`
}

// Model pointer
var PortfolioTransactionsModel *mongo.Collection

// Tags de TYPE_TRANSACTION que se saben registrar
var knownTransactionTags = []string{
	cmm.TransactionTypeBuy.Tag, cmm.TransactionTypeSell.Tag, cmm.TransactionTypeDividend.Tag,
	cmm.TransactionTypeDeposit.Tag, cmm.TransactionTypeWithdrawal.Tag,
}

// CONSTRUCTORS

// PortfolioTransactionOptionalData datos opcionales de la transaccion
type PortfolioTransactionOptionalData struct {
	// Al actualizar se conservan el id y la fecha de creacion de la transaccion reemplazada
	ID          primitive.ObjectID
	CreatedDate time.Time
	// Obligatorio en compra, venta y dividendo; no se envia en deposito ni retiro
	Asset *AssetsModelType
	// Solo compras y ventas se ligan a una oportunidad
	IDOpportunity *primitive.ObjectID
}

// NewPortfolioTransactionModel crea una transaccion del portfolio validando las reglas de su tipo:
//   - Compra y venta: asset, quantity y unitPrice mayores a 0; value se calcula (quantity * unitPrice) y no se envia
//   - Dividendo: asset y value mayor a 0, sin quantity ni unitPrice
//   - Deposito y retiro: value mayor a 0, sin asset, quantity ni unitPrice
//
// Solo compras y ventas se ligan a una oportunidad y executedAt no puede estar en el futuro
func NewPortfolioTransactionModel(
	idPortfolio primitive.ObjectID,
	typeTransaction CatalogsModelType,
	quantity, unitPrice, value, fee float64,
	executedAt time.Time,
	comment string,
	owner RecordOwnerData,
	opts *PortfolioTransactionOptionalData,
) (PortfolioTransactionsModelType, *cmm.ErrorHandler) {
	var transaction PortfolioTransactionsModelType
	now := time.Now().UTC()
	tag := typeTransaction.Tag

	if opts == nil {
		opts = &PortfolioTransactionOptionalData{}
	}

	movesPosition := tag == cmm.TransactionTypeBuy.Tag || tag == cmm.TransactionTypeSell.Tag
	needsAsset := movesPosition || tag == cmm.TransactionTypeDividend.Tag

	if !slices.Contains(knownTransactionTags, tag) {
		return transaction, cmm.NewErrorHandler(fmt.Errorf("tag_type_transaction %s", tag), "El tipo de transaccion enviado no esta soportado", cmm.LevelError, "MPTRE001")
	}
	if needsAsset && opts.Asset == nil {
		return transaction, cmm.NewErrorHandler(fmt.Errorf("asset vacio en %s", tag), "Las compras, ventas y dividendos necesitan idAsset", cmm.LevelError, "MPTRE002")
	}
	if !needsAsset && opts.Asset != nil {
		return transaction, cmm.NewErrorHandler(fmt.Errorf("asset en %s", tag), "Los depositos y retiros no llevan idAsset", cmm.LevelError, "MPTRE003")
	}
	if !movesPosition && opts.IDOpportunity != nil {
		return transaction, cmm.NewErrorHandler(fmt.Errorf("id_opportunity en %s", tag), "Solo las compras y ventas se pueden ligar a una oportunidad", cmm.LevelError, "MPTRE004")
	}
	if executedAt.After(now) {
		return transaction, cmm.NewErrorHandler(fmt.Errorf("executed_at %s", executedAt), "La fecha de ejecucion no puede estar en el futuro", cmm.LevelError, "MPTRE005")
	}

	if movesPosition {
		if quantity <= 0 || unitPrice <= 0 {
			return transaction, cmm.NewErrorHandler(fmt.Errorf("quantity %v unit_price %v", quantity, unitPrice), "Las compras y ventas necesitan quantity y unitPrice mayores a 0", cmm.LevelError, "MPTRE006")
		}
		if value != 0 {
			return transaction, cmm.NewErrorHandler(fmt.Errorf("value %v", value), "En compras y ventas value se calcula con quantity * unitPrice, no se envia", cmm.LevelError, "MPTRE007")
		}
		value = quantity * unitPrice
	} else {
		if quantity != 0 || unitPrice != 0 {
			return transaction, cmm.NewErrorHandler(fmt.Errorf("quantity %v unit_price %v", quantity, unitPrice), "Los dividendos, depositos y retiros no llevan quantity ni unitPrice", cmm.LevelError, "MPTRE008")
		}
		if value <= 0 {
			return transaction, cmm.NewErrorHandler(fmt.Errorf("value %v", value), "Los dividendos, depositos y retiros necesitan value mayor a 0", cmm.LevelError, "MPTRE009")
		}
	}

	transaction = PortfolioTransactionsModelType{
		ID:                         opts.ID,
		RecordOwnerData:            owner,
		Comment:                    comment,
		IDPortfolio:                idPortfolio,
		IDTypeTransaction:          typeTransaction.ID,
		TagTypeTransaction:         typeTransaction.Tag,
		NameTypeTransaction:        typeTransaction.Name,
		DescriptionTypeTransaction: typeTransaction.Description,
		Quantity:                   quantity,
		UnitPrice:                  unitPrice,
		Value:                      value,
		Fee:                        fee,
		ExecutedAt:                 executedAt.UTC(),
		IDOpportunity:              opts.IDOpportunity,

		// Auditoría
		CoreDB:      os.Getenv("JM_CTRD_MDB_NAME"),
		CoreVersion: "1.0.0", // TODO: poner version
		UserDB:      os.Getenv("JM_CTRD_MDB_USER"),
		SchemaDB:    "1.0.0", // TODO: mejorar estructura

		CreatedDate:   opts.CreatedDate,
		LastUpdatedAt: now,
		LogicalDelete: false,
	}

	if transaction.ID.IsZero() {
		transaction.ID = primitive.NewObjectID()
	}
	if transaction.CreatedDate.IsZero() {
		transaction.CreatedDate = now
	}

	if opts.Asset != nil {
		idAsset := opts.Asset.ID
		idTypeAsset := opts.Asset.IDTypeAsset
		transaction.IDAsset = &idAsset
		transaction.IDTypeAsset = &idTypeAsset
		transaction.TypeTagAsset = opts.Asset.TypeTag
		transaction.TypeNameAsset = opts.Asset.TypeName
		transaction.TypeDescriptionAsset = opts.Asset.TypeDescription
		transaction.TagAsset = opts.Asset.Tag
		transaction.NameAsset = opts.Asset.Name
		transaction.DescriptionAsset = opts.Asset.Description
	}

	return transaction, cmm.NewEmptyErrorHandler()
}
