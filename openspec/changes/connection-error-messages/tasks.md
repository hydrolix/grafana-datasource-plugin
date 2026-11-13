## 1. Implementation

- [x] 1.1 Add `pkg/plugin/connection_error.go`: the fixed messages, the error type that prints the message and unwraps to the cause, the transport classifier, and the connect-failure conversion (HTTP status and ClickHouse code reduced to a number)
- [x] 1.2 `Connect` in `pkg/plugin/driver.go`: log the ping failure once and return the connect-failure conversion
- [x] 1.3 `MutateQueryError` in `pkg/plugin/driver.go`: replace a transport failure with its fixed message after the exchange, redirect, response-limit and ClickHouse-exception branches; log the original; log a redirect on this path
- [x] 1.4 `HdxSqlDatasource.CheckHealth` in `pkg/plugin/hdx_sqlds.go`: for a stored-credential mode, log and replace the message of an error result
- [x] 1.5 `RedirectError` in `pkg/plugin/redirects.go`: message by status alone; keep the redacted `Location` field for the log

## 2. Go unit tests

- [x] 2.1 `connection_error_test.go`: table of typed and text errors for each transport kind, and errors that are an answer and must not be classified
- [x] 2.2 `connection_error_test.go`: connect-failure conversion for auth codes, HTTP statuses, other exceptions, the generic fallback and a redirect; no network detail in any message; cause reachable with `errors.Is`
- [x] 2.3 `connection_error_test.go`: instance build against a closed port (native, HTTP, TLS), a plain port behind TLS, and `httptest` servers answering 403 and 404
- [x] 2.4 `connection_error_test.go`: `MutateQueryError` for a dial error, a deadline inside a URL error, and a SQL error that passes through
- [x] 2.5 `datasource_test.go`: health check of an instance built through a TCP relay to the ClickHouse test container, before and after the relay closes, on both protocols
- [x] 2.6 `redirects_test.go`: the message carries the status and no part of the location

## 3. E2E

- [x] 3.1 New spec `tests/connectionErrors.spec.ts`: create a stored-credential data source through the API pointing at a closed port; assert the health check answers an error with `Connection refused` and no host or port
- [x] 3.2 Same spec: create a Forward OAuth data source pointing at a closed port; assert a query's error is `Connection refused` and its source is downstream
- [x] 3.3 Same spec: a stored-credential data source with a wrong password reports `Authentication failed`

## 4. Verification

- [x] 4.1 `go vet ./...`, `golangci-lint run`, `go test -race ./...` pass
- [x] 4.2 Dev stack on the rebuilt backend: health check and query messages for refused, unresolved, timed out, TLS, wrong password, redirect and a non-cluster HTTP service; original errors present in the Grafana log
- [ ] 4.3 E2E spec passes in PR CI
