# Unit tests

Unit tests run without MongoDB or internet access: `go test ./...` must pass on any freshly cloned
machine.

```bash
go test ./...                                          # all tests
go test ./tests/providers/ -run TestExecuteRequest -v  # one test, verbose
go test -coverpkg=./... -coverprofile=cover.out ./tests/... && go tool cover -func=cover.out   # coverage
go test -race ./...                                    # race detector (needs CGO/gcc)
```

Since the tests live in a different folder than the code, plain `-cover` only measures the test
packages themselves. Use `-coverpkg=./...` to measure the project code.

## Layout

All tests live under `tests/`, mirroring the project packages:

```
tests/
  testutil/                 shared helpers (testutil.go is the only non-test file)
  common/                   error_handler_test.go, format_validator_test.go, ...
  cronjobs/                 check_conditions_test.go
  models/                   opportunities_model_test.go
  providers/                alpha_vantage_test.go, provider_http_handler_test.go, ...
    testdata/               real provider responses (JSON)
  services/                 operation_service_test.go
    testdata/               Google Trends CSV exports
  tracking_codes_test.go    fails if a tracking code is repeated anywhere in the project
```

- One test file per source file, named after it: `providers/alpha_vantage.go` →
  `tests/providers/alpha_vantage_test.go`.
- Test packages are named `<package>_test` (`providers_test`, `common_test`, ...).
- **Tests only use the exported API.** A test in another folder is another Go package, so it cannot
  call unexported functions. Test private logic through the exported function that uses it (for
  example `validateHost` is tested through `providers.ExecuteRequest`). Only export something when
  there is no exported entry point that reaches it without MongoDB, as was done with
  `cronjobs.EvaluateCondition`, `cronjobs.NewConditionStatus` and `cronjobs.ConditionInterval`.
- Input files go in `testdata/` next to the test and are read with
  `testutil.ReadFixture(t, "file.json")`. Go ignores `testdata` folders when building.

## Writing a test

1. **Table of cases + `t.Run`**: a slice of structs with `name`, the input and the expected result.
   Adding a case is adding one line, and the name shows up in the report when it fails.
2. **`t.Parallel()`** in the test and in every subtest, unless it touches global state
   (`t.Setenv`, `providers.FunctionRegistry`, `http.DefaultTransport`). In that case keep it
   sequential and say why in a comment.
3. **Assert errors by tracking code**, not by message. The code points to the exact place that
   failed:

   ```go
   _, err := providers.ExecuteRequest(ctx, config, credentials)
   testutil.RequireTrackingCode(t, err, "PRHTTPE013")
   ```

   For the success path use `testutil.RequireEmptyError(t, err)`, which also fails if a function
   returns `nil` instead of `cmm.NewEmptyErrorHandler()`.
4. **Failure messages that explain**: `t.Errorf("status = %d, want %d", got, want)`.
5. **No real network**: external APIs are faked with `httptest.NewServer` (or `httptest.NewTLSServer`
   for authenticated requests). Middlewares are tested with `httptest.NewRecorder` and
   `testutil.NextSpy`, which records whether the next handler was called and with which request.
6. **Temporary files** with `t.TempDir()` (removed automatically) and environment variables with
   `t.Setenv` (restored automatically).

Minimal example:

```go
package providers_test

func TestMyCallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		want bool
	}{
		{"high ratio", `{"ratio":"1.5"}`, true},
		{"neutral ratio", `{"ratio":"0.8"}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := providers.MyCallback([]byte(tt.body))

			testutil.RequireEmptyError(t, err)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
```

## What is unit tested and what is not

| Layer | Unit tests | How |
| --- | --- | --- |
| `providers/` callbacks | Yes | Fixtures in `testdata/` with real provider responses |
| `providers/` HTTP requests | Yes | `httptest` servers: retries, redirects, size limit, auth and secret redaction |
| `common/` middlewares and validators | Yes | `httptest.NewRecorder` + `testutil.NextSpy` |
| `cronjobs/` decisions and evaluation | Yes | Test callbacks registered with `registerCallback` |
| `models/` constructors | Yes | Directly |
| `services/` pure logic (parsers, files) | Yes | Directly, `t.TempDir()` for files |
| `data_validates/` paths that fail before querying MongoDB (Authorization header without token) | Yes | `httptest.NewRecorder` + `testutil.NextSpy` |
| `services/` Mongo queries, `data_validates/`, `controllers/` | No | Integration tests against a real MongoDB (pending) |

Mongo queries are not faked with mocks: a mock of the driver proves that the expected query was
written, not that the query does the right thing. Those belong in integration tests, kept apart with
the `//go:build integration` build tag and run with `go test -tags integration ./...` against a test
replica set, since transactions require one.

## When to add tests

- **New callback**: save a real provider response in `testdata/` and cover the exact threshold, both
  sides of it and the provider's error messages.
- **Bug fix**: first a test that reproduces the bug and fails, then the fix.
- **New tracking code**: if the function can be tested without MongoDB, add the case that produces it.
