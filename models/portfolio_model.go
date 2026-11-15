package models

import (
	"os"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Valores dentro del portfolio. La posicion se reconstruye desde las transacciones del asset cada vez que se crea,
// actualiza o borra una de ellas
type PortfolioAsset struct {
	IDAsset              primitive.ObjectID `bson:"id_asset" json:"idAsset"`
	IDTypeAsset          primitive.ObjectID `bson:"id_type_asset" json:"idTypeAsset"`
	TypeTagAsset         string             `bson:"type_tag_asset" json:"typeTagAsset" description:"Tag del tipo de asset"`
	TypeNameAsset        string             `bson:"type_name_asset" json:"typeNameAsset" description:"Nombre del tipo de asset"`
	TypeDescriptionAsset string             `bson:"type_description_asset" json:"typeDescriptionAsset" description:"Descripcion del tipo de asset"`
	TagAsset             string             `bson:"tag_asset" json:"tagAsset" description:"Tag o identificador"`
	NameAsset            string             `bson:"name_asset" json:"nameAsset" description:"Nombre"`
	DescriptionAsset     string             `bson:"description_asset" json:"descriptionAsset" description:"Descripcion"`
	Quantity             float64            `bson:"quantity" json:"quantity" description:"Cantidad del activo dentro del portfolio"`
	// El valor de mercado no se guarda: se calcula al leer con la cantidad y el precio actual del asset
	AverageCost float64 `bson:"average_cost" json:"averageCost" description:"Costo promedio por unidad en dolares (incluye comisiones de compra)"`
	CostBasis   float64 `bson:"cost_basis" json:"costBasis" description:"Total invertido en dolares en la cantidad que sigue en el portfolio"`
	RealizedPnL float64 `bson:"realized_pnl" json:"realizedPnl" description:"Ganancia o perdida realizada en dolares por las ventas del activo"`
}

type PortfolioModelType struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	// Usuario dueño del portfolio
	RecordOwnerData `bson:",inline"`

	Tag         string `bson:"tag" json:"tag" description:"Tag del portfolio."`
	Name        string `bson:"name" json:"name" description:"Nombre del portfolio."`
	Description string `bson:"description" json:"description" description:"Descripcion del portfolio."`
	// Assets
	Assets []PortfolioAsset `bson:"assets" json:"assets" description:"Cartera del portafolio."`

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
var PortfolioModel *mongo.Collection

// CONSTRUCTORS

// NewPortfolioModel crea un portfolio vacio del usuario. assets se guarda como [] y se llena con las transacciones
func NewPortfolioModel(tag, name, description string, owner RecordOwnerData) (PortfolioModelType, *cmm.ErrorHandler) {
	now := time.Now().UTC()

	return PortfolioModelType{
		ID:              primitive.NewObjectID(),
		RecordOwnerData: owner,
		Tag:             tag,
		Name:            name,
		Description:     description,
		Assets:          []PortfolioAsset{},

		// Auditoría
		CoreDB:      os.Getenv("JM_CTRD_MDB_NAME"),
		CoreVersion: "1.0.0", // TODO: poner version
		UserDB:      os.Getenv("JM_CTRD_MDB_USER"),
		SchemaDB:    "1.0.0", // TODO: mejorar estructura

		CreatedDate:   now,
		LastUpdatedAt: now,
		LogicalDelete: false,
	}, cmm.NewEmptyErrorHandler()
}
