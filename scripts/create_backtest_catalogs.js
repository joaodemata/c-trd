// Prepara la base de datos para el backtesting: catalogo BACKTEST_RUN_STATUS (en cola, en ejecucion, exitosa y fallida)
// e indices de backtest_datasets, backtest_runs y condition_evaluations.
// Los ids son fijos para que el catalogo sea el mismo en todos los ambientes (mismos valores que common/constants.go).
// Se puede ejecutar varias veces: los catalogos se crean si faltan y si existen no se tocan; createIndex no hace nada
// si el indice ya existe con las mismas opciones.
//
//   mongosh "mongodb://localhost:27017/ctrd" --file scripts/create_backtest_catalogs.js

const CATALOGS = [
  {
    _id: ObjectId("6b049228935fd9c82d919845"),
    type_catalog: "BACKTEST_RUN_STATUS",
    tag: "BACKTEST-QUEUED",
    name: "En cola",
    description: "La corrida espera a que el backtester la tome.",
  },
  {
    _id: ObjectId("6b049228be88239ab17d242d"),
    type_catalog: "BACKTEST_RUN_STATUS",
    tag: "BACKTEST-RUNNING",
    name: "En ejecucion",
    description: "El backtester esta simulando la corrida.",
  },
  {
    _id: ObjectId("6b0492280b750163917f0a1e"),
    type_catalog: "BACKTEST_RUN_STATUS",
    tag: "BACKTEST-SUCCESS",
    name: "Exitosa",
    description: "La corrida termino y tiene metricas y operaciones.",
  },
  {
    _id: ObjectId("6b0492281b0fa548773d30c0"),
    type_catalog: "BACKTEST_RUN_STATUS",
    tag: "BACKTEST-FAILED",
    name: "Fallida",
    description: "La corrida no se pudo simular, el error queda en la corrida.",
  },
];

const now = new Date();
const audit = {
  core_db: db.getName(),
  core_version: "1.0.0",
  user_db: process.env.JM_CTRD_MDB_USER || "",
  schema_db: "1.0.0",
};

for (const catalog of CATALOGS) {
  const result = db.catalogs.updateOne(
    { _id: catalog._id },
    {
      $setOnInsert: {
        ...catalog,
        configs: {},
        last_updated_at: now,
        created_date: now,
        logical_delete: false,
        ...audit,
      },
    },
    { upsert: true },
  );
  print(
    `${catalog.type_catalog} ${catalog.tag}: ${result.upsertedCount === 1 ? "creado" : "ya existia"}`,
  );
}

// Un dataset vivo por asset y temporalidad: el upsert de la subida lo busca por estos campos
db.backtest_datasets.createIndex(
  { id_asset: 1, id_candlestick_timeframe: 1 },
  { name: "uniq_backtest_datasets_asset_timeframe", unique: true, partialFilterExpression: { logical_delete: false } },
);
db.backtest_datasets.createIndex({ last_updated_at: -1 }, { name: "idx_backtest_datasets_updated" });

// Cola del backtester (la corrida en cola mas antigua) y listados
db.backtest_runs.createIndex({ tag_status: 1, created_date: 1 }, { name: "idx_backtest_runs_status_created" });
db.backtest_runs.createIndex({ id_trigger: 1, created_date: -1 }, { name: "idx_backtest_runs_trigger_created" });
db.backtest_runs.createIndex({ id_dataset: 1, created_date: -1 }, { name: "idx_backtest_runs_dataset_created" });
db.backtest_runs.createIndex({ created_date: -1 }, { name: "idx_backtest_runs_created" });

// Historia de cada condicion para reconstruir su estado en el pasado
db.condition_evaluations.createIndex(
  { id_condition: 1, executed_at: 1 },
  { name: "idx_condition_evaluations_condition_executed" },
);

print("Catalogos e indices de backtesting listos");
