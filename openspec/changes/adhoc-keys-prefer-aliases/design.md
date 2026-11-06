## Context

The frontend already has one schema pipeline for ad-hoc filters: `MetadataProvider.tableKeys(table)` issues `DESCRIBE <table>`, parses it with `getKeyMap`, memoizes the result per table, and both `getTagKeys` and `getTagValues` read from it. `getKeyMap` resolves an alias's type through its target (`getType`), but only recognizes a target when the `default_expression` is backtick-wrapped, which is the Hydrolix DESCRIBE spelling. The bare spelling, which plain ClickHouse and the metadata-only Hydrolix DESCRIBE emit for `ALIAS cliIP`, yields an empty target and the alias is filtered out as untyped.

PR #249 (already on this branch) layered the alias preference on a second pipeline: `DataSource.describeColumns` validates the table with its own regex, calls `executeQuery("DESCRIBE …")` directly, and `preferredColumnsFromResponse` re-parses the frame. That path bypasses the memoized key list, has no Map handling, rejects the backticked names `adHocFilterTableName` produces for views like ``akamai.`logs#redacted` ``, and leaves `getKeyMap`'s spelling gap unfixed. The `preferredColumns` variable opt-in also lives on `HdxQuery`, the shape every panel query sends to the backend, and returns the same key list the ad-hoc path produces under a second name.

Constraints: no plugin-side caching beyond the metadata provider's existing per-table memoization; `DataSource` holds no per-request state; the option defaults off; Grafana 11 dashboards must keep working.

## Goals / Non-Goals

**Goals:**
- Fix identity-alias detection at its source so both DESCRIBE spellings resolve.
- Express "prefer the alias" as a pure function over `AdHocFilterKeys[]`, applied by the two opt-in entry points.
- Remove the parallel DESCRIBE path and the ad-hoc table regex from `DataSource`.
- Keep Map columns, quoted view names and default-database resolution working under the opt-in.
- Keep PR #249's observable contract: alias shown instead of base, SQL untouched, hidden base still resolves values.

**Non-Goals:**
- A variable query editor.
- Changing Monaco completion, the schema browser or arbitrary SQL variable results.
- Ranking between several identity aliases of one base; all remain visible.
- Backend changes.

## Decisions

**D1. Fix alias target parsing in `getKeyMap`.** The alias target is the backtick content when the expression is wrapped, otherwise the trimmed expression. A target that is not a column name still resolves to type `""` and is filtered out, so computed aliases (`lower(cliIP)`, `toStartOfHour(datetime)`) behave exactly as today in both spellings. Rationale: the bug is in the parser; fixing it there makes stock behavior consistent across Hydrolix and ClickHouse and lets the new policy reuse the result. *Alternative considered:* leave `getKeyMap` alone and parse again in the policy (PR #249). Rejected: two parsers for one frame drift, and stock keeps dropping bare aliases.

**D2. Expose the identity-alias relation on the key.** `AdHocFilterKeys` gains optional `aliasFor?: string`, set only when the key is an alias whose target is an existing non-alias column with the same declared type. Everything else (computed, cast, missing target, alias-to-alias) leaves it unset. Rationale: the policy needs exactly this relation and nothing else from DESCRIBE; a `string | undefined` field is the smallest surface. The field is internal to the frontend; Grafana ignores extra properties on `MetricFindValue`.

**D3. Policy is a pure helper, `preferIdentityAliases(keys)`.** It collects every `aliasFor` and returns the keys minus those names. Lives in `src/editor/preferIdentityAliases.ts`, which loses `columnsFromResponse` / `preferredColumnsFromResponse`. Rationale: a filter over an array is trivially unit-testable and has no I/O; Map columns, array columns and quoted names pass through untouched because nothing is re-read.

**D4. `getTagKeys` applies the policy to the memoized key list, then enumerates Map keys as before.** When `jsonData.adHocKeysPreferAliases` is true the only difference from stock is one `preferIdentityAliases` call between `tableKeys(table)` and the Map enumeration. Rationale: reuses the single-fetch of the primary key, the memoized DESCRIBE and the existing Map path; opt-in no longer costs one DESCRIBE per dropdown open and no longer hides Map columns. `getTagValues` reverts to stock: it already reads the full key list, so a filter on a hidden base keeps resolving values with no extra branch.

**D5. The `preferredColumns` query-variable opt-in is dropped.** `preferredColumns` is removed from `HdxQuery` and `metricFindQuery` returns to its pre-PR shape (`Partial<HdxQuery> | string`, `rawSql` only). Rationale: after D4 the variable returned exactly the ad-hoc key list of a table under a second name, with the table spelled again in dashboard JSON next to the datasource's `adHocTableVariable`; it had no editor and a type set narrower than a SQL field dropdown wants. The ticket's need, canonical names in the ad-hoc filter, is met by D4 alone. *Alternative considered:* keep it renamed (`adHocKeysOf`) and governed by the same flag. Rejected for this change: a field-choice variable deserves a proper variable query type with an editor, which is its own change.

**D6. The option is a switch in the config editor.** A `Switch` field labelled "Ad hoc filter keys prefer aliases" sits in the additional settings between the ad-hoc table variable and the condition variable, writing `jsonData.adHocKeysPreferAliases` through the same `onOptionsChange` path as the other booleans. Rationale: a flag reachable only through provisioning or the datasource API is hard to discover; the e2e sets it through the form like every other field. *Alternative considered:* keep it JSON-only as PR #249 did. Rejected: nothing about the behavior is experimental once the parser fix lands.

**D7. Documentation.** `docs/adhoc-keys-prefer-aliases.md` is rewritten to the final contract (both spellings, Map columns kept, quoted views supported, default database applied, ad-hoc keys only). The README pointer stays.

## Risks / Trade-offs

- [Stock behavior change: bare-spelled identity aliases now appear as ad-hoc keys where they were previously dropped] → Unit test on `getKeyMap` for both spellings; e2e over the ClickHouse fixture (bare spelling) asserts stock shows both names. The addition is a strict superset of today's keys.
- [Existing e2e key-list assertions over `e2e.adhoc_topk` break once the fixture gains an alias column] → Grep the ad-hoc specs for exact key lists before adding the column; add the alias to the fixture and update assertions in the same task.
- [`aliasFor` leaks into `MetricFindValue` objects Grafana receives] → Grafana already receives `type` on these keys; unit test asserts `getTagKeys` output shape with the flag off is byte-for-byte unchanged.
- [Two identity aliases on one base both stay visible] → By design (no ranking); covered by a unit case.

## Migration Plan

No data or backend migration. PR #249 was a local prototype; a variable saved with its `preferredColumns` JSON returns no options after this change. Rollback is reverting the commit; the option defaults off, so stock users see only the alias-parsing fix.

## Open Questions

- None blocking. A field-choice query variable with an editor is deferred until a dashboard needs it.
