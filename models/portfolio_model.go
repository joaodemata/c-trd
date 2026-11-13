package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Valores dentro del portfolio
type PortfolioAsset struct {
	IDAsset              primitive.ObjectID `bson:"id_asset" json:"idAsset"`
	IDTypeAsset          primitive.ObjectID `bson:"id_type_asset" json:"idTypeAsset"`
	TypeTagAsset         string             `bson:"type_tag_asset" json:"typeTagAsset" description:"Tag del tipo de asset"`
	TypeNameAsset        string             `bson:"type_name_asset" json:"typeNameAsset" description:"Nombre del tipo de asset"`
	TypeDescriptionAsset string             `bson:"type_description_asset" json:"typeDescriptionAsset" description:"Descripcion del tipo de asset"`
	TagAsset             string             `bson:"tag_asset" json:"tagAsset" description:"Tag o identificador"`
	NameAsset            string             `bson:"name_asset" json:"nameAsset" description:"Nombre"`
	DescriptionAsset     string             `bson:"description_asset" json:"descriptionAsset" description:"Nombre"`
	Quantity             float64            `bson:"quantity" json:"quantity" description:"Cantidad del activo dentro del portfolio"`
	NetWorth             float64            `bson:"net_worth" json:"netWorth" description:"Patrimonio neto del activo en dolares"`
	MedianPrice          float64            `bson:"median_price" json:"medianPrice" description:"Precio medio del portfolio"`
}

type PortfolioModelType struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	IdUser      primitive.ObjectID `bson:"id_user,omitempty" json:"idUser"`
	Tag         string             `bson:"tag" json:"tag" description:"Tag del portfolio."`
	Name        string             `bson:"name" json:"name" description:"Nombre del portfolio."`
	Description string             `bson:"description" json:"description" description:"Descripcion del portfolio."`
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
