package common

import (
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Valores hardcodeados de la aplicacion. Los status son una copia de la coleccion catalogs,
// si cambian en base de datos deben actualizarse aqui.

// CatalogStatus representa un status del catalogo (id, tag y nombre)
type CatalogStatus struct {
	ID   primitive.ObjectID
	Tag  string
	Name string
}

// GENERIC_STATUS (utilizados por las condiciones)
var (
	GenericStatusActive   = CatalogStatus{ID: mustParseObjectId("6a8423ce9687e916f29f233a"), Tag: "STATUS-ACTIVE", Name: "Activo"}
	GenericStatusInactive = CatalogStatus{ID: mustParseObjectId("6a84242c9687e916f29f233d"), Tag: "STATUS-INACTIVE", Name: "Inactivo"}
	// Condiciones y triggers que crea un agente: el cronjob no evalua la condicion y el hook no dispara el trigger hasta
	// que el super usuario los apruebe. Quien aprueba es el super usuario (solo existe uno) y cuando, last_updated_at
	GenericStatusPendingForApprove = CatalogStatus{ID: mustParseObjectId("6ae86ce7898915eb801dc41c"), Tag: "STATUS-PENDING-FOR-APPROVE", Name: "Pendiente de aprobacion"}
	// Rechazado por el super usuario: se conserva (sin borrado logico) pero el cronjob y el hook lo ignoran y ningun
	// trigger nuevo o aprobado puede usar la condicion
	GenericStatusNotApproved = CatalogStatus{ID: mustParseObjectId("6ae86ce7fbb7528939b5cf50"), Tag: "STATUS-NOT-APPROVED", Name: "No aprobado"}
)

// OPPORTUNITY_STATUS
var (
	OpportunityStatusOngoing  = CatalogStatus{ID: mustParseObjectId("6a84257d9687e916f29f234e"), Tag: "OPPORTUNITY-ONGOING", Name: "En Curso"}
	OpportunityStatusActive   = CatalogStatus{ID: mustParseObjectId("6ab64e0904584b67c7c0ac59"), Tag: "OPPORTUNITY-ACTIVE", Name: "Activa"}
	OpportunityStatusFinished = CatalogStatus{ID: mustParseObjectId("6ab2c54804584b67c7c0ac33"), Tag: "OPPORTUNITY-FINISHED", Name: "Finalizada"}
	OpportunityStatusExpired  = CatalogStatus{ID: mustParseObjectId("6a84257d9687e916f29f234d"), Tag: "OPPORTUNITY-EXPIRED", Name: "Expirada"}
	OpportunityStatusCanceled = CatalogStatus{ID: mustParseObjectId("6a84257d9687e916f29f234c"), Tag: "OPPORTUNITY-CANCELED", Name: "Cancelada"}
)

// USER_PRIVILEGES. Solo puede existir un super usuario (indice unico parcial sobre id_type_privilege en users)
var (
	UserPrivilegeSuperUser   = CatalogStatus{ID: mustParseObjectId("6ae38e7bb1bb56b09f96ce13"), Tag: "SUPER_USER", Name: "Super user"}
	UserPrivilegeGenericUser = CatalogStatus{ID: mustParseObjectId("6ae38ef3b1bb56b09f96ce14"), Tag: "GENERIC_USER", Name: "Usuario genérico"}
	// Usuario de los agentes (claude): crea condiciones y triggers que quedan pendientes de revision del super usuario
	UserPrivilegeAgent = CatalogStatus{ID: mustParseObjectId("6ae86ce7b01de16516693815"), Tag: "AGENT", Name: "Agente"}
)

// AGENT_THESIS_STATUS. Cola de tesis que procesan los agentes cada jueves en la noche (FIFO)
var (
	AgentThesisStatusPending    = CatalogStatus{ID: mustParseObjectId("6ae86ce77e2d74e73ec913ad"), Tag: "THESIS-PENDING", Name: "Pendiente"}
	AgentThesisStatusProcessing = CatalogStatus{ID: mustParseObjectId("6ae86ce7c12c85f41a662e00"), Tag: "THESIS-PROCESSING", Name: "Procesando"}
	AgentThesisStatusProcessed  = CatalogStatus{ID: mustParseObjectId("6ae86ce7fae9f7626bd4c1d3"), Tag: "THESIS-PROCESSED", Name: "Procesada"}
)

// AGENT_RUN_STATUS. Resultado de la corrida de los agentes sobre una tesis
var (
	AgentRunStatusRunning    = CatalogStatus{ID: mustParseObjectId("6ae86ce7a7ffe76272b20232"), Tag: "RUN-RUNNING", Name: "En ejecucion"}
	AgentRunStatusSuccess    = CatalogStatus{ID: mustParseObjectId("6ae86ce79edfa79e7137a41c"), Tag: "RUN-SUCCESS", Name: "Exitosa"}
	AgentRunStatusNeedsInput = CatalogStatus{ID: mustParseObjectId("6ae86ce799e020873803db97"), Tag: "RUN-NEEDS-INPUT", Name: "Requiere informacion"}
	AgentRunStatusFailed     = CatalogStatus{ID: mustParseObjectId("6ae86ce704d146a72e6afe00"), Tag: "RUN-FAILED", Name: "Fallida"}
)

// BACKTEST_RUN_STATUS. Corrida de backtest de un trigger: la API la deja en cola y el binario cmd/backtester la toma,
// la ejecuta y la cierra. Se crean con scripts/create_backtest_catalogs.js
var (
	BacktestRunStatusQueued  = CatalogStatus{ID: mustParseObjectId("6b049228935fd9c82d919845"), Tag: "BACKTEST-QUEUED", Name: "En cola"}
	BacktestRunStatusRunning = CatalogStatus{ID: mustParseObjectId("6b049228be88239ab17d242d"), Tag: "BACKTEST-RUNNING", Name: "En ejecucion"}
	BacktestRunStatusSuccess = CatalogStatus{ID: mustParseObjectId("6b0492280b750163917f0a1e"), Tag: "BACKTEST-SUCCESS", Name: "Exitosa"}
	BacktestRunStatusFailed  = CatalogStatus{ID: mustParseObjectId("6b0492281b0fa548773d30c0"), Tag: "BACKTEST-FAILED", Name: "Fallida"}
)

// TYPE_TRANSACTION. Transacciones del portfolio: compra y venta mueven la posicion del asset, el dividendo suma a la
// ganancia realizada y el deposito y el retiro son movimientos de efectivo sin asset. Se crean con scripts/create_portfolio_catalogs.js
var (
	TransactionTypeBuy        = CatalogStatus{ID: mustParseObjectId("6af9aad4c3e8131cda0c5183"), Tag: "TRANSACTION-BUY", Name: "Compra"}
	TransactionTypeSell       = CatalogStatus{ID: mustParseObjectId("6af9aad462469d39d6a86d80"), Tag: "TRANSACTION-SELL", Name: "Venta"}
	TransactionTypeDividend   = CatalogStatus{ID: mustParseObjectId("6af9aad421702109f93bd590"), Tag: "TRANSACTION-DIVIDEND", Name: "Dividendo"}
	TransactionTypeDeposit    = CatalogStatus{ID: mustParseObjectId("6af9aad46a7bb747aecc1ee0"), Tag: "TRANSACTION-DEPOSIT", Name: "Deposito"}
	TransactionTypeWithdrawal = CatalogStatus{ID: mustParseObjectId("6af9aad41ab057268aa0dc23"), Tag: "TRANSACTION-WITHDRAWAL", Name: "Retiro"}
)

// Agentes
const (
	// Tesis que toma cada corrida, las mas antiguas primero. El resto espera a la siguiente semana
	AgentThesesPerRun = 5
	// Una tesis en PROCESSING por mas de este tiempo se considera abandonada (el runner fallo) y se vuelve a tomar
	AgentThesisClaimTimeout = 12 * time.Hour
	// Placeholder del codigo del trigger que se reemplaza por el id_webhook al guardarlo
	TriggerCodeWebhookPlaceholder = "{{ID_WEBHOOK}}"
)

// Prefijos de los titulos de los plot que el pine del trigger expone para el backtest. El titulo completo es el
// prefijo + tag del trigger, asi varios triggers del mismo asset y temporalidad caben en un solo "Export chart data"
// de TradingView sin que se repitan las columnas
const (
	BacktestSignalColumnPrefix     = "CTRD_SIGNAL_"
	BacktestStopLossColumnPrefix   = "CTRD_SL_"
	BacktestTakeProfitColumnPrefix = "CTRD_TP_"
)

// Eventos del websocket (typeEvent). Solo avisan que algo cambio, sin datos: cada cliente vuelve a consultar su
// listado, asi no se envian registros a usuarios que no los pueden ver
const (
	// Se creo una oportunidad o cambio su status o el de sus condiciones
	EventOpportunitiesUpdated = "OPPORTUNITIES-UPDATED"
	// Ventana en la que el hub agrupa los eventos: una rafaga de cambios se envia como un solo evento por tipo
	EventsCoalesceWindow = 5 * time.Second
)

// TYPE_VISIBILITY. Quien puede ver un trigger, condicion u oportunidad: solo su usuario, todos o el usuario y los de
// allowed_users. Las consultas filtran por el tag
var (
	VisibilityPrivate   = CatalogStatus{ID: mustParseObjectId("6ae515f8b1bb56b09f96ce26"), Tag: "VISIBILITY_PRIVATE", Name: "Privado"}
	VisibilityPublic    = CatalogStatus{ID: mustParseObjectId("6ae5164cb1bb56b09f96ce28"), Tag: "VISIBILITY_PUBLIC", Name: "Público"}
	VisibilityWhitelist = CatalogStatus{ID: mustParseObjectId("6ae5168eb1bb56b09f96ce29"), Tag: "VISIBILITY_WHITELIST", Name: "Lista privada"}
)

// Valores de type_catalog en la coleccion catalogs
const (
	CatalogTypeGenericStatus     = "GENERIC_STATUS"
	CatalogTypeOpportunityStatus = "OPPORTUNITY_STATUS"
	CatalogTypeAction            = "TYPE_ACTION"
	CatalogTypeAsset             = "TYPE_ASSET"
	CatalogTypeCandleStick       = "TYPE_CANDLE_STICK"
	CatalogTypeCancelReason      = "CANCEL_REASON"
	CatalogTypeCondition         = "TYPE_CONDITION"
	CatalogTypeUserPrivileges    = "USER_PRIVILEGES"
	CatalogTypeVisibility        = "TYPE_VISIBILITY"
	CatalogTypeAgentThesisStatus = "AGENT_THESIS_STATUS"
	CatalogTypeAgentRunStatus    = "AGENT_RUN_STATUS"
	CatalogTypeTransaction       = "TYPE_TRANSACTION"
	CatalogTypeBacktestRunStatus = "BACKTEST_RUN_STATUS"
)

// Formatos de respuesta de las APIs (response_format de la coleccion apis). Cada callback declara
// el formato que sabe leer, asi solo se ofrecen los callbacks compatibles con la API
const (
	ResponseFormatAlphaVantagePutCallRatio = "ALPHA_VANTAGE_PUT_CALL_RATIO"
	ResponseFormatGoogleTrendsSeries       = "GOOGLE_TRENDS_SERIES"
	ResponseFormatGoogleTrendsFile         = "GOOGLE_TRENDS_FILE"
	ResponseFormatFredSeriesObservations   = "FRED_SERIES_OBSERVATIONS"
	ResponseFormatPolymarketFedDecisions   = "POLYMARKET_FED_DECISIONS"
	ResponseFormatCoinMarketCapFearGreed   = "COINMARKETCAP_FEAR_AND_GREED"
	ResponseFormatCoinMarketCapGlobal      = "COINMARKETCAP_GLOBAL_METRICS"
	ResponseFormatCoinMarketCapUsdtQuote   = "COINMARKETCAP_USDT_QUOTE"  // /v2/cryptocurrency/quotes/latest con symbol=USDT
	ResponseFormatBiquoteEconomicCalendar  = "BIQUOTE_ECONOMIC_CALENDAR" // /api/calendar, arreglo de eventos macroeconomicos
)

// Origen de la fecha del dato (as_of_source de condition_evaluations): PROVIDER si la respuesta del proveedor traia la
// fecha y SERVER si no la traia y se uso la hora del servidor al ejecutar la evaluacion (executed_at)
const (
	AsOfSourceProvider = "PROVIDER"
	AsOfSourceServer   = "SERVER"
)

// Tipos de dato de los parametros de una API (data_type). El valor se guarda como string y se valida con este tipo
const (
	ApiParameterTypeString  = "STRING"
	ApiParameterTypeInteger = "INTEGER"
	ApiParameterTypeFloat   = "FLOAT"
	ApiParameterTypeBoolean = "BOOLEAN"
	ApiParameterTypeDate    = "DATE" // formato YYYY-MM-DD
)

// Donde envia una API sus parametros (data_location). Una API solo envia datos por un lugar
const (
	ApiDataLocationQuery = "QUERY"
	ApiDataLocationBody  = "BODY"
)

// Origen de las opciones permitidas de un parametro (options.source)
const (
	ApiParameterOptionsStatic = "STATIC" // las opciones se guardan en options.values
	ApiParameterOptionsApi    = "API"    // las opciones se obtienen llamando a otra API de la coleccion apis
)

// Base de datos
const (
	// Timeout de cada llamada a la base de datos
	DBTimeout = 5 * time.Second
	// Cantidad de operaciones por cada BulkWrite
	BulkWriteBatchSize = 500
	// Maximo de BulkWrites simultaneos, basado en el maxPoolSize de MongoDB y no en la CPU
	BulkWriteMaxWorkers = 20
)

// Cronjob de condiciones
const (
	// Maximo de condiciones evaluadas simultaneamente (llamadas a APIs externas)
	ConditionEvaluationWorkers = 10
	// Intervalo por defecto si la condicion no tiene interval_seconds
	DefaultConditionIntervalSeconds = 60
	// Intervalo minimo al crear una condicion, el ticker del cronjob corre cada minuto
	ConditionMinIntervalSeconds = 60
	// Tiempo maximo para evaluar todas las condiciones de un ciclo (el ticker corre cada minuto)
	ConditionEvaluationTimeout = 50 * time.Second
	// Evaluaciones fallidas seguidas (incluyendo la actual) tras las que una condicion pasa a INACTIVE,
	// asi una API caida no deja la condicion ACTIVE para siempre
	ConditionMaxConsecutiveFailures = 3
)

// Peticiones HTTP a proveedores externos
const (
	// Prefijo de las variables de entorno con secretos de proveedores, ej: JM_CTRD_SECRET_ALPHAVANTAGE_APIKEY
	ProviderSecretEnvPrefix = "JM_CTRD_SECRET_"
	// Timeout por defecto si la peticion no tiene timeout_seconds
	ProviderDefaultTimeout = 10 * time.Second
	// Maximo de reintentos permitidos por peticion
	ProviderMaxRetries = 5
	// Maxima espera entre reintentos
	ProviderMaxRetryDelay = 30 * time.Second
	// Tamaño maximo de la respuesta de un proveedor (10 MB)
	ProviderMaxResponseBytes = 10 << 20
)

// Precio de mercado de un asset cuando el cronjob pasa su oportunidad a ACTIVE (si nace activa en el hook se usa el
// precio de TradingView). El asset guarda en configs.price_provider
// el tag del proveedor (coleccion providers, de donde salen el secret_ref y los allowed_hosts) y en
// configs.price_symbol el simbolo en ese proveedor
const (
	PriceProviderCoinMarketCap = "PROVIDER-COINMARKETCAP"
	PriceProviderFinnhub       = "PROVIDER-FINNHUB"
	// Tiempo maximo para consultar los precios de las oportunidades que se activan en un ciclo del cronjob
	ActivationPricesTimeout = 20 * time.Second
)

// Archivos CSV de Google Trends subidos por asset
const (
	// Variable de entorno con la carpeta donde se guardan los CSV
	GoogleTrendsDirEnv = "JM_CTRD_GOOGLE_TRENDS_DIR"
	// Carpeta por defecto si la variable no existe (relativa al directorio de ejecucion)
	GoogleTrendsDefaultDir = "data/google_trends"
)

// Tags del catalogo TYPE_ACTION: sentido de la operacion del trigger
const (
	ActionTagBuy  = "BUY"
	ActionTagSell = "SELL"
)

// Backtesting. Los CSV ("Export chart data" de TradingView) se guardan por su SHA-256 y nunca se borran al subir uno
// nuevo, asi una corrida vieja se puede repetir con los mismos datos
const (
	// Variable de entorno con la carpeta donde se guardan los CSV de los datasets
	BacktestDirEnv = "JM_CTRD_BACKTESTS_DIR"
	// Carpeta por defecto si la variable no existe (relativa al directorio de ejecucion)
	BacktestDefaultDir = "data/backtests"
	// Tamaño maximo del CSV de un dataset
	BacktestDatasetMaxBytes = 20 << 20
	// Tiempos de lectura y escritura de la conexion solo para la subida del CSV (el servidor usa 10s): 20 MB en
	// una conexion lenta tardan mas que eso
	BacktestUploadReadTimeout  = 3 * time.Minute
	BacktestUploadWriteTimeout = 3*time.Minute + 30*time.Second
	// Triggers que se pueden encolar en una sola peticion
	BacktestMaxTriggersPerRequest = 25
	// Una corrida con mas operaciones falla en vez de guardar un documento cerca del limite de 16 MB de Mongo
	BacktestMaxTrades = 20000
	// Una corrida en ejecucion por mas de este tiempo se considera abandonada (el binario se cayo) y se marca fallida
	BacktestRunClaimTimeout = 30 * time.Minute
	// Corridas que procesa cada ejecucion del binario, el resto espera a la siguiente
	BacktestRunsPerInvocation = 10
	// Tiempo maximo para traer la historia de un proveedor al reconstruir una condicion
	BacktestHistoryTimeout = 60 * time.Second
	// Una evaluacion guardada sirve hasta este multiplo del intervalo de la condicion, despues el estado es desconocido
	BacktestEvaluationMaxAgeIntervals = 2
)

// Origen de los datos de las condiciones en un backtest (settings.condition_source)
const (
	// Evaluaciones guardadas y, donde no hay, la historia actual del proveedor
	BacktestConditionSourceAuto = "AUTO"
	// Solo evaluaciones guardadas por el cronjob
	BacktestConditionSourceEvaluationsOnly = "EVALUATIONS_ONLY"
	// Solo la historia actual del proveedor (datos revisados)
	BacktestConditionSourceCurrentDataOnly = "CURRENT_DATA_ONLY"
)

// De donde salio el estado de una condicion en un instante del backtest
const (
	// Evaluacion guardada por el cronjob (lo que el sistema supo en ese momento)
	BacktestStateSourceEvaluation = "EVALUATION"
	// Historia actual del proveedor filtrada a la fecha: puede tener revisiones posteriores (FRED) o reescalado (Google Trends)
	BacktestStateSourceCurrentData = "CURRENT_DATA"
	// Sin datos: cuenta como no cumplida
	BacktestStateSourceUnknown = "UNKNOWN"
)

// Variantes que se simulan con las mismas señales
const (
	BacktestVariantTriggerOnly    = "TRIGGER_ONLY"
	BacktestVariantWithConditions = "WITH_CONDITIONS"
)

// Como termino cada oportunidad simulada
const (
	BacktestExitTakeProfit = "TP"
	BacktestExitStopLoss   = "SL"
	// Llego al final de las max_candlestick_qty velas con la operacion abierta: se cierra al cierre de la ultima vela
	BacktestExitExpired = "EXPIRED"
	// Toco el stop loss o el take profit mientras esperaba las condiciones (en curso): setup invalidado, sin operacion
	BacktestExitCanceled = "CANCELED"
	// Las condiciones no se cumplieron dentro de la ventana: expiro sin operacion
	BacktestExitNotActivated = "NOT_ACTIVATED"
	// El precio de entrada ya estaba del otro lado del stop loss o del take profit (gap): sin operacion
	BacktestExitInvalidEntry = "INVALID_ENTRY"
	// El CSV termino antes de cerrar la oportunidad: se reporta pero no entra en las metricas
	BacktestExitEndOfData = "END_OF_DATA"
)

// mustParseObjectId convierte un hexadecimal hardcodeado en ObjectID, detiene la app si es invalido
func mustParseObjectId(hex string) primitive.ObjectID {
	id, err := primitive.ObjectIDFromHex(hex)
	if err != nil {
		log.Fatalf("ObjectID hardcodeado invalido %s: %v", hex, err)
	}
	return id
}

// Autenticacion
const (
	// Duracion de una sesion desde el login
	SessionTTL = 12 * time.Hour
	// Iteraciones de PBKDF2-SHA256 (recomendacion OWASP)
	PasswordHashIterations = 600000
	// Intentos fallidos de login antes de bloquear el usuario y duracion del bloqueo
	LoginMaxFailedAttempts = 5
	LoginLockDuration      = 15 * time.Minute
)
