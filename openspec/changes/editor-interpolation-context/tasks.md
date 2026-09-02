# Tasks: editor-interpolation-context

## 1. Interval derivation helper

- [x] 1.1 Add an interval-derivation helper (`src/editor/timeRangeUtils.ts`)
      that takes a `TimeRange` plus an optional resolution and returns the
      interval as a millisecond duration string, using
      `rangeUtil.calculateInterval` for the numeric part
- [x] 1.2 Add a named default-resolution constant in `src/constants.ts` for the
      case where the panel supplies no `maxDataPoints`
- [x] 1.3 Unit tests: derived values for 6h / 19d / 90d / 1y / 5y at high and
      low resolutions; every result parses as a Go duration (assert the `ms`
      suffix and a numeric body, and that no result carries `d`/`w`/`M`/`y`);
      default resolution applies when none is passed

## 2. Interpolation takes explicit context

- [x] 2.1 Define the interpolation context type (range, interval, filters) in
      `src/types.ts`
- [x] 2.2 Change `getInterpolatedQuery` in `src/datasource.ts` to take the
      context parameter and post its values, reading nothing from instance state
- [x] 2.3 Change `interpolateQuery` to take and forward the context parameter
- [x] 2.4 Unit tests: the posted body carries the supplied range, the derived
      interval and the supplied filters; a second call with a different range
      posts a correspondingly different interval

## 3. QueryEditor supplies the context

- [x] 3.1 Build the context in `src/components/QueryEditor.tsx` from
      `props.range`, `props.data?.request?.filters` and the derived interval,
      with resolution from `props.data?.request?.maxDataPoints`
- [x] 3.2 Replace the debounce's two-branch body with a single interpolation
      call; update the dependency array to the props the context is built from
      (dropping `props.datasource.options`)
- [x] 3.3 Delete `dryRun`, `dryRunTriggered` and the `skipNextRun` plumbing that
      exists only to serve it; check whether `skipNextRun` has any remaining
      consumer in `src/datasource.ts` and remove it too if not
- [x] 3.4 Update `src/components/QueryEditor.test.tsx` and
      `src/components/InterpolatedQuery.test.tsx` for the new signatures
- [x] 3.5 Unit test: interpolation fires on the first debounce pass with no
      cached datasource state present

## 4. Ad-hoc paths take explicit context

- [x] 4.1 Add the `DataSourceGetTagKeysOptions` parameter to `getTagKeys` in
      `src/datasource.ts` and thread `timeRange` / `filters` into
      `getTagKeysForMap`
- [x] 4.2 Give `getTagKeysForMap` explicit range and filters parameters,
      falling back to `templateSrv.timeRange` when no range is supplied
- [x] 4.3 Switch `getTagValues`'s filter source from instance state to its own
      options argument
- [x] 4.4 Unit tests: map-key discovery uses the supplied range capped to 24h;
      omitting the range falls back to the template service's range; the
      existing capping and guardrail assertions still hold

## 5. Remove the cached state

- [x] 5.1 Delete the `options` and `filters` fields from `DataSource` and the
      assignments in `query()`, including the `CoreApp.Unknown` /
      `ZERO_TIME_RANGE` guard that protected them
- [x] 5.2 Remove the now-obsolete tests that pinned the caching behaviour
      (`this.options` / `this.filters` assertions in `src/datasource.test.ts`),
      keeping the annotation-retag assertions that do not depend on the fields
- [x] 5.3 Confirm `ZERO_TIME_RANGE` still has a consumer in
      `src/editor/metadataProvider.ts`; leave it in place if so
- [x] 5.4 Run `npm run typecheck`, `npm run lint`, `npm run test:ci` (verify
      test count ≠ 0)

## 6. E2E

- [x] 6.1 Rebuild `dist/` via the `build-plugin` skill before any e2e run
- [x] 6.2 E2E: on a freshly-opened panel, enter SQL and show the interpolated
      query without setting a time range first; assert the SQL renders and that
      no data query POST fires between panel-edit open and the preview
      appearing
- [x] 6.3 E2E: change the time range without running the panel and assert the
      interpolated preview's time bounds follow the new selection
- [x] 6.4 E2E: on a dashboard with a `Map`-typed ad-hoc key, assert map-key
      discovery still returns keys and its query carries the capped window
- [x] 6.5 Confirm the existing `tests/queryEditor.spec.ts` interpolation test
      passes with the dry-run path gone
- [x] 6.6 Run the affected e2e specs on the verified Grafana matrix (10.4.x,
      11.5.x, 12.0.x, 12.3.x, 13.0.x) per the `e2e-dev` skill

## 7. Docs and follow-through

- [x] 7.1 Update `.claude/findings/2026-05-14-interpolate-uses-datasource-options-singleton.md`
      to record that the refactor landed, and close its TODO list
- [x] 7.2 Add a note to the `e2e-dev` skill that production webpack strips
      `console.log` / `console.info` (`drop_console` in
      `.config/webpack/webpack.config.ts`), so temporary in-plugin
      instrumentation must use `console.warn`
- [x] 7.3 Update `.claude/CLAUDE.md`'s "Datasource specifics" section, which
      documents the `this.options` / `this.filters` caching that this change
      removes
- [x] 7.4 Run `go vet ./...`, `golangci-lint run`, `go test -race ./...` in the
      dev container (no backend change expected — confirm green)
- [x] 7.5 Add an `annotations` spec delta removing the two cached-state
      requirements and rewording the retag rationale; at archive/sync, also
      trim the "(ad-hoc filter cache, cached request options)" clause from the
      synced spec's Purpose paragraph
