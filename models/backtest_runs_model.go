package models

import (
	"os"
	"time"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// BacktestSettings costos y origen de los datos de las condiciones de la corrida
type BacktestSettings struct {
	FeeBps          float64 `bson:"fee_bps" json:"feeBps" description:"Comision por lado en puntos basicos (10 = 0.1%)"`
	SlippageBps     float64 `bson:"slippage_bps" json:"slippageBps" description:"Deslizamiento en contra en las ordenes a mercado (entrada, stop loss y expiracion)"`
	ConditionSource string  `bson:"condition_source" json:"conditionSource" description:"AUTO, EVALUATIONS_ONLY o CURRENT_DATA_ONLY"`
}

// BacktestRunCondition copia de una condicion del trigger al ejecutar la corrida y de donde salieron sus estados
type BacktestRunCondition struct {
	IDCondition        primitive.ObjectID `bson:"id_condition" json:"idCondition"`
	TagCondition       string             `bson:"tag_condition" json:"tagCondition"`
	NameCondition      string             `bson:"name_condition" json:"nameCondition"`
	CallbackFunction   string             `bson:"callback_function" json:"callbackFunction"`
	ParametersCallback map[string]any     `bson:"parameters_callback" json:"parametersCallback"`
	// Porcentaje (0 a 100) de las consultas de estado que se respondieron con cada origen
	Checks                 int     `bson:"checks" json:"checks" description:"Veces que se consulto el estado de la condicion"`
	CoveragePctEvaluations float64 `bson:"coverage_pct_evaluations" json:"coveragePctEvaluations"`
	CoveragePctCurrentData float64 `bson:"coverage_pct_current_data" json:"coveragePctCurrentData" description:"Estados con la historia actual del proveedor (datos revisados)"`
	CoveragePctUnknown     float64 `bson:"coverage_pct_unknown" json:"coveragePctUnknown" description:"Estados sin datos, cuentan como no cumplidos"`
	// Error al traer la historia del proveedor, null si no se pidio o se obtuvo
	CurrentDataError *string `bson:"current_data_error" json:"currentDataError"`
}

// BacktestConditionState estado de una condicion en un instante del backtest
type BacktestConditionState struct {
	IDCondition primitive.ObjectID `bson:"id_condition" json:"idCondition"`
	Accepted    bool               `bson:"accepted" json:"accepted" description:"false tambien si el estado es desconocido"`
	Source      string             `bson:"source" json:"source" description:"EVALUATION, CURRENT_DATA o UNKNOWN"`
	Value       *float64           `bson:"value" json:"value" description:"Valor medido, null si es desconocido"`
	AsOf        *time.Time         `bson:"as_of" json:"asOf" description:"Fecha del dato, null si es desconocido o el proveedor no la trae"`
}

// BacktestTrade una oportunidad simulada de una variante. Los campos de entrada y salida son null si no hubo operacion
type BacktestTrade struct {
	Variant    string    `bson:"variant" json:"variant" description:"TRIGGER_ONLY o WITH_CONDITIONS"`
	SignalAt   time.Time `bson:"signal_at" json:"signalAt" description:"Apertura de la vela con CTRD_SIGNAL=1 (UTC)"`
	StopLoss   float64   `bson:"stop_loss" json:"stopLoss"`
	TakeProfit float64   `bson:"take_profit" json:"takeProfit"`
	// Momento (cierre de vela) en que todas las condiciones se cumplieron, solo WITH_CONDITIONS
	ActivatedAt *time.Time `bson:"activated_at" json:"activatedAt"`
	EntryAt     *time.Time `bson:"entry_at" json:"entryAt" description:"Apertura de la vela de entrada"`
	EntryPrice  *float64   `bson:"entry_price" json:"entryPrice" description:"Apertura con deslizamiento en contra"`
	ExitAt      *time.Time `bson:"exit_at" json:"exitAt" description:"Apertura de la vela donde termino la oportunidad"`
	ExitPrice   *float64   `bson:"exit_price" json:"exitPrice"`
	ExitReason  string     `bson:"exit_reason" json:"exitReason" description:"TP, SL, EXPIRED, CANCELED, NOT_ACTIVATED, INVALID_ENTRY o END_OF_DATA"`
	BarsHeld    int        `bson:"bars_held" json:"barsHeld" description:"Velas con la operacion abierta, contando la de entrada y la de salida"`
	// Resultado neto de comisiones y deslizamiento
	ReturnPct *float64 `bson:"return_pct" json:"returnPct"`
	RMultiple *float64 `bson:"r_multiple" json:"rMultiple" description:"Resultado dividido por el riesgo (entrada - stop loss)"`
	// Peor y mejor movimiento del precio contra la entrada mientras la operacion estuvo abierta, en %
	MaePct *float64 `bson:"mae_pct" json:"maePct"`
	MfePct *float64 `bson:"mfe_pct" json:"mfePct"`
	// Estado de las condiciones al momento de la señal (TRIGGER_ONLY) o de la activacion o el fin (WITH_CONDITIONS)
	Conditions []BacktestConditionState `bson:"conditions" json:"conditions"`
}

// BacktestMetrics metricas de una variante. Las de resultado solo usan las operaciones cerradas (TP, SL y EXPIRED);
// son null si no hay ninguna
type BacktestMetrics struct {
	Signals        int `bson:"signals" json:"signals" description:"Velas con CTRD_SIGNAL=1"`
	SkippedSignals int `bson:"skipped_signals" json:"skippedSignals" description:"Señales con una oportunidad del trigger abierta, como el hook en vivo"`
	TradeCount     int `bson:"trade_count" json:"tradeCount" description:"Operaciones cerradas"`
	Wins           int `bson:"wins" json:"wins" description:"Operaciones con R mayor a 0"`
	Losses         int `bson:"losses" json:"losses"`
	TakeProfits    int `bson:"take_profits" json:"takeProfits"`
	StopLosses     int `bson:"stop_losses" json:"stopLosses"`
	Expired        int `bson:"expired" json:"expired"`
	Canceled       int `bson:"canceled" json:"canceled"`
	NotActivated   int `bson:"not_activated" json:"notActivated"`
	InvalidEntries int `bson:"invalid_entries" json:"invalidEntries"`
	EndOfData      int `bson:"end_of_data" json:"endOfData"`

	HitRate              *float64 `bson:"hit_rate" json:"hitRate" description:"Ganadoras / cerradas, de 0 a 1"`
	ExpectancyPct        *float64 `bson:"expectancy_pct" json:"expectancyPct" description:"Resultado neto promedio por operacion en %"`
	AverageR             *float64 `bson:"average_r" json:"averageR" description:"R promedio por operacion (esperanza en R)"`
	AverageWinR          *float64 `bson:"average_win_r" json:"averageWinR"`
	AverageLossR         *float64 `bson:"average_loss_r" json:"averageLossR"`
	TotalR               *float64 `bson:"total_r" json:"totalR"`
	MaxDrawdownR         *float64 `bson:"max_drawdown_r" json:"maxDrawdownR" description:"Mayor caida de la curva de R acumulado, ordenada por salida"`
	MaxAdverseMovePct    *float64 `bson:"max_adverse_move_pct" json:"maxAdverseMovePct"`
	MaxFavourableMovePct *float64 `bson:"max_favourable_move_pct" json:"maxFavourableMovePct"`
}

// BacktestResults metricas del trigger solo y del trigger con sus condiciones, sobre las mismas señales
type BacktestResults struct {
	TriggerOnly    BacktestMetrics `bson:"trigger_only" json:"triggerOnly"`
	WithConditions BacktestMetrics `bson:"with_conditions" json:"withConditions"`
}

// BacktestRunsModelType corrida de backtest de un trigger sobre un dataset. La API la crea en cola con la copia del
// trigger y del archivo; el binario cmd/backtester la ejecuta y guarda condiciones, metricas y operaciones
type BacktestRunsModelType struct {
	ID primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	// Dataset y copia del archivo usado (el dataset puede cambiar de archivo despues)
	IDDataset         primitive.ObjectID `bson:"id_dataset" json:"idDataset"`
	DatasetSha256     string             `bson:"dataset_sha256" json:"datasetSha256"`
	DatasetFirstBarAt time.Time          `bson:"dataset_first_bar_at" json:"datasetFirstBarAt"`
	DatasetLastBarAt  time.Time          `bson:"dataset_last_bar_at" json:"datasetLastBarAt"`
	// Copia del trigger
	IDTrigger      primitive.ObjectID `bson:"id_trigger" json:"idTrigger"`
	TagTrigger     string             `bson:"tag_trigger" json:"tagTrigger"`
	Trigger        string             `bson:"trigger" json:"trigger"`
	PineCodeSha256 *string            `bson:"pine_code_sha256" json:"pineCodeSha256" description:"SHA-256 del pine_code al encolar, null si no tiene"`
	IDAsset        primitive.ObjectID `bson:"id_asset" json:"idAsset"`
	TagAsset       string             `bson:"tag_asset" json:"tagAsset"`
	Asset          string             `bson:"asset" json:"asset"`
	IDAction       primitive.ObjectID `bson:"id_action" json:"idAction"`
	TagAction      string             `bson:"tag_action" json:"tagAction"`
	Action         string             `bson:"action" json:"action"`
	// Temporalidad y ventana de la oportunidad en velas
	TagCandlestickTimeframe string `bson:"tag_candlestick_timeframe" json:"tagCandlestickTimeframe"`
	MaxCandlestickQty       int    `bson:"max_candlestick_qty" json:"maxCandlestickQty"`

	Settings BacktestSettings `bson:"settings" json:"settings"`

	// Status (catalogo BACKTEST_RUN_STATUS)
	IDStatus  primitive.ObjectID `bson:"id_status" json:"idStatus"`
	TagStatus string             `bson:"tag_status" json:"tagStatus"`
	Status    string             `bson:"status" json:"status"`
	// Resultado: [] y null hasta que termine
	Conditions        []BacktestRunCondition `bson:"conditions" json:"conditions"`
	Results           *BacktestResults       `bson:"results" json:"results"`
	Trades            []BacktestTrade        `bson:"trades" json:"trades" description:"null en el listado, solo se envian en el detalle"`
	Error             *string                `bson:"error" json:"error" description:"Razon por la que fallo, null si no fallo"`
	ErrorTrackingCode *string                `bson:"error_tracking_code" json:"errorTrackingCode"`
	StartedAt         *time.Time             `bson:"started_at" json:"startedAt" description:"null mientras esta en cola"`
	FinishedAt        *time.Time             `bson:"finished_at" json:"finishedAt" description:"null mientras no termina"`
	// Usuario que encolo la corrida
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

// Model pointer
var BacktestRunsModel *mongo.Collection

// CONSTRUCTORS

// NewBacktestRunModel crea la corrida en cola de un trigger sobre el archivo actual del dataset. pineCodeSha256 es nil
// si el trigger no tiene pine_code
func NewBacktestRunModel(dataset BacktestDatasetsModelType, trigger TriggersModelType, pineCodeSha256 *string, settings BacktestSettings, owner RecordOwnerData) (BacktestRunsModelType, *cmm.ErrorHandler) {
	now := time.Now().UTC()

	return BacktestRunsModelType{
		ID:                      primitive.NewObjectID(),
		IDDataset:               dataset.ID,
		DatasetSha256:           dataset.FileSha256,
		DatasetFirstBarAt:       dataset.FirstBarAt,
		DatasetLastBarAt:        dataset.LastBarAt,
		IDTrigger:               trigger.ID,
		TagTrigger:              trigger.Tag,
		Trigger:                 trigger.Name,
		PineCodeSha256:          pineCodeSha256,
		IDAsset:                 trigger.IDAsset,
		TagAsset:                trigger.TagAsset,
		Asset:                   trigger.Asset,
		IDAction:                trigger.IDAction,
		TagAction:               trigger.TagAction,
		Action:                  trigger.Action,
		TagCandlestickTimeframe: trigger.TagCandlestickTimeframe,
		MaxCandlestickQty:       trigger.MaxCandlestickQty,
		Settings:                settings,
		IDStatus:                cmm.BacktestRunStatusQueued.ID,
		TagStatus:               cmm.BacktestRunStatusQueued.Tag,
		Status:                  cmm.BacktestRunStatusQueued.Name,
		Conditions:              []BacktestRunCondition{},
		Results:                 nil,
		Trades:                  []BacktestTrade{},
		Error:                   nil,
		ErrorTrackingCode:       nil,
		StartedAt:               nil,
		FinishedAt:              nil,
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
