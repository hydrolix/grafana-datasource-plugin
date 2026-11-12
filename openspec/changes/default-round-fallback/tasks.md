## 1. Implementation

- [x] 1.1 `PluginSettings.DefaultRound` (`json:"defaultRound,omitempty"`), read in `NewPluginSettings` only when it is a JSON string
- [x] 1.2 `HdxInterpolator.defaultRound`; `interpolate` uses `query.Round` if non-empty, else `defaultRound`, and keeps `"0"` as the opt-out
- [x] 1.3 `NewHdxSqlDatasource` sets `interp.defaultRound` from the parsed settings

## 2. Go unit tests

- [x] 2.1 `interpolator_test.go`: default only, query round over default, query `"0"` with default, neither set
- [x] 2.2 `settings_test.go`: string `defaultRound` parsed; number ignored without error
- [x] 2.3 `hdx_sqlds_test.go` (or nearest): `NewHdxSqlDatasource` wires `defaultRound` into the interpolator

## 3. E2E

- [x] 3.1 New spec: a data source with `defaultRound: "1m"` returns a minute-aligned `$__fromTime` for a query without its own round, and an unaligned one for `round: "0"`

## 4. Verification

- [x] 4.1 `go vet ./...`, `golangci-lint run`, `go test -race ./...` pass
- [x] 4.2 Backend binary rebuilt in the dev container (fixed code confirmed in the binary); run the new e2e locally once
