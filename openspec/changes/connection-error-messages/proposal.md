## Why

A data source that cannot connect reports the driver's raw error. That error names the address and port it dialled, the resolver it asked, the request URL with the user name, and for an HTTP answer the response body or the redirect target. The host is configurable, so on a shared Grafana a data source admin can read from those messages what listens on addresses they cannot otherwise reach, and what an internal HTTP service answers. Grafana's plugin review of v0.13.4 asked for fixed messages in "Save & test"; the same text reaches panels, so the change covers both.

## What Changes

- "Save & test" reports a connection failure as one fixed sentence per kind of failure: host not resolved, connection refused, connection timed out, TLS handshake failed, authentication failed, an HTTP status, a ClickHouse error code, or a generic could-not-connect.
- The same applies to a health check on a data source that connected earlier and has since lost its cluster.
- A panel whose query never reached the cluster shows the same fixed sentences. An answer from the cluster, a SQL error included, is still shown.
- The clickhouse-go fork moves to its v2.51.0 branch, which types an HTTP error's status and exception.
- A cluster's HTTP answer to a query reads as its status followed by the exception, or by the body as the cluster wrote it.
- The frontend beautifier shows a ClickHouse exception by its message alone over either protocol, and keeps a Turbine syntax error whole.
- A refused redirect is reported by its status alone; where it pointed is no longer shown.
- The original error, and a redirect's target, are written to the Grafana server log.
- The error source stays downstream, and the cause stays reachable under the message for the retry and deadline checks that inspect it.
- Go unit, Jest and Playwright e2e coverage.

Not breaking: no dashboard, query or saved configuration changes meaning. Anything that matched on the old error text, such as an alert on a specific `dial tcp` message or on `error querying the database: sendQuery: [HTTP …`, no longer matches.

## Capabilities

### New Capabilities

- `connection-error-messages`: what a connection failure may and may not say in a health check, a panel and a redirect refusal, and where the original error goes.
- `query-error-messages`: how a cluster's answer to a failed query reads in a panel and in the validation bar, over HTTP and native.

### Modified Capabilities

- `hdx-sqlds-wrapper`: the wrapper answers `CheckHealth` itself rather than promoting the upstream method.

## Impact

- `pkg/plugin/connection_error.go` (new), `driver.go`, `hdx_sqlds.go`, `redirects.go`, and their tests.
- `src/errors/errorBeautifier.ts` and its tests.
- `go.mod`: the clickhouse-go fork pin, and the modules `go mod tidy` moved with it.
- Operators diagnosing a failed connection read the cause in the Grafana server log, not in the UI. On Grafana Cloud that log is not available to them.
- No `plugin.json` changes.
