# plugin-oauth-keyed-pooling — implementation tasks

## 1. `injectConnectionArgs` helper

- [x] 1.1 Add `pkg/plugin/connection_args.go` exporting `injectConnectionArgs(body json.RawMessage, args map[string]string) (json.RawMessage, error)`. Round-trip through `map[string]json.RawMessage`; overwrite-not-merge; return fresh allocation.
- [x] 1.2 Treat `body == nil` as `{}` for ergonomics (callers don't have to pre-seed an empty object). Malformed JSON returns the unmarshal error verbatim.

## 2. Extend `Driver.MutateQueryData`

- [x] 2.1 In `pkg/plugin/driver.go`, after the plugin-settings parse and before the per-query loop, compute `connArgs map[string]string`:
  - `pluginSettings.CredentialsType == "forwardOAuth"` AND non-empty `req.GetHTTPHeaders().Get(backend.OAuthIdentityTokenHeaderName)` → set `connArgs["oauthToken"] = strings.TrimPrefix(value, "Bearer ")`.
  - Non-empty `req.GetHTTPHeaders().Get(OrgIdHeaderKey)` → set `connArgs["orgId"] = value` (any credentials type).
- [x] 2.2 Inside the existing per-query loop, build a single `patches` map covering `querySettings` (existing) plus `connectionArgs` (new, only when `len(connArgs) > 0`). Continue using `jsonSet`.
- [x] 2.3 Log only `err` + `refId` on serialize failures — never the JSON body, never the token.

## 3. Flat-shape readers

- [x] 3.1 Rewrite `getOAuthToken(args json.RawMessage)` to read `m["oauthToken"]` (no `Bearer ` prefix handling).
- [x] 3.2 Rewrite `getOrgId(args json.RawMessage)` to read `m["orgId"]`.
- [x] 3.3 Extract the common decode path into a private `readConnArg(args, key)` helper.
- [x] 3.4 Delete `getHeader` and the `sqlds.HeaderKey` import once no caller remains.

## 4. Lazy bootstrap in `Driver.Connect`

- [x] 4.1 In the `forwardOAuth` branch of `Connect`, replace `return nil, fmt.Errorf("cannot get auth header")` for the `args == nil` case with a path that proceeds to build a `*sql.DB` with `token == ""`. The resulting DB does not contact the server until first ping; `sqlds.NewConnector`'s bootstrap call succeeds.
- [x] 4.2 Keep the existing `args != nil && !ok` error path returning `backend.DownstreamError(fmt.Errorf("forwardOAuth: missing OAuth token in connection args"))`. This is the real per-query path; the upstream bootstrap call never hits it.
- [x] 4.3 Confirm `db.PingContext` continues to be skipped when `settings.CredentialsType == "forwardOAuth"` (existing behaviour). Lazy bootstrap relies on this.

## 5. Tests

- [x] 5.1 Add `pkg/plugin/connection_args_test.go`: round-trip (no `connectionArgs` initially → present after; existing `connectionArgs` overwritten not merged); nil body treated as empty object; malformed body returns an error.
- [x] 5.2 Update `pkg/plugin/driver_test.go::TestGetHeader` — replace nested-shape cases with flat-shape cases (or delete and recreate as `TestReadConnArg`). Adjust `TestGetOAuthToken` and `TestGetOrgId` for the flat input shape and the no-prefix contract.
- [x] 5.3 Add `TestMutateQueryData_InjectsConnectionArgs` in `driver_test.go`: matrix of (forwardOAuth on/off) × (OAuth header present/empty) × (Org header present/empty). Assert that `req.Queries[0].JSON.connectionArgs` reflects exactly the keys that should be set per matrix cell.
- [x] 5.4 Add cross-request safety test: construct two `*backend.QueryDataRequest`s with no shared fields; run `MutateQueryData` on each with different OAuth tokens; assert second request's queries carry the second token, not the first.
- [x] 5.5 Add `TestConnect_LazyBootstrapForwardOAuth` in `driver_test.go`: with `forwardOAuth` + `args == nil`, `Connect` returns a non-nil `*sql.DB` and no error. (Don't ping; the contract is that bootstrap succeeds without contacting the server.)
- [x] 5.6 Add `TestConnect_MissingOAuthTokenWhenArgsPresent`: with `forwardOAuth` + `args = {"orgId": "5"}` (no oauthToken), `Connect` returns a wrapped error.

## 6. Quality gates

- [x] 6.1 `go build ./...` — clean.
- [x] 6.2 `go vet ./...` — clean.
- [x] 6.3 `golangci-lint run` — no new issues vs C5.
- [x] 6.4 `go test -race ./...` — green.
- [x] 6.5 `npm run typecheck && npm run lint && npm run test:ci` — green. (No frontend changes in this PR; this is regression-only.)
- [x] 6.6 Playwright e2e — deferred to coordinated-set verification; C4 alone cannot exercise the OAuth path end-to-end against the test stack.

## 7. Commit

- [x] 7.1 Single commit including code + design + tasks + specs.
- [x] 7.2 Commit message: `pkg/plugin: OAuth + Org-Id keyed pooling via MutateQueryData (C4)`. Body summarises (a) flat connectionArgs shape, (b) lazy bootstrap, (c) reader rewrite.
