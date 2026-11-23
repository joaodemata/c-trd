package models

import (
	"os"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// BacktestGap hueco entre dos velas seguidas del CSV (ej: fin de semana de una accion). Solo se aceptan si el dataset
// se subio con allow_session_gaps
type BacktestGap struct {
	From        time.Time `bson:"from" json:"from" description:"Apertura de la ultima vela antes del hueco (UTC)"`
	To          time.Time `bson:"to" json:"to" description:"Apertura de la primera vela despues del hueco (UTC)"`
	MissingBars int       `bson:"missing_bars" json:"missingBars" description:"Velas que faltan segun la temporalidad"`
}

// BacktestDatasetsModelType CSV "Export chart data" de TradingView de un asset y una temporalidad. Trae las velas y
// las columnas CTRD_SIGNAL_<tag>, CTRD_SL_<tag> y CTRD_TP_<tag> de cada trigger que estaba en el grafico. Hay uno por
// asset y temporalidad: subir otro reemplaza el archivo del dataset pero el anterior queda guardado por su SHA-256
type BacktestDatasetsModelType struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	// Asset
	IDAsset  primitive.ObjectID `bson:"id_asset" json:"idAsset"`
	TagAsset string             `bson:"tag_asset" json:"tagAsset"`
	Asset    string             `bson:"asset" json:"asset"`
	// Temporalidad de las velas (catalogo TYPE_CANDLE_STICK)
	IDCandlestickTimeframe  primitive.ObjectID `bson:"id_candlestick_timeframe" json:"idCandlestickTimeframe"`
	TagCandlestickTimeframe string             `bson:"tag_candlestick_timeframe" json:"tagCandlestickTimeframe"`
	CandlestickTimeframe    string             `bson:"candlestick_timeframe" json:"candlestickTimeframe"`
	// Archivo
	FileSha256   string `bson:"file_sha256" json:"fileSha256" description:"SHA-256 del CSV, tambien es su nombre en la carpeta de backtests"`
	OriginalName string `bson:"original_name" json:"originalName" description:"Nombre con el que se subio, solo informativo"`
	SizeBytes    int64  `bson:"size_bytes" json:"sizeBytes"`
	// Velas
	RowCount         int           `bson:"row_count" json:"rowCount"`
	FirstBarAt       time.Time     `bson:"first_bar_at" json:"firstBarAt" description:"Apertura de la primera vela (UTC)"`
	LastBarAt        time.Time     `bson:"last_bar_at" json:"lastBarAt" description:"Apertura de la ultima vela (UTC)"`
	AllowSessionGaps bool          `bson:"allow_session_gaps" json:"allowSessionGaps" description:"Se aceptaron huecos entre velas (mercados con sesiones)"`
	Gaps             []BacktestGap `bson:"gaps" json:"gaps" description:"Huecos aceptados, [] si no hay"`
	SignalTags       []string      `bson:"signal_tags" json:"signalTags" description:"Tags de los triggers con columnas en el CSV"`
	// Usuario que subio el archivo
	RecordOwnerData `bson:",inline"`

	// Common
	LastUpdatedAt time.Time `bson:"last_updated_at" json:"lastUpdatedAt" description:"Ultima Actualizacion"`
	CreatedDate   time.Time `bson:"created_date" json:"createdDate" description:"Fecha de Creacion"`
	LogicalDelete bool      `bson:"logical_delete" json:"-" description:"Borrado Logico"`
	CoreDB        string    `bson:"core_db" json:"-" description:"Nombre del Core de Base de datos que hizo el registro"`
	CoreVersion   string    `bson:"core_version" json:"-" description:"Version del Core"`
	UserDB        string    `bson:"user_db" json:"-" description:"Usuario de Base de datos que hizo el registro"`
	SchemaDB      string    `bson:"schema_db" json:"-" description:"Versionado de la Base de datos"`
}

// BacktestDatasetFileData datos del CSV ya validado que se copian al dataset
type BacktestDatasetFileData struct {
	FileSha256       string
	OriginalName     string
	SizeBytes        int64
	RowCount         int
	FirstBarAt       time.Time
	LastBarAt        time.Time
	AllowSessionGaps bool
	Gaps             []BacktestGap
	SignalTags       []string
}

// Model pointer
var BacktestDatasetsModel *mongo.Collection

// CONSTRUCTORS

// NewBacktestDatasetModel crea el dataset de un asset y una temporalidad. gaps y signalTags nil se guardan como []
func NewBacktestDatasetModel(asset AssetsModelType, timeframe CatalogsModelType, file BacktestDatasetFileData, owner RecordOwnerData) (BacktestDatasetsModelType, *cmm.ErrorHandler) {
	now := time.Now().UTC()

	gaps := file.Gaps
	if gaps == nil {
		gaps = []BacktestGap{}
	}
	signalTags := file.SignalTags
	if signalTags == nil {
		signalTags = []string{}
	}

	return BacktestDatasetsModelType{
		ID:                      primitive.NewObjectID(),
		IDAsset:                 asset.ID,
		TagAsset:                asset.Tag,
		Asset:                   asset.Name,
		IDCandlestickTimeframe:  timeframe.ID,
		TagCandlestickTimeframe: timeframe.Tag,
		CandlestickTimeframe:    timeframe.Name,
		FileSha256:              file.FileSha256,
		OriginalName:            file.OriginalName,
		SizeBytes:               file.SizeBytes,
		RowCount:                file.RowCount,
		FirstBarAt:              file.FirstBarAt.UTC(),
		LastBarAt:               file.LastBarAt.UTC(),
		AllowSessionGaps:        file.AllowSessionGaps,
		Gaps:                    gaps,
		SignalTags:              signalTags,
		RecordOwnerData:         owner,

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
