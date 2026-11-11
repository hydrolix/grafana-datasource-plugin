## 1. Metadata provider: alias parsing

- [x] 1.1 In `src/editor/metadataProvider.ts` `getKeyMap`, resolve the alias target from either the backtick content or the trimmed bare expression; keep `getType` semantics (unknown target → `""` → excluded).
- [x] 1.2 Add optional `aliasFor?: string` to `AdHocFilterKeys` in `src/types.ts` and set it in `getKeyMap` only for a same-type identity alias whose target is a non-alias column.
- [x] 1.3 Unit tests in `src/editor/metadataProvider.test.ts`: bare and backticked identity alias listed with the target type and `aliasFor`; computed alias excluded in both spellings; cast / missing / chained aliases carry no `aliasFor`; existing `getKeyMap` expectations still pass.

## 2. Preference policy

- [x] 2.1 Replace the contents of `src/editor/preferIdentityAliases.ts` with a pure `preferIdentityAliases(keys: AdHocFilterKeys[]): AdHocFilterKeys[]`; delete `columnsFromResponse` and `preferredColumnsFromResponse`.
- [x] 2.2 Rewrite `src/editor/preferIdentityAliases.test.ts`: base hidden with unrelated and Map columns kept in order; several aliases of one base all remain; input without `aliasFor` returned unchanged.

## 3. Datasource entry points

- [x] 3.1 In `src/datasource.ts` `getTagKeys`, remove the opt-in early-return branch; when `jsonData.adHocKeysPreferAliases` is true, pass `tableKeys(table)` through `preferIdentityAliases` before the Map-key enumeration.
- [x] 3.2 Revert `getTagValues` to the single `metadataProvider.tableKeys(table)` lookup.
- [x] 3.3 Delete `describeColumns` and `preferredColumnChoices` and their imports (`columnsFromResponse`, `preferredColumnsFromResponse`, `SUPPORTED_TYPES`, `ARRAY_TYPES` if now unused).
- [x] 3.4 Remove `preferredColumns` from `HdxQuery` in `src/types.ts`; add no variable-query type.
- [x] 3.5 Revert `metricFindQuery` to its pre-PR shape (`Partial<HdxQuery> | string`, `rawSql` only); delete the `preferredColumns` branch and `preferredColumnChoices`.
- [x] 3.6 Rewrite `src/datasource.adHocKeysPreferAliases.test.ts` against `tableKeys` / `executeQuery` spies: alias offered and Map keys enumerated under the opt-in; quoted view `logs#redacted` with default database works; one DESCRIBE across two `getTagKeys` calls; hidden base still yields values with time filter, filters and guardrails; option off returns the stock shape; `metricFindQuery` string and `rawSql` forms unchanged; `applyTemplateVariables` leaves alias/base SQL untouched.
- [x] 3.7 Run `npm run typecheck`, `npm run lint`, `npm run test:ci` (confirm test count is non-zero).

## 4. Documentation

- [x] 4.1 Rewrite `docs/adhoc-keys-prefer-aliases.md` for the final contract: both DESCRIBE spellings, Map columns kept, quoted views and default database supported, ad-hoc keys only, switch in the config editor, default off, SQL never rewritten.
- [x] 4.2 Keep the README pointer; verify the link target path is unchanged.

## 5. Config editor switch

- [x] 5.1 Add the `adHocKeysPreferAliases` label (test id, label, description) to `src/labels.ts` and a `Switch` field after the ad-hoc table variable in `src/components/ConfigEditor.tsx`, writing `jsonData.adHocKeysPreferAliases` via `onOptionsChange`.
- [x] 5.2 Unit test in `src/components/ConfigEditor.test.tsx`: renders unchecked by default and writes `true` on toggle.
- [x] 5.3 Add `adHocKeysPreferAliasesSwitch()` to the `ConfigPageLocator` interface in `tests/helpers.ts`; assert hidden/visible/unchecked in `tests/configEditor.spec.ts`; set the flag through the switch in `tests/adHocKeysPreferAliases.spec.ts` instead of the datasource API.

## 6. E2E

- [x] 6.1 Grep `tests/adHoc*.spec.ts` for exact key-list assertions over `e2e.adhoc_topk`; add `status_alias String ALIAS status` to that table in `testdata/containers/initdb.sql` and update any affected assertions (stock now lists both `status` and `status_alias`).
- [x] 6.2 Add `tests/adHocKeysPreferAliases.spec.ts` using the `e2e-dev` skill conventions: a datasource with `adHocKeysPreferAliases: true` and `adHocTableVariable` set offers `status_alias` and not `status`, still offers `attrs['…']` map keys, and filtering on `status_alias = rare1` yields the fixture's single row; a stock datasource offers both names.
- [x] 6.3 `npm run build`, then run the new spec, `tests/configEditor.spec.ts` and the touched ad-hoc specs via the `e2e-dev` skill; confirm green.

## 7. Validation bar: identity alias of the primary key

- [x] 7.1 Add `MetadataProvider.QueryAliases` in `pkg/plugin/metadata.go`: an uncached `DESCRIBE TABLE` (default database applied, user headers carried) returning ALIAS columns with their expressions.
- [x] 7.2 In `pkg/plugin/validator.go`, when the leading primary-key column is unfiltered, accept a filter on any alias whose expression names exactly that column (bare or backticked); skip the check when the lookup fails.
- [x] 7.3 Unit tests in `pkg/plugin/validator_test.go`: summary key filtered through its alias, bare identity alias, alias filtered by a CTE reader, computed alias and alias of another column still warn, no DESCRIBE when the column is filtered, lookup failure skips and logs; `Validate` sends `DESCRIBE TABLE` with the user's headers.
- [x] 7.4 Integration test in `pkg/plugin/validator_suite_test.go` over a summary-shaped ClickHouse table: filtered through the alias is valid, unfiltered warns.
- [x] 7.5 Add `e2e.validation_summary` (key column `toStartOfDay(reqTimeSec)`, identity alias `reqTimeSec`) to `testdata/containers/initdb.sql`.
- [x] 7.6 E2E in `tests/queryValidation.spec.ts`: `$__timeFilter(reqTimeSec)` on `e2e.validation_summary` shows "Query is valid"; filtering only `hits` warns naming `toStartOfDay(reqTimeSec)`. Run via the `e2e-dev` skill.
