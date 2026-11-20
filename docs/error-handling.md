# Error handling: what fails, what a person reads, what is logged

One page for every error the plugin shows from the cluster connection: where it
enters, how it is recognised, what text leaves the backend, what the frontend
does to that text, and which log line holds the original.

The requirements behind it are the `connection-error-messages` and
`query-error-messages` OpenSpec capabilities. This page describes the code.

## Who is who

| Name on this page | What it is |
|---|---|
| **clickhouse-go** | The ClickHouse driver that talks to the cluster, over native or HTTP. The plugin pins Hydrolix's fork of it (the `replace` line in `go.mod`), which adds Turbine support. It produces the raw errors. |
| **sqlds** | Grafana's library for SQL data sources. It owns the connection, runs the query, decides on retries, and calls the plugin's hooks. |
| **the plugin** | This repository's backend (`pkg/plugin`): `Connect`, `CheckHealth` and `MutateQueryError` are the three hooks where it rewrites an error. |
| **the frontend** | `src/`: shortens the backend's text before it is shown. |

## The rule

Every error belongs to one of two families, and the family decides everything
else.

| Family | Meaning | What a person reads | Where the original goes |
|---|---|---|---|
| **No answer** | Resolving, dialling, TLS, a timeout, a dropped connection. Nothing at the address answered, or the answer never arrived whole. | One fixed sentence per kind. No address, port, URL or body. | Grafana server log, error level. |
| **An answer** | Something answered: an HTTP status, a ClickHouse exception, a redirect. | **Save & test:** the status or the ClickHouse code, nothing of the text. **Query:** the answer as the cluster wrote it, because the query's author needs it. | Save & test: the log. Query: nowhere; the person already has it. |

## Where an error enters

There are three entry points. Only the third passes through `MutateQueryError`.

```
            Save & test                                        Panel, Explore, validation bar
                 │                                                          │
      ┌──────────┴─────────────┐                                            │
      ▼                        ▼                                            ▼
 (1) instance not built   (2) instance already built            (3) sqlds runs the query
     sqlds calls              HdxSqlDatasource.CheckHealth           db.QueryContext fails
     Hydrolix.Connect         sqlds pings the held connection        (error is typed, wrapped
     db.PingContext fails     and returns the error AS TEXT           in sqlds.ErrorQuery)
     (error is typed)                  │                                    │
      │                                │                                    │
      │  log the original              │  log the text                      │
      ▼                                ▼                                    ▼
 connectFailure(err)          connectFailure(errors.New(text))    Hydrolix.MutateQueryError(err)
      │                                │                                    │
      └───────────────┬────────────────┘                                    │
                      ▼                                                     ▼
        fixed sentence, status or code                     fixed sentence, or the cluster's answer
                      │                                                     │
                      ▼                                                     ▼
        frontend: testDatasource()                         frontend: query() / validateQuery()
        ErrorMessageBeautifier                             ErrorMessageBeautifier (+ ErrorExposer)
```

Things that are easy to miss:

- **Path 1 is also a panel path.** When `Connect` fails, the instance is not
  built, and the SDK answers every query on that data source with the same
  error. `MutateQueryError` never sees it.
- **The forwarding modes skip paths 1 and 2.** Forward OAuth and token exchange
  do not ping in `Connect` (there is no user to ping as), and `CheckHealth`
  answers them itself without dialling. Their first dial is a query, so their
  connection failures arrive on path 3.
- **Path 2 has only text.** sqlds hands the health check a string, so
  everything there is recognised by matching text.
- **The validation bar is path 3.** `/validate` runs `EXPLAIN` through the same
  sqlds query path.

## Path 1 and 2: `connectFailure`

`pkg/plugin/connection_error.go`. The first match wins. The order matters: an
HTTP status is read before anything else, because the body that comes with it
was written by someone else and can look like an exception or a network error.

```
err
 │
 ├─ is *RedirectError ────────────────► the redirect message (status only)
 │
 ├─ is *clickhouse.HTTPError ─────────► 401, 403 ► "Authentication failed"
 │                                      other    ► "The server answered HTTP <n>"
 │
 ├─ is *clickhouse.Exception ─────────► 192, 193, 194, 516 ► "Authentication failed"
 │                                      other ► "The server refused the connection
 │                                               (ClickHouse error code <n>)"
 │
 ├─ text holds "[HTTP nnn]" ──────────► as *clickhouse.HTTPError        ┐
 ├─ text holds "code: N, message:" ───► as *clickhouse.Exception        ├ path 2: text only
 ├─ text opens like a RedirectError ──► the redirect message            ┘
 │
 ├─ transportMessage(err) matches ────► its sentence; "The request timed out"
 │                                      becomes "Connection timed out"
 │
 └─ anything else ────────────────────► "Could not connect to the server"
```

Nothing falls through unreplaced: an error that matches no step gets the
generic sentence.

## The transport classifier: `transportMessage`

Used by both `connectFailure` and `MutateQueryError`. Each kind is recognised
by type where the error still has one, and by text otherwise.

```
text holds "[HTTP nnn] response body:" ──────────────► not transport: it is an answer
DNS error that is not a timeout, or "no such host" ──► "Host name could not be resolved"
TLS error types, "tls: ", "x509: ", HTTP on an
  HTTPS port ────────────────────────────────────────► "TLS handshake failed"
ECONNREFUSED, or "connection refused" ───────────────► "Connection refused"
deadline exceeded, a net timeout, "i/o timeout" ─────► while dialling ► "Connection timed out"
                                                       otherwise      ► "The request timed out"
net.OpError, url.Error, "dial tcp", "read tcp",
  "write tcp" ───────────────────────────────────────► "Could not connect to the server"
anything else ───────────────────────────────────────► not transport
```

## Path 3: `MutateQueryError`

`pkg/plugin/driver.go`. sqlds calls it with clickhouse-go's error wrapped in
`sqlds.ErrorQuery`. The first match wins.

```
err
 │
 1 ├─ *exchange.Error ──────────────────► "exchange: <reason>"
 │                                        source: plugin if Misconfigured, else downstream
 │
 2 ├─ *RedirectError ───────────────────► the redirect message (status only)
 │                                        LOG warn, with the redacted Location
 │
 3 ├─ httpclient.ErrResponseBodyTooLarge ► the response-limit sentence
 │
 4 ├─ *clickhouse.HTTPError
 │     │
 │     ├─ the body arrived (answerMessage)
 │     │    ├─ a ClickHouse exception ──► "[HTTP n] Code: N. Name: message"
 │     │    ├─ an "exception" that is a
 │     │    │  Turbine JSON body that was
 │     │    │  cut by clickhouse-go ────► "[HTTP n] response body: {"error": "Code: N. …", …}"
 │     │    └─ any other body ──────────► "[HTTP n] response body: <body, clickhouse-go's quoting undone>"
 │     │
 │     └─ the body could not be read ───► transportMessage, or "Could not connect to the server"
 │                                        LOG error "query answer could not be read"
 │
 5 ├─ *proto.Exception (native) ────────► "Code: N. Name: message"
 │
 6 ├─ transportMessage(err) matches ────► its sentence
 │                                        LOG error "query did not reach the cluster"
 │
 7 └─ anything else ────────────────────► unchanged: "error querying the database: <clickhouse-go text>"
```

All of these are downstream errors except a misconfigured exchange.

**Why the wrapper types.** Steps 1 to 4 and 6 return an error whose `Error()`
is the message and whose `Unwrap()` is the original (`shownError`,
`exchangeQueryError`, `responseLimitError`). sqlds decides what to do next from
the returned error:

- it retries only when `errors.Is(err, sqlds.ErrorQuery)` holds, the error is
  not a deadline, and the **message text** contains an entry of `RetryOn`;
- token-exchange mode sets `RetryOn` to `[HTTP 401]` with one retry, so a `401`
  must keep that status at the front of its message. `withStatus` prints it
  through clickhouse-go's own `HTTPError` for that reason.

Step 5 drops the chain (`errors.New`), so a native exception is never retried.

### Two cases `MutateQueryError` does not decide

| Case | What sqlds does | What a person reads |
|---|---|---|
| The query was cancelled | Returns the context error without calling `MutateQueryError`. | Grafana's own cancellation handling. |
| The query started and then failed **while rows were being read** (a dropped connection or the response limit partway through a stream) | Calls `MutateQueryError` only to learn the error source, and builds its own message. | `<clickhouse-go text>: Could not process SQL results` |

The second row is a known gap. clickhouse-go's text is not replaced there, and a
connection reset mid-stream prints its addresses:
`readData stream: … read tcp 127.0.0.1:51692->127.0.0.1:51691: read: connection reset by peer: Could not process SQL results`.
The log line of step 6 is still written.

## Frontend: `ErrorMessageBeautifier`

`src/errors/errorBeautifier.ts`. It shortens the backend's text and never
decides whether something is an error. It runs at three places in
`src/datasource.ts`:

| Call site | Shows in | If `beautify` returns nothing |
|---|---|---|
| `query()` | Panel and Explore errors. Also feeds `ErrorExposer` (the dashboard error variable, see `error-panel.md`). | The backend's text is kept. |
| `validateQuery()` | The query editor's validation bar. | The backend's text is kept. |
| `testDatasource()` | Save & test. | The backend's text is kept. |

```
message from the backend
 │
 ├─ "dial tcp … <ip:port>: X" or "Post … <ip:port>: X" ──► X
 │     (older backend text; the fixed sentences no longer match it)
 │
 ├─ find "{ … }" in the message and parse it as JSON ────► json (or none)
 │
 ├─ Turbine authentication error ────────────────────────► "<reason>:" and the JSON's fields
 │
 └─ database error
      text = json.error if there is JSON, else the whole message; newlines become spaces
      code = the number in "Code: N." or "code: N,"
      │
      ├─ no code ► "<Word message (Hydrolix …)>" ► message; otherwise the text as it is
      │
      └─ code found, first match wins:
           "DB::…Exception: <message>. (NAME)" ──────────► message   (HTTP form)
           "code: N, message: <message>" ────────────────► message   (clickhouse-go's form)
           "<ChTurbineDatabaseError message>" ───────────► message
           "DB::…Exception: <message>" to the end ───────► message   (native form)
           none ─────────────────────────────────────────► nothing: the caller keeps the text
```

`query()` also sends every query error to Grafana's frontend logging with
`logError`, before it is shortened.

## Catalogue

What a person reads for each kind of failure. "Sentence" columns are literal.

| What happened | Save & test | Panel or validation bar |
|---|---|---|
| Host name does not resolve | `Host name could not be resolved` | same |
| TLS handshake fails, or HTTP sent to an HTTPS port | `TLS handshake failed` | same |
| Nothing listens at the port | `Connection refused` | same |
| Timeout while dialling | `Connection timed out` | same |
| Timeout after the connection was made | `Connection timed out` | `The request timed out` |
| Any other network failure | `Could not connect to the server` | same |
| HTTP `401` or `403` | `Authentication failed` | `[HTTP 401] …` with the cluster's text; retried once in token-exchange mode |
| Any other HTTP status with a body | `The server answered HTTP <n>` | `[HTTP n] Code: N. Name: message` or `[HTTP n] response body: <body>`, then shortened by the frontend |
| HTTP status whose body could not be read | `The server answered HTTP <n>` | a network sentence from the rows above |
| ClickHouse exception, code 192, 193, 194 or 516 | `Authentication failed` | `Code: N. Name: message`, shortened by the frontend to the message |
| Any other ClickHouse exception | `The server refused the connection (ClickHouse error code <n>)` | as the row above |
| Redirect (`301`, `302`, `303`, `307`, `308`) | `the cluster answered HTTP <n>, a redirect, which the plugin does not follow: …` | same |
| Answer larger than Grafana's response limit | not applicable | `the cluster's answer exceeds Grafana's response limit (dataproxy.response_limit) …` |
| Token exchange failed | not tested: Save & test exchanges nothing. It reports only a missing configuration, `Token exchange is not configured on this Grafana: …` | `exchange: the console refused this account access to this cluster`, `exchange: the console could not be reached`, or `exchange: no exchange configuration is usable` |
| Failure while rows were being read | not applicable | `<clickhouse-go text>: Could not process SQL results` (the gap above) |
| Anything unrecognised | `Could not connect to the server` | `error querying the database: <clickhouse-go text>` |

Turbine's own shapes differ from stock ClickHouse: a wrong password answers
`400` or `500`, never `401` or `403`, so Save & test reads
`The server answered HTTP 400`.

## Log lines

Every line the error-presentation code writes to the Grafana server log.
Search for the message to find the original error.

| Message | Level | Written by | Fields |
|---|---|---|---|
| `cluster redirected; not following` | warn | `Connect`, `MutateQueryError` | `status`, `location` (scheme, host and path only) |
| `cluster answered with an error` | error | `Connect`, for an HTTP answer | `status`, `error` |
| `clickhouse exception` | error | `Connect`, for a native exception | `code`, `message`, `stack` |
| `connecting to the cluster failed` | error | `Connect`, for everything else | `error` |
| `health check failed` | error | `CheckHealth`, stored-credential modes | `error` (sqlds's text) |
| `query answer could not be read` | error | `MutateQueryError` step 4 | `error` |
| `query did not reach the cluster` | error | `MutateQueryError` step 6 | `error` |

sqlds adds `query failed: … Retrying` (warn) when it retries a `401` in
token-exchange mode.

A cluster's answer to a query (a SQL error, a `[HTTP n]` answer) is not logged
by this code: it is returned to the person in full.

## Where to change what

| To change | Edit | Guarded by |
|---|---|---|
| A fixed sentence, or how a network failure is recognised | `transportMessage` and the `msg…` constants in `pkg/plugin/connection_error.go` | `connection_error_test.go`; `tests/connectionErrors.spec.ts` |
| What Save & test says for an answer | `connectFailure`, `statusMessage`, `exceptionMessage` | `TestConnectFailureRepeatsNothingOfWhatAnswered`; `TestHealthCheckOfALostClusterNamesNoAddress` |
| How a query's HTTP answer is printed | `answerMessage`, `restoredJSONBody` | `TestMutateQueryErrorShowsAnHTTPAnswerAsTheClusterWroteIt` |
| Anything that depends on clickhouse-go's wording (`response body: `, `failed to read response body: `) | the two prefix constants in `connection_error.go` | `TestMutateQueryErrorOnErrorsFromClickhouseGo`, which takes the errors from clickhouse-go itself. Run it after every bump of the clickhouse-go pin. |
| The order of the query branches | `MutateQueryError` in `pkg/plugin/driver.go` | `connection_error_test.go`, `driver_exchange_test.go`, `transport_test.go` |
| The redirect message | `RedirectError` in `pkg/plugin/redirects.go`; keep `redirectText` in step with it | `redirects_test.go` |
| How a message is shortened in the UI | `src/errors/errorBeautifier.ts` | `errorBeautifier.test.ts`; `tests/queryValidation.spec.ts` |
