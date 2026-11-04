// Prepara la base de datos para los agentes: catalogos (privilegio AGENT, GENERIC_STATUS STATUS-PENDING-FOR-APPROVE y
// STATUS-NOT-APPROVED, AGENT_THESIS_STATUS y AGENT_RUN_STATUS), indices de agent_theses y agent_runs, y el usuario "claude" con privilegio AGENT.
// Los ids son fijos para que el catalogo y el usuario sean los mismos en todos los ambientes.
// Se puede ejecutar varias veces: los catalogos se crean si faltan y si el usuario existe no se toca.
//
// La contraseña se lee de variables de entorno para que no quede en el historial del shell ni en el archivo.
//
// PowerShell:
//   $env:CTRD_AGENT_PASSWORD = Read-Host
//   mongosh "mongodb://localhost:27017/ctrd" --file scripts/create_agent_user.js
//   Remove-Item Env:CTRD_AGENT_PASSWORD
//
// Bash:
//   read -rs CTRD_AGENT_PASSWORD && export CTRD_AGENT_PASSWORD
//   mongosh "mongodb://localhost:27017/ctrd" --file scripts/create_agent_user.js

const crypto = require("crypto");

// Mismos valores que common/constants.go y services/auth_services.go
const AGENT_USER_ID = ObjectId("6ae86ed7e014ebd19a6fb069");
const AGENT_USERNAME = "claude";
const PASSWORD_HASH_ITERATIONS = 600000;

const CATALOGS = [
  { _id: ObjectId("6ae86ce7b01de16516693815"), type_catalog: "USER_PRIVILEGES", tag: "AGENT", name: "Agente",
    description: "Usuario de los agentes: crea condiciones y triggers pendientes de aprobacion del super usuario." },

  { _id: ObjectId("6ae86ce7898915eb801dc41c"), type_catalog: "GENERIC_STATUS", tag: "STATUS-PENDING-FOR-APPROVE", name: "Pendiente de aprobacion",
    description: "Condicion o trigger creado por un agente: el cronjob y el hook lo ignoran hasta que el super usuario lo apruebe." },
  { _id: ObjectId("6ae86ce7fbb7528939b5cf50"), type_catalog: "GENERIC_STATUS", tag: "STATUS-NOT-APPROVED", name: "No aprobado",
    description: "Rechazado por el super usuario: se conserva, pero el cronjob y el hook lo ignoran." },

  { _id: ObjectId("6ae86ce77e2d74e73ec913ad"), type_catalog: "AGENT_THESIS_STATUS", tag: "THESIS-PENDING", name: "Pendiente",
    description: "En la cola, la toma la siguiente corrida de los viernes." },
  { _id: ObjectId("6ae86ce7c12c85f41a662e00"), type_catalog: "AGENT_THESIS_STATUS", tag: "THESIS-PROCESSING", name: "Procesando",
    description: "Tomada por una corrida en ejecucion." },
  { _id: ObjectId("6ae86ce7fae9f7626bd4c1d3"), type_catalog: "AGENT_THESIS_STATUS", tag: "THESIS-PROCESSED", name: "Procesada",
    description: "La corrida termino, el resultado esta en agent_runs." },

  { _id: ObjectId("6ae86ce7a7ffe76272b20232"), type_catalog: "AGENT_RUN_STATUS", tag: "RUN-RUNNING", name: "En ejecucion",
    description: "Los agentes estan procesando la tesis." },
  { _id: ObjectId("6ae86ce79edfa79e7137a41c"), type_catalog: "AGENT_RUN_STATUS", tag: "RUN-SUCCESS", name: "Exitosa",
    description: "Se crearon las condiciones y el trigger, pendientes de revision." },
  { _id: ObjectId("6ae86ce799e020873803db97"), type_catalog: "AGENT_RUN_STATUS", tag: "RUN-NEEDS-INPUT", name: "Requiere informacion",
    description: "Un agente necesita que se aclare la tesis, las preguntas estan en los pasos." },
  { _id: ObjectId("6ae86ce704d146a72e6afe00"), type_catalog: "AGENT_RUN_STATUS", tag: "RUN-FAILED", name: "Fallida",
    description: "Un agente no dio una salida valida tras los reintentos o fallo Claude." },
];

function fail(message) {
  print(`ERROR: ${message}`);
  quit(1);
}

// Mismo formato que HashPasswordService: pbkdf2_sha256$<iteraciones>$<salt>$<hash> en base64 sin padding
function hashPassword(password) {
  const salt = crypto.randomBytes(16);
  const key = crypto.pbkdf2Sync(password, salt, PASSWORD_HASH_ITERATIONS, 32, "sha256");
  const b64 = (buffer) => buffer.toString("base64").replace(/=+$/, "");
  return `pbkdf2_sha256$${PASSWORD_HASH_ITERATIONS}$${b64(salt)}$${b64(key)}`;
}

const now = new Date();
const audit = {
  core_db: db.getName(),
  core_version: "1.0.0",
  user_db: process.env.JM_CTRD_MDB_USER || "",
  schema_db: "1.0.0",
};

// --- Catalogos ---

for (const catalog of CATALOGS) {
  const result = db.catalogs.updateOne(
    { _id: catalog._id },
    { $setOnInsert: { ...catalog, configs: {}, last_updated_at: now, created_date: now, logical_delete: false, ...audit } },
    { upsert: true }
  );
  print(`${catalog.type_catalog} ${catalog.tag}: ${result.upsertedCount === 1 ? "creado" : "ya existia"}`);
}

// --- Indices ---

// La corrida toma las tesis pendientes mas antiguas (FIFO) y el listado filtra por status
db.agent_theses.createIndex({ tag_status: 1, created_date: 1 }, { name: "idx_agent_theses_status_created" });
db.agent_runs.createIndex({ id_thesis: 1, started_at: -1 }, { name: "idx_agent_runs_thesis_started" });
db.agent_runs.createIndex({ started_at: -1 }, { name: "idx_agent_runs_started" });
print("Indices de agent_theses y agent_runs listos.");

// --- Usuario claude ---

if (db.users.findOne({ _id: AGENT_USER_ID })) {
  print(`El usuario ${AGENT_USERNAME} ya existe, no se modifica.`);
  quit(0);
}
if (db.users.findOne({ username: AGENT_USERNAME, logical_delete: false })) {
  fail(`Ya existe otro usuario ${AGENT_USERNAME} con un id distinto de ${AGENT_USER_ID}. Borrelo o renombrelo antes.`);
}

const password = process.env.CTRD_AGENT_PASSWORD || "";
if (password.length < 8 || password.length > 128) {
  fail("CTRD_AGENT_PASSWORD debe tener de 8 a 128 caracteres.");
}

const privilege = CATALOGS[0];
db.users.insertOne({
  _id: AGENT_USER_ID,
  username: AGENT_USERNAME,
  email: null,
  name: "Claude",
  last_name: "Anthropic",
  id_type_privilege: privilege._id,
  tag_privilege: privilege.tag,
  name_privilege: privilege.name,
  description_privilege: privilege.description,
  is_active: true,
  password_hash: hashPassword(password),
  password_updated_at: now,
  failed_login_attempts: 0,
  locked_until: null,
  last_login_at: null,
  max_triggers_quantity: null,
  max_conditions_quantity: null,
  available_apis: [],
  config: {},
  id_created_by: null,
  last_updated_at: now,
  created_date: now,
  logical_delete: false,
  ...audit,
});

print(`Usuario ${AGENT_USERNAME} creado con id ${AGENT_USER_ID}.`);
