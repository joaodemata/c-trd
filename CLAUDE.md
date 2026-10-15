# CTRD

CTRD is a core to raise market opportunities for medium-large term.

## Stack

- Go 1.25.4
- MongoDB (driver: go.mongodb.org/mongo-driver v1.17.9)
- External APIs: finhubb (Market data), alphaVantage (Market data), polymarket (Probabilities data)

## Commands

```bash
go build ./...              # build
go build ./... && go vet ./... && gofmt -l .                         # run the server developer
go test ./...               # unit tests (no MongoDB or network needed)
go test -coverpkg=./... -coverprofile=cover.out ./tests/... && go tool cover -func=cover.out   # coverage
```

## Project layout

- `main.go`: entry point and wiring
- `common/`: general interfaces to manage errors, http responses, parsers and server configs
- `models/`: data models (APIs, Conditions, Opportunities), pointers of the model and constructors
- `cronjobs/`: `check_conditions.go` (Check the status of the conditions, if is active update the opportunities with STATUS ON GOING), `check_opportunities.go` (check the expired time of the opportunities)
- `services/`: can only be called inside cronjobs, data_validates, controllers or inside another service. They are CRUD with the databases and business logic.
- `format_validates/`: format data validation for each route or generic format validations
- `data_validates/`: data validation against the database to check integrity of the data.
- `controllers/`: Handlers of the routes they capture all the business logic using inside services
- `providers/`: functions to check where a condition is met using external API data
- `routers/`: route for each API and join all the middlewares and handlers
- `tests/`: all unit tests, one folder per package (`tests/providers/`, `tests/common/`, ...) plus `tests/testutil/` with shared helpers. Tests only use the exported API. Testing conventions are in `TESTING.md`

## Domain notes

- Request pipeline: router → format validate → data validate → controller. Data passes between them with SetInRequestContext and GetFromRequestContext, using the keys payload for the format validate and for controller could be any descriptive name.
- Tracking codes: they follow <layer><file>E<nnn>, for example SLISE001, PRHTTPE003 and DVHOOK002. Each should be unique.
- Adding a condition callback: write the function in providers/, then register it in FunctionRegistry in callbacks.go. The DB stores its name in callback_function
- Condition scheduling fields: is_processing is a lock, next_execution_at and interval_seconds drive the cron, and is_webhook_only conditions are skipped
- Logging: Use log for diagnostic errors, info or warnings.
- Timeouts: every DB call uses a context.WithTimeout (5s).
- An opportunity can have zero or multiple conditions, when all the conditions are met the opportunity changes to status ACTIVE. The life flow of the opportunities are OPPORTUNITY-ONGOING -> OPPORTUNITY-ACTIVE -> OPPORTUNITY-FINISHED OR OPPORTUNITY-ONGOING -> OPPORTUNITY-EXPIRED OR OPPORTUNITY-ONGOING -> OPPORTUNITY-CANCELED
- An opportunity can only be created with a trigger

## Conventions

- Errors: use the custom error type in `common/error_handler.go`; NEVER the error can be nil, must be used NewEmptyErrorHandler constructor
- IDs: Mongo IDs are `ObjectID`; use the string-to-ObjectID parser in `common/parsers.go`.
- Naming: Services must finish with service word, Controllers must finish with Controller word, data_validates must finish with DataValidate word and format_validates must finish with FormatValidate word.
- Language: code and identifiers in English; comments and commit messages in Spanish.

## Rules for Claude

- Run `go build ./... && go vet ./... && gofmt -l.` and `go test ./...` before saying a task is done.
- When adding or changing logic that does not need MongoDB (callbacks, parsers, validators, middlewares, cron decisions), add or update its unit tests under `tests/<package>/` following `TESTING.md`; never put `_test.go` files next to the code. Assert errors by tracking code with `testutil.RequireTrackingCode`.
- Don't add dependencies without asking.
- Don't edit generated files or `.env`/secrets.
- Make small, focused changes; don't refactor unrelated code.
- Ask before changing model schemas or DB indexes.
- Never compare `*ErrorHandler` with `nil` (neither `err != nil` nor `return x, nil`); always check `!err.IsEmtpy()` and return `cmm.NewEmptyErrorHandler()`. The method name is spelled `IsEmtpy` on purpose: don't rename it.
- Tracking codes can not be a parameter of any function, any function with error must have his own tracking code

## Gotchas

- `cmm.AppConfig` is assigned in `main.go` after the cronjobs and the HTTP server start, so it can be nil very early at startup.
- `godotenv.Load` runs only on Windows in `main.go`; on other systems the `JM_CTRD_MDB_*` env vars must be exported.
- The cron ticker fires every 1 minute and the HTTP server has 10s read/write timeouts.
- All stored times are UTC.
