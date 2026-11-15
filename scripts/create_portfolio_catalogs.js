// Prepara la base de datos para el portfolio: catalogo TYPE_TRANSACTION (compra, venta, dividendo, deposito y retiro).
// Los ids son fijos para que el catalogo sea el mismo en todos los ambientes (mismos valores que common/constants.go).
// Se puede ejecutar varias veces: los catalogos se crean si faltan y si existen no se tocan.
//
//   mongosh "mongodb://localhost:27017/ctrd" --file scripts/create_portfolio_catalogs.js

const CATALOGS = [
  {
    _id: ObjectId("6af9aad4c3e8131cda0c5183"),
    type_catalog: "TYPE_TRANSACTION",
    tag: "TRANSACTION-BUY",
    name: "Compra",
    description: "Compra de un asset: suma cantidad y costo a la posicion.",
  },
  {
    _id: ObjectId("6af9aad462469d39d6a86d80"),
    type_catalog: "TYPE_TRANSACTION",
    tag: "TRANSACTION-SELL",
    name: "Venta",
    description:
      "Venta de un asset: resta cantidad a la posicion y suma la ganancia o perdida realizada.",
  },
  {
    _id: ObjectId("6af9aad421702109f93bd590"),
    type_catalog: "TYPE_TRANSACTION",
    tag: "TRANSACTION-DIVIDEND",
    name: "Dividendo",
    description:
      "Dividendo o rendimiento de un asset: suma a la ganancia realizada sin mover la cantidad.",
  },
  {
    _id: ObjectId("6af9aad46a7bb747aecc1ee0"),
    type_catalog: "TYPE_TRANSACTION",
    tag: "TRANSACTION-DEPOSIT",
    name: "Deposito",
    description: "Ingreso de efectivo al portfolio, sin asset.",
  },
  {
    _id: ObjectId("6af9aad41ab057268aa0dc23"),
    type_catalog: "TYPE_TRANSACTION",
    tag: "TRANSACTION-WITHDRAWAL",
    name: "Retiro",
    description: "Salida de efectivo del portfolio, sin asset.",
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
