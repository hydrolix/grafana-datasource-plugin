## 1. Implementation

- [x] 1.1 Add `pkg/plugin/connection_error.go`: the fixed messages, the error type that prints the message and unwraps to the cause, the transport classifier, and the connect-failure conversion (HTTP status and ClickHouse code reduced to a number)
- [x] 1.2 `Connect` in `pkg/plugin/driver.go`: log the ping failure once and return the connect-failure conversion
- [x] 1.3 `MutateQueryError` in `pkg/plugin/driver.go`: replace a transport failure with its fixed message after the exchange, redirect, response-limit and ClickHouse-exception branches; log the original; log a redirect on this path
- [x] 1.4 `HdxSqlDatasource.CheckHealth` in `pkg/plugin/hdx_sqlds.go`: for a stored-credential mode, log and replace the message of an error result
- [x] 1.5 `RedirectError` in `pkg/plugin/redirects.go`: message by status alone; keep the redacted `Location` field for the log; recover the error from its message for the health check
- [x] 1.6 `go.mod`: move the clickhouse-go fork pin to the `hydrolix/v2.51.0` head and run `go mod tidy`
- [x] 1.7 `connectFailure`: read a typed HTTP answer before a typed exception; `Connect` logs it with its status
- [x] 1.8 `MutateQueryError`: print a typed HTTP answer as status and exception, or status and body with clickhouse-go's quoting undone and a Turbine JSON body clickhouse-go cut put back whole; an answer whose body could not be read gets the fixed message
- [x] 1.9 `src/errors/errorBeautifier.ts`: end a message at `. (NAME)` for any `DB::…Exception` class; take a text without one to its end, after the older patterns

## 2. Go unit tests

- [x] 2.1 `connection_error_test.go`: table of typed and text errors for each transport kind, and errors that are an answer and must not be classified, an HTTP body that repeats a query full of transport phrases included; a resolver timeout, typed and as text
- [x] 2.2 `connection_error_test.go`: connect-failure conversion for auth codes, HTTP statuses, other exceptions, the generic fallback and a redirect, typed and as text; a body shaped like an exception or quoting the redirect message, and a body that could not be read, reduced to the status; no network detail in any message; cause reachable with `errors.Is`
- [x] 2.3 `connection_error_test.go`: instance build against a closed port (native, HTTP, TLS), a plain port behind TLS, and `httptest` servers answering 403 and 404
- [x] 2.4 `connection_error_test.go`: `MutateQueryError` for a dial error, a deadline inside a URL error, a SQL error that passes through even when it repeats a query about connections, and an unreadable body
- [x] 2.5 `datasource_test.go`: health check of an instance built through a TCP relay to the ClickHouse test container, before and after the relay closes, on both protocols
- [x] 2.6 `redirects_test.go`: the message carries the status and no part of the location
- [x] 2.7 `connection_error_test.go`: typed HTTP answers in the connect-failure table and the instance build: an exception-shaped body, a syntax error, and an exception announced by the header whose body is cut short
- [x] 2.8 `connection_error_test.go`: `MutateQueryError` for typed answers: an exception with and without a name, bodies quoted and not, a Turbine body cut by the header, each kind of unreadable body, and the `401` retry
- [x] 2.9 `connection_error_test.go`: the same messages for errors taken from clickhouse-go through a stub cluster
- [x] 2.10 `http_stream_test.go`: a connection cut partway through a streamed answer stays an error; a stream the server ended is complete
- [x] 2.11 Jest, `errorBeautifier.test.ts` and `datasource.test.ts`: native exceptions, another exception class, Turbine tokens, a Turbine database error inside an exception, an escaped query in a Turbine body, and the validation bar

## 3. E2E

- [x] 3.1 New spec `tests/connectionErrors.spec.ts`: create a stored-credential data source through the API pointing at a closed port; assert the health check answers an error with `Connection refused` and no host or port
- [x] 3.2 Same spec: create a Forward OAuth data source pointing at a closed port; assert a query's error is `Connection refused` and its source is downstream
- [x] 3.3 Same spec: a stored-credential data source with a wrong password reports `Authentication failed`
- [x] 3.4 `tests/queryValidation.spec.ts`: on a native data source the unknown-column error shows no `DB::Exception`

## 4. Verification

- [x] 4.1 `go vet ./...`, `golangci-lint run`, `go test -race ./...` pass
- [x] 4.2 Dev stack on the rebuilt backend: health check and query messages for refused, unresolved, timed out, TLS, wrong password, redirect and a non-cluster HTTP service; original errors present in the Grafana log
- [x] 4.3 E2E spec passes in PR CI
- [x] 4.4 The Go gates of 4.1 pass on the new clickhouse-go pin; `npm run typecheck`, `npm run lint`, `npm run test:ci` pass
- [ ] 4.5 E2E passes in PR CI with 3.4
