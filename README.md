# CTRD

[English](#english) · [Español](#español)

---

## English

CTRD is a backend core that raises **medium and long term market opportunities**. An opportunity is born from an external trigger (a TradingView alert), carries a set of conditions that are evaluated against market data APIs, and moves through its life cycle until it is activated, finished, expired or canceled.

### Stack

- Go 1.25.4 (standard library `net/http`, no web framework)
- MongoDB with the official driver `go.mongodb.org/mongo-driver` v1.17.9
- External data providers: Finnhub and Alpha Vantage (market data), Polymarket (probabilities)
- [Air](https://github.com/air-verse/air) for hot reload in development (`.air.toml`)

### How it works

```
TradingView alert ──► POST /hook/trading_view ──► Opportunity (ONGOING)
                                                        │
          cron (every 1 min) ──► evaluate conditions ───┤
                                 via external APIs      │
                                                        ▼
                         all conditions active ──► Opportunity (ACTIVE)
```

**Opportunity life cycle**

```
OPPORTUNITY-ONGOING ──► OPPORTUNITY-ACTIVE ──► OPPORTUNITY-FINISHED
        ├─────────────► OPPORTUNITY-EXPIRED
        └─────────────► OPPORTUNITY-CANCELED
```

- An opportunity can only be created by a **trigger**. If the trigger already has an ONGOING opportunity, it is reused.
- It can have zero or more **conditions**. When all of them are `STATUS-ACTIVE`, it moves to `OPPORTUNITY-ACTIVE`.
- Its expiration date is calculated from the trigger's maximum number of candles multiplied by the candle timeframe.

**Condition cronjob** (`cronjobs/check_conditions.go`)

Every minute the ticker runs `ProcessConditions`:

1. Loads the conditions that are due (`next_execution_at` in the past or empty, not `is_webhook_only`, not locked) and locks them with `is_processing`.
2. Evaluates them with a pool of workers: it calls the external API described in `http_request` and passes the response to the callback named in `callback_function`.
3. Saves the results in concurrent batches. Each batch runs in **one MongoDB transaction** that updates the condition copy inside ONGOING opportunities and the condition itself (status, `last_executed_at`, `next_execution_at`, lock). If the transaction fails, only the lock is released, so the batch is retried on the next cycle.
4. Promotes to `OPPORTUNITY-ACTIVE` every ONGOING, non-expired opportunity whose conditions are all active.

If a run takes longer than one tick, the next tick is skipped.

### Project layout

| Path | Purpose |
| --- | --- |
| `main.go` | Entry point: loads env, connects MongoDB, starts cronjobs, HTTP server and websocket hub |
| `common/` | Error handler, HTTP responses, parsers, router decorator, format validator, server config, websocket, hardcoded constants (`constants.go`) |
| `models/` | MongoDB models, collection pointers and constructors |
| `routers/` | Routes per API, wiring middlewares and handlers |
| `format_validates/` | Request format validation (`...FormatValidate`) |
| `data_validates/` | Data integrity validation against the database (`...DataValidate`) |
| `controllers/` | Route handlers (`...Controller`) |
| `services/` | Database CRUD and business logic (`...Service`). Only called from cronjobs, data validates, controllers or other services |
| `providers/` | External HTTP request executor and condition callbacks (`FunctionRegistry`) |
| `cronjobs/` | Background jobs: condition evaluation and opportunity expiration |

Request pipeline: **router → format validate → data validate → controller**. Data travels between steps with `SetInRequestContext` / `GetFromRequestContext` (the format validate stores it under the `payload` key).

### API

| Method | Path | Description |
| --- | --- | --- |
| `POST` | `/hook/trading_view` | Receives a TradingView alert and creates (or returns) the ONGOING opportunity of the trigger. Body: `{ "idTrigger": "<ObjectID>", "metadatos": { ... } }` |
| `GET` | `/list/opportunities` | Paginated list of opportunities. JSON body (optional): `{ "limit": 10, "page": 1, "search": "", "startDate": "...", "endDate": "..." }` |
| `POST` | `/operation/create/oportunity` | Placeholder, not implemented yet |
| `GET` | `/ws` | Websocket hub |

The server listens on `:8080` with 10 s read/write timeouts.

### Getting started

**Requirements**

- Go 1.25.4+
- MongoDB running as a **replica set** (a single node is enough). The condition cronjob uses transactions, which do not work on a standalone instance.

**Environment variables**

| Variable | Description | Default |
| --- | --- | --- |
| `JM_CTRD_MDB_HOST` | MongoDB connection URI, e.g. `mongodb://localhost:27017/?replicaSet=rs0` | `mongodb://localhost:27017` |
| `JM_CTRD_MDB_NAME` | Database name | `test` |
| `JM_CTRD_MDB_USER` | Database user, stored in audit fields | — |
| `JM_CTRD_SECRET_<secret_ref>` | Secret of an external provider (API key, token or `user:password` for BASIC), e.g. `JM_CTRD_SECRET_ALPHAVANTAGE_APIKEY` | — |

On Windows they are loaded from a `.env` file in the project root (required there). On other systems `.env` is not loaded by `main.go`, so the variables must be exported.

**Provider secrets**

Secrets are never stored in MongoDB. A condition's `http_request.auth` only says how to send the secret (`type`, `key`, `in`); `auth.value` is ignored. The value comes from the condition's provider document:

| Field (`providers`) | Description |
| --- | --- |
| `secret_ref` | Name of the secret. It is read from `JM_CTRD_SECRET_<secret_ref>` and may only contain `A-Z`, `0-9` and `_` |
| `allowed_hosts` | Hosts the provider's requests may be sent to, e.g. `["www.alphavantage.co"]` |

```json
{ "tag": "ALPHA_VANTAGE", "secret_ref": "ALPHAVANTAGE_APIKEY", "allowed_hosts": ["www.alphavantage.co"] }
```

If a request has `auth`, the provider must have `secret_ref` and `allowed_hosts`, the URL host must be in `allowed_hosts` and the URL must be `https`. Otherwise the request fails without being sent. Redirects to hosts outside `allowed_hosts` are not followed.

**Run**

```bash
go run .
```

With hot reload:

```bash
air
```

**Before committing**

```bash
go build ./... && go vet ./... && gofmt -l .
```

### Adding a condition callback

1. Write a function with the signature `func(responseBody []byte) (bool, *cmm.ErrorHandler)` in `providers/`.
2. Register it in `FunctionRegistry` in `providers/callbacks.go`.
3. Store the registered name in the condition's `callback_function` field in the database.

### Conventions

- **Errors:** always use `*cmm.ErrorHandler` from `common/error_handler.go`. It is never `nil`: return `cmm.NewEmptyErrorHandler()` and check with `!err.IsEmtpy()` (spelled that way on purpose).
- **Tracking codes:** `<layer><file>E<nnn>` (e.g. `SLISE001`, `PRHTTPE003`), unique, and never passed as a function parameter.
- **IDs:** MongoDB `ObjectID`, parsed with `common/parsers.go`.
- **Times:** always stored in UTC. Every DB call uses a 5 s `context.WithTimeout`.
- **Catalog statuses:** hardcoded in `common/constants.go` as a copy of the `catalogs` collection. Update them there if the catalog changes.
- **Language:** code and identifiers in English; comments and commit messages in Spanish.

### License

Proprietary. All rights reserved. See [license.md](license.md).

---

## Español

CTRD es un core backend que levanta **oportunidades de mercado a mediano y largo plazo**. Una oportunidad nace de un trigger externo (una alerta de TradingView), tiene un conjunto de condiciones que se evalúan contra APIs de datos de mercado y avanza por su ciclo de vida hasta activarse, finalizar, expirar o cancelarse.

### Stack

- Go 1.25.4 (librería estándar `net/http`, sin framework web)
- MongoDB con el driver oficial `go.mongodb.org/mongo-driver` v1.17.9
- Proveedores de datos externos: Finnhub y Alpha Vantage (datos de mercado), Polymarket (probabilidades)
- [Air](https://github.com/air-verse/air) para recarga en caliente en desarrollo (`.air.toml`)

### Cómo funciona

```
Alerta TradingView ──► POST /hook/trading_view ──► Oportunidad (ONGOING)
                                                         │
          cron (cada 1 min) ──► evalúa condiciones ──────┤
                                con APIs externas        │
                                                         ▼
                      todas las condiciones activas ──► Oportunidad (ACTIVE)
```

**Ciclo de vida de una oportunidad**

```
OPPORTUNITY-ONGOING ──► OPPORTUNITY-ACTIVE ──► OPPORTUNITY-FINISHED
        ├─────────────► OPPORTUNITY-EXPIRED
        └─────────────► OPPORTUNITY-CANCELED
```

- Una oportunidad solo puede crearse a partir de un **trigger**. Si el trigger ya tiene una oportunidad ONGOING, se reutiliza.
- Puede tener cero o más **condiciones**. Cuando todas están en `STATUS-ACTIVE`, pasa a `OPPORTUNITY-ACTIVE`.
- Su fecha de expiración se calcula con la cantidad máxima de velas del trigger multiplicada por la temporalidad de la vela.

**Cronjob de condiciones** (`cronjobs/check_conditions.go`)

Cada minuto el ticker ejecuta `ProcessConditions`:

1. Obtiene las condiciones vencidas (`next_execution_at` pasado o vacío, que no sean `is_webhook_only` ni estén bloqueadas) y las bloquea con `is_processing`.
2. Las evalúa con un pool de workers: llama a la API externa descrita en `http_request` y pasa la respuesta al callback indicado en `callback_function`.
3. Guarda los resultados en lotes concurrentes. Cada lote se ejecuta en **una transacción de MongoDB** que actualiza la copia de la condición dentro de las oportunidades ONGOING y la condición en sí (status, `last_executed_at`, `next_execution_at`, lock). Si la transacción falla, solo se libera el lock y el lote se reintenta en el siguiente ciclo.
4. Pasa a `OPPORTUNITY-ACTIVE` toda oportunidad ONGOING, no expirada, cuyas condiciones estén todas activas.

Si una ejecución tarda más de un tick, el siguiente tick se omite.

### Estructura del proyecto

| Ruta | Propósito |
| --- | --- |
| `main.go` | Punto de entrada: carga variables de entorno, conecta MongoDB, inicia cronjobs, servidor HTTP y hub de websocket |
| `common/` | Manejo de errores, respuestas HTTP, parsers, decorador de rutas, validador de formato, configuración del servidor, websocket y constantes hardcodeadas (`constants.go`) |
| `models/` | Modelos de MongoDB, punteros a colecciones y constructores |
| `routers/` | Rutas por API, unión de middlewares y handlers |
| `format_validates/` | Validación de formato de las peticiones (`...FormatValidate`) |
| `data_validates/` | Validación de integridad de datos contra la base de datos (`...DataValidate`) |
| `controllers/` | Handlers de las rutas (`...Controller`) |
| `services/` | CRUD de base de datos y lógica de negocio (`...Service`). Solo se llaman desde cronjobs, data validates, controllers u otros services |
| `providers/` | Ejecutor de peticiones HTTP externas y callbacks de condiciones (`FunctionRegistry`) |
| `cronjobs/` | Tareas en segundo plano: evaluación de condiciones y expiración de oportunidades |

Pipeline de una petición: **router → format validate → data validate → controller**. Los datos viajan entre pasos con `SetInRequestContext` / `GetFromRequestContext` (el format validate los guarda con la llave `payload`).

### API

| Método | Ruta | Descripción |
| --- | --- | --- |
| `POST` | `/hook/trading_view` | Recibe una alerta de TradingView y crea (o retorna) la oportunidad ONGOING del trigger. Body: `{ "idTrigger": "<ObjectID>", "metadatos": { ... } }` |
| `GET` | `/list/opportunities` | Listado paginado de oportunidades. Body JSON (opcional): `{ "limit": 10, "page": 1, "search": "", "startDate": "...", "endDate": "..." }` |
| `POST` | `/operation/create/oportunity` | Pendiente, aún no implementado |
| `GET` | `/ws` | Hub de websocket |

El servidor escucha en `:8080` con timeouts de lectura/escritura de 10 s.

### Primeros pasos

**Requisitos**

- Go 1.25.4+
- MongoDB como **replica set** (un solo nodo es suficiente). El cronjob de condiciones usa transacciones, que no funcionan en una instancia standalone.

**Variables de entorno**

| Variable | Descripción | Por defecto |
| --- | --- | --- |
| `JM_CTRD_MDB_HOST` | URI de conexión a MongoDB, ej. `mongodb://localhost:27017/?replicaSet=rs0` | `mongodb://localhost:27017` |
| `JM_CTRD_MDB_NAME` | Nombre de la base de datos | `test` |
| `JM_CTRD_MDB_USER` | Usuario de base de datos, se guarda en los campos de auditoría | — |
| `JM_CTRD_SECRET_<secret_ref>` | Secreto de un proveedor externo (API key, token o `usuario:password` para BASIC), ej. `JM_CTRD_SECRET_ALPHAVANTAGE_APIKEY` | — |

En Windows se cargan desde un archivo `.env` en la raíz del proyecto (obligatorio ahí). En otros sistemas `main.go` no carga el `.env`, así que las variables deben exportarse.

**Secretos de proveedores**

Los secretos nunca se guardan en MongoDB. El `http_request.auth` de una condición solo indica cómo enviar el secreto (`type`, `key`, `in`); `auth.value` se ignora. El valor sale del documento del proveedor de la condición:

| Campo (`providers`) | Descripción |
| --- | --- |
| `secret_ref` | Nombre del secreto. Se lee de `JM_CTRD_SECRET_<secret_ref>` y solo puede contener `A-Z`, `0-9` y `_` |
| `allowed_hosts` | Hosts a los que se pueden enviar las peticiones del proveedor, ej. `["www.alphavantage.co"]` |

```json
{ "tag": "ALPHA_VANTAGE", "secret_ref": "ALPHAVANTAGE_APIKEY", "allowed_hosts": ["www.alphavantage.co"] }
```

Si una petición tiene `auth`, el proveedor debe tener `secret_ref` y `allowed_hosts`, el host de la URL debe estar en `allowed_hosts` y la URL debe ser `https`. Si no, la petición falla sin enviarse. No se siguen redirecciones a hosts fuera de `allowed_hosts`.

**Ejecutar**

```bash
go run .
```

Con recarga en caliente:

```bash
air
```

**Antes de hacer commit**

```bash
go build ./... && go vet ./... && gofmt -l .
```

### Agregar un callback de condición

1. Escribe una función con la firma `func(responseBody []byte) (bool, *cmm.ErrorHandler)` en `providers/`.
2. Regístrala en `FunctionRegistry` en `providers/callbacks.go`.
3. Guarda el nombre registrado en el campo `callback_function` de la condición en la base de datos.

### Convenciones

- **Errores:** usa siempre `*cmm.ErrorHandler` de `common/error_handler.go`. Nunca es `nil`: retorna `cmm.NewEmptyErrorHandler()` y valida con `!err.IsEmtpy()` (escrito así a propósito).
- **Tracking codes:** `<capa><archivo>E<nnn>` (ej. `SLISE001`, `PRHTTPE003`), únicos y nunca pasados como parámetro de una función.
- **IDs:** `ObjectID` de MongoDB, parseados con `common/parsers.go`.
- **Fechas:** siempre se guardan en UTC. Toda llamada a la base de datos usa un `context.WithTimeout` de 5 s.
- **Status del catálogo:** hardcodeados en `common/constants.go` como copia de la colección `catalogs`. Si el catálogo cambia, actualízalos ahí.
- **Idioma:** código e identificadores en inglés; comentarios y mensajes de commit en español.

### Licencia

Propietaria. Todos los derechos reservados. Ver [license.md](license.md).
