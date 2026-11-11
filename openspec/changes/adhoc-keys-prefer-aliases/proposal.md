## Why

Hydrolix normalization profiles publish identity aliases (`client_ip String ALIAS cliIP`) so that one stored column carries both a customer-facing and a canonical name. Today the plugin's field pickers either show only the physical base (the alias is dropped when its DESCRIBE expression is not backtick-quoted) or show both names side by side, so a dashboard author cannot present the canonical OCSF name alone (CFB-4004). PR #249 adds that preference as an opt-in, but it does so through a second DESCRIBE pipeline in `DataSource` that bypasses the metadata provider, drops Map columns, rejects the quoted view names the plugin itself produces, and leaves the underlying alias-parsing bug in place. This change keeps the feature and reshapes the implementation to fit the existing architecture.

Identity aliases also mislead the validation bar: on Hydrolix summary tables `$__timeFilter(reqTimeSec)` filters the key column `toStartOfDay(reqTimeSec)` through its alias, yet draws the unfiltered-primary-key warning.

## What Changes

- Alias resolution in the metadata provider's ad-hoc key parsing accepts both bare and backtick-quoted identity expressions, so a bare identity alias is no longer silently dropped. Non-breaking: computed aliases stay excluded as today.
- Ad-hoc filter keys carry which physical column an identity alias stands for, so a presentation policy can be applied without re-reading the schema.
- A pure "prefer identity aliases" policy helper hides a physical base from new choices when a same-type identity alias exists; computed, type-changing, unresolved and chained aliases stay visible.
- `getTagKeys` applies that policy over the already-fetched key list when the datasource option `adHocKeysPreferAliases` is on. Map columns and their key enumeration are preserved; quoted view names keep working. `getTagValues` is unchanged, so an existing filter on a hidden base still loads values.
- PR #249's `preferredColumns` query-variable opt-in is dropped: it returned the same ad-hoc key list under a second name and required the table to be spelled a second time in dashboard JSON. `HdxQuery` and `metricFindQuery` return to their pre-PR shape.
- The parallel `describeColumns` / `preferredColumnsFromResponse` DESCRIBE path introduced by PR #249 is removed from `DataSource`.
- The datasource config editor gains an "Ad hoc filter keys prefer aliases" switch in the additional settings, next to the ad-hoc table variable; the option defaults off.
- The validation bar's primary-key check accepts a filter on an identity alias of the key column. Non-breaking.
- `docs/adhoc-keys-prefer-aliases.md` is rewritten for the final behavior.
- Unit coverage for the parser, the policy, the ad-hoc entry point and the validator; Go integration and Playwright e2e coverage over ClickHouse fixtures with identity aliases.

## Capabilities

### New Capabilities
- `adhoc-keys-prefer-aliases`: identity-alias resolution in ad-hoc key discovery and the opt-in policy that prefers an identity alias over its physical base in ad-hoc keys.
- `query-validation`: the validation bar's primary-key check, seeded with the identity-alias rule only.

### Modified Capabilities
- (none — existing specs cover the backend macro and the value-preload guardrails, neither of which changes)

## Impact

- `src/editor/metadataProvider.ts` (`getKeyMap`, `AdHocFilterKeys` producer), `src/types.ts` (`AdHocFilterKeys` gains `aliasFor`, `HdxQuery` loses `preferredColumns`), `src/editor/preferIdentityAliases.ts` (policy only), `src/datasource.ts` (`getTagKeys`, removal of the raw DESCRIBE path and of the variable branch in `metricFindQuery`), `src/components/ConfigEditor.tsx` and `src/labels.ts` (the switch).
- Tests: `src/components/ConfigEditor.test.tsx`, `tests/configEditor.spec.ts`, `tests/helpers.ts`, `src/editor/metadataProvider.test.ts`, `src/editor/preferIdentityAliases.test.ts`, `src/datasource.adHocKeysPreferAliases.test.ts`, `testdata/containers/initdb.sql`, a new `tests/adHocKeysPreferAliases.spec.ts`; existing ad-hoc e2e key assertions over `e2e.adhoc_topk` may need the new alias column accounted for.
- Backend: `pkg/plugin/validator.go`, `pkg/plugin/metadata.go` and their tests; e2e in `tests/queryValidation.spec.ts`.
- No `plugin.json` change; no new dependency.
- A query variable saved with PR #249's `{"preferredColumns": ...}` JSON (a local prototype only, never released) now returns no options, as any object without `rawSql` does.
