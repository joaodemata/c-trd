// Crea los indices de users y sessions y el super usuario (solo puede existir uno).
// Se puede ejecutar varias veces: los indices ya creados se mantienen y si el super usuario existe no se crea otro.
//
// La contraseña se lee de variables de entorno para que no quede en el historial del shell ni en el archivo.
//
// PowerShell:
//   $env:CTRD_SUPER_USER_USERNAME = "jmata"
//   $env:CTRD_SUPER_USER_PASSWORD = Read-Host
//   mongosh "mongodb://localhost:27017/ctrd" --file scripts/create_super_user.js
//   Remove-Item Env:CTRD_SUPER_USER_PASSWORD
//
// Bash:
//   read -rs CTRD_SUPER_USER_PASSWORD && export CTRD_SUPER_USER_PASSWORD
//   CTRD_SUPER_USER_USERNAME=jmata mongosh "mongodb://localhost:27017/ctrd" --file scripts/create_super_user.js
//
// Opcionales: CTRD_SUPER_USER_EMAIL, CTRD_SUPER_USER_NAME, CTRD_SUPER_USER_LASTNAME

const crypto = require("crypto");

// Mismos valores que common/constants.go y services/auth_services.go
const SUPER_USER_PRIVILEGE_ID = ObjectId("6ae38e7bb1bb56b09f96ce13");
const PASSWORD_HASH_ITERATIONS = 600000;

function fail(message) {
  print(`ERROR: ${message}`);
  quit(1);
}

function optionalEnv(name) {
  const value = (process.env[name] || "").trim();
  return value === "" ? null : value;
}

// Mismo formato que HashPasswordService: pbkdf2_sha256$<iteraciones>$<salt>$<hash> en base64 sin padding
function hashPassword(password) {
  const salt = crypto.randomBytes(16);
  const key = crypto.pbkdf2Sync(password, salt, PASSWORD_HASH_ITERATIONS, 32, "sha256");
  const b64 = (buffer) => buffer.toString("base64").replace(/=+$/, "");
  return `pbkdf2_sha256$${PASSWORD_HASH_ITERATIONS}$${b64(salt)}$${b64(key)}`;
}

// --- Indices ---

// username y email unicos entre los usuarios no borrados (email solo cuando existe, puede ser null)
db.users.createIndex(
  { username: 1 },
  { name: "uniq_users_username", unique: true, partialFilterExpression: { logical_delete: false } }
);
db.users.createIndex(
  { email: 1 },
  { name: "uniq_users_email", unique: true, partialFilterExpression: { email: { $type: "string" }, logical_delete: false } }
);
// Solo un super usuario no borrado
db.users.createIndex(
  { id_type_privilege: 1 },
  {
    name: "uniq_users_super_user",
    unique: true,
    partialFilterExpression: { id_type_privilege: SUPER_USER_PRIVILEGE_ID, logical_delete: false },
  }
);

// Las sesiones se buscan por el hash del token, el TTL borra las vencidas y el logout general borra por usuario
db.sessions.createIndex({ token_hash: 1 }, { name: "uniq_sessions_token_hash", unique: true });
db.sessions.createIndex({ expires_at: 1 }, { name: "ttl_sessions_expires_at", expireAfterSeconds: 0 });
db.sessions.createIndex({ id_user: 1 }, { name: "idx_sessions_id_user" });

print("Indices de users y sessions listos.");

// --- Super usuario ---

const existing = db.users.findOne({ id_type_privilege: SUPER_USER_PRIVILEGE_ID, logical_delete: false }, { username: 1 });
if (existing) {
  print(`El super usuario ya existe (${existing.username}), no se crea otro.`);
  quit(0);
}

const username = (process.env.CTRD_SUPER_USER_USERNAME || "").trim();
const password = process.env.CTRD_SUPER_USER_PASSWORD || "";
const email = optionalEnv("CTRD_SUPER_USER_EMAIL");

// Mismas reglas que CreateUserFormat
if (!/^[a-z0-9]{3,32}$/.test(username)) {
  fail("CTRD_SUPER_USER_USERNAME debe tener de 3 a 32 letras minusculas o numeros.");
}
if (password.length < 8 || password.length > 128) {
  fail("CTRD_SUPER_USER_PASSWORD debe tener de 8 a 128 caracteres.");
}
if (email !== null && (email !== email.toLowerCase() || !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email))) {
  fail("CTRD_SUPER_USER_EMAIL debe ser un email valido en minusculas.");
}

// El privilegio se copia del catalogo, igual que NewUserModel
const privilege = db.catalogs.findOne({ _id: SUPER_USER_PRIVILEGE_ID, type_catalog: "USER_PRIVILEGES", logical_delete: false });
if (!privilege) {
  fail("No existe el catalogo USER_PRIVILEGES SUPER_USER (6ae38e7bb1bb56b09f96ce13).");
}

const now = new Date();
const result = db.users.insertOne({
  username: username,
  email: email,
  name: optionalEnv("CTRD_SUPER_USER_NAME"),
  last_name: optionalEnv("CTRD_SUPER_USER_LASTNAME"),
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
  core_db: db.getName(),
  core_version: "1.0.0",
  user_db: process.env.JM_CTRD_MDB_USER || "",
  schema_db: "1.0.0",
});

print(`Super usuario ${username} creado con id ${result.insertedId}.`);
