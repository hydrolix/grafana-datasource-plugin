# Design: editor-interpolation-context

## Context

`DataSource` carries two mutable public fields written only as a side effect of
`query()`:

```ts
if (request.app !== CoreApp.Unknown && request.range !== ZERO_TIME_RANGE) {
  this.options = request;          // read by getInterpolatedQuery, getTagKeysForMap
}
if (request.app === CoreApp.Dashboard) {
  this.filters = request.filters;  // same readers
}
```

Two consumers read them instead of taking parameters:

- `getInterpolatedQuery` posts `range` + `interval` + `filters` to the
  `/interpolate` backend resource — the editor's SQL preview.
- `getTagKeysForMap` uses the cached range as the base window for map-key
  discovery.

Three defects follow from the design, all documented in
`.claude/findings/2026-05-14-interpolate-uses-datasource-options-singleton.md`:

1. The preview reflects the last completed run, not the current time picker.
2. A freshly-opened panel has `options === undefined`, so `QueryEditor` runs a
   throwaway `dryRun()` query (`onRunQuery` with `skipNextRun`) purely to
   populate the field, plus a `useDebounce` dependency on the mutable field to
   re-arm after it lands.
3. Any query passing through `query()` with a real range overwrites the fields.
   The ad-hoc value preload began doing exactly that when its range was capped
   to a trailing 24h window, which is why the `CoreApp.Unknown` guard above
   exists at all.

The compatibility floor is Grafana >= 10; the locally verified matrix is
10.4.16, 11.5.4, 12.0.2, 12.3.1, 13.0.1.

### Measured facts this design rests on

Probed on the dev stack (Grafana 13.0.1) by instrumenting `QueryEditor` and
reading the values through Playwright:

| Question | Result |
| --- | --- |
| `props.range` populated before the first run? | yes — real bounds, `raw.from: "now-6h"`, while the cached field is still undefined |
| `props.data.request` populated before the first run? | yes — `state: "Done"`, `request` present |
| ad-hoc filters reachable from props? | yes — `props.data.request.filters` byte-identical to the cached `filters` |
| does `getTagKeys` receive a time range? | yes — `DataSourceGetTagKeysOptions` extends `DataSourceFilteringRequestOptions` (`timeRange?`, `filters`), and the current signature takes no arguments and discards it |
| `props.range` re-resolved per render? | yes — moves between renders for a `now`-relative range |

Backend contract, read from `pkg/api/routes.go:58,193`: the resource declares
`Interval string` and parses it with `time.ParseDuration`, returning an error to
the caller when parsing fails.

Verified against the container's Go toolchain:

```
30s        -> ok        1d  -> unknown unit "d"
30m        -> ok        7d  -> unknown unit "d"
2h         -> ok        1y  -> unknown unit "y"
86400000ms -> 24h0m0s
```

And `rangeUtil.calculateInterval` (the same routine that produces
`DataQueryRequest.interval`) emits those rejected units at low resolutions:

```
range=90d maxDataPoints=20  -> "1d"
range=1y  maxDataPoints=20  -> "7d"
range=5y  maxDataPoints=20  -> "1y"
```

So the *current* code already breaks: a wide dashboard range with a low
`maxDataPoints` sends `"1d"` and the preview fails outright. This design must
not reproduce it.

## Goals / Non-Goals

**Goals:**

- Interpolation and map-key discovery receive their context from callers.
- The preview reflects the range the user currently has selected.
- The interval sent to the backend is always parseable by `time.ParseDuration`.
- Remove `this.options`, `this.filters`, `dryRun`, and the metadata-query guard.

**Non-Goals:**

- No change to the `/interpolate` wire shape or any Go code.
- No change to what the ad-hoc value preload sends; its 24h cap and guardrail
  settings stay exactly as they are.
- No new configuration surface.
- Not addressing `metricFindQuery`'s own range resolution, which already falls
  back to `templateSrv.timeRange`.

## Decisions

### D1: Pass context as an explicit parameter object

`interpolateQuery` and `getInterpolatedQuery` take a context argument carrying
`range`, `interval` and `filters`; `QueryEditor` builds it from its own props.

Rationale: the component already holds everything the datasource was reaching
back for. A parameter makes the dependency visible, removes the ordering
coupling between an unrelated `query()` call and a render, and makes the values
unit-testable without simulating a panel run.

Alternative considered: keep the fields but refresh them from the component on
render. Rejected — it preserves the shared mutable state and the ordering
hazard while adding a second writer.

### D2: Derive the interval from the current range, not from the last request

`props.data.request.interval` is available, but it belongs to the last completed
run. Pairing it with a freshly-selected `props.range` would produce a preview
whose bucket width contradicts its own bounds. The interval is therefore
computed from the range in hand with `rangeUtil.calculateInterval`.

Alternative considered: ship `props.data.request.interval` verbatim. Rejected —
it reintroduces, in the interval, exactly the staleness this change removes from
the range, and it inherits the unit bug in D3.

### D3: Emit the interval as a millisecond duration

The derived interval is sent as `<intervalMs>ms` rather than Grafana's
formatted string. Grafana's vocabulary includes `d`, `w`, `M` and `y`;
`time.ParseDuration` accepts only `ns`, `us`, `ms`, `s`, `m` and `h`. A
millisecond suffix is always in both vocabularies, so no range width can produce
an unparseable value.

Alternative considered: widen the Go parser to accept `d`/`y`. Rejected — it
changes backend behavior for a frontend formatting problem, and day/year units
are ambiguous across DST and leap years.

### D4: Resolution for the interval calculation comes from the panel, with a constant fallback

`calculateInterval` needs a resolution (`maxDataPoints`). It is taken from
`props.data?.request?.maxDataPoints` when present, else a named constant.

Rationale: `maxDataPoints` is a function of panel width and panel settings, not
of the time range, so reading it from the last request does not reintroduce
range-staleness — moving the time picker does not change it. Using the panel's
real value keeps the preview's bucket width representative of what the panel
will actually execute.

### D5: Map-key discovery reads the tag-keys options argument

`getTagKeys` gains the `DataSourceGetTagKeysOptions` parameter Grafana already
passes and threads `timeRange` and `filters` down to `getTagKeysForMap`. Where
`timeRange` is absent — it exists only from Grafana 10.3, below the verified
matrix but inside the declared floor — the code falls back to
`templateSrv.timeRange`, the same fallback `runQuery` already uses.

Alternative considered: keep `this.options` solely for this reader. Rejected —
one remaining reader keeps the field, the guard, and the whole defect class
alive for no benefit.

### D6: Remove the dry-run fallback and the metadata-query guard together with the fields

With context supplied per call, the `options === undefined` branch is
unreachable: `dryRun`, `dryRunTriggered`, the debounce's `else` branch and the
`props.datasource.options` dependency all become dead. The `CoreApp.Unknown`
guard in `query()` exists only to protect a field that no longer exists, so it
goes in the same pass — last, so that at no intermediate commit is the field
both live and unguarded.

### D7: Test layering

Unit tests cover interval derivation (including the wide-range cases that
previously produced `d`/`y` units), context construction from props, and the
tag-keys threading. E2E covers the user-visible behavior the unit tests cannot
reach: that the preview reflects the current picker without a panel run, and
that opening a fresh panel and clicking Show issues no extra query.

## Risks / Trade-offs

- [Removing `dryRun` changes what `tests/queryEditor.spec.ts` exercises. That
  test currently passes because `timeRange.set` triggers an implicit run, so it
  never covered the fallback] → Keep the test and add one that clicks Show
  without setting a range first; the proving signal is that no `/api/ds/query`
  POST fires between panel-edit open and the preview rendering.
- [The derived interval can differ from the interval the panel will actually
  use, because the panel's minimum-interval setting is not applied] → Accepted:
  the preview is representative, not a promise of the executed plan. Proving
  signal is a unit test pinning derived values per range width.
- [`options.timeRange` is absent below Grafana 10.3, inside the declared floor
  but below the verified matrix] → `templateSrv.timeRange` fallback; proving
  signal is a unit test with the argument omitted.
- [Signature changes ripple into existing component tests] → Contained to three
  test files; typecheck is the proving signal.
- [Reading `props.range` makes the preview change without a panel run, which
  users may read as the preview disagreeing with the panel] → This is the
  intended fix for the documented staleness defect; the panel and preview agree
  again as soon as the panel runs.

## Migration Plan

Internal refactor with no persisted state and no wire change, so no data
migration and no feature flag. Ordering within the change matters: introduce the
parameters and switch the callers first, then delete `dryRun`, then delete the
fields and the guard. Rollback is a revert of the change; the `/interpolate`
contract is untouched, so a reverted frontend talks to the same backend.

## Open Questions

- Should the derived interval honour a minimum interval (panel setting or the
  datasource's default round) rather than being computed from range and
  resolution alone? Deferred — it only sharpens bucket width in the preview.
- Once `templateSrv.timeRange` is the only pre-10.3 fallback in the file,
  is the declared Grafana 10.0-10.2 support still worth carrying, given the
  verified matrix starts at 10.4?
