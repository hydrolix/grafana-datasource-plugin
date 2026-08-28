# Editor interpolation context as explicit parameters

Jira: [HDX-11854](https://hydrolix.atlassian.net/browse/HDX-11854) (follow-up)

## Why

The query editor's interpolated-SQL preview reads its time range, interval and
ad-hoc filters from two mutable fields on the datasource singleton, populated as
a side effect of `query()`. Nothing guarantees those values describe what the
user is currently looking at: the preview lags the time picker until the next
panel run, an internal metadata query can overwrite them with its own narrower
window, and a freshly-opened panel has to issue a throwaway query purely to
populate them. Grafana already hands the correct context to both readers as
arguments.

## What Changes

- Interpolation takes range, interval and filters as explicit parameters, which
  `QueryEditor` supplies from its own props (`src/components/QueryEditor.tsx`,
  `src/datasource.ts`).
- The interpolation interval is derived from the currently selected range rather
  than carried over from the last panel run, and is sent in a unit the backend
  can parse — fixing an existing failure where wide ranges produced an interval
  the interpolation resource rejected outright.
- Ad-hoc map-key discovery takes its time window and filters from the tag-keys
  options argument Grafana already supplies (`src/datasource.ts`).
- The dry-run fallback and its one-shot gate are removed from `QueryEditor`; a
  freshly-opened panel no longer runs a query just to populate datasource state.
- The two cached request fields are removed from `DataSource`, together with the
  metadata-query guard that existed to protect them.
- The preview reflects the current time-picker selection instead of the last
  completed run — a user-visible behavior change, non-breaking for dashboards.
- Test coverage: frontend unit tests plus Playwright e2e.

## Capabilities

### New Capabilities

- `editor-interpolation-context`: how the query editor and the ad-hoc filter
  paths obtain the time range, interval and filters they send to the
  interpolation resource and to metadata queries — sourced from caller-supplied
  context rather than from cached request state, including the interval unit
  contract with the backend.

### Modified Capabilities

- `adhoc-value-preload`: map-key discovery derives its time window and filters
  from the tag-keys options argument rather than from cached dashboard request
  state.

## Impact

- `src/datasource.ts` — signatures of `getInterpolatedQuery`,
  `interpolateQuery`, `getTagKeys` and `getTagKeysForMap`; removal of two public
  fields and of the metadata-query cache guard.
- `src/components/QueryEditor.tsx` — the interpolation debounce and the removal
  of dry-run state.
- `src/components/InterpolatedQuery.test.tsx`,
  `src/components/QueryEditor.test.tsx`, `src/datasource.test.ts` — updated for
  the new signatures.
- No Go backend changes; the interpolation wire contract is unchanged.
- Compatibility: the tag-keys time-range argument exists from Grafana 10.3. The
  plugin's floor is Grafana 10 and the verified matrix starts at 10.4, so a
  fallback keeps older minors working.
