## 1. Frontend

- [x] 1.1 In `src/constants.ts`, build `AD_HOC_MAP_KEY_QUERY` with `arrayJoin(topKArray(AD_HOC_MAP_KEY_TOP_K)(mapKeys(${column})))` instead of `distinct(arrayJoin(mapKeys(${column})))`.
- [x] 1.2 Drop the result-column alias from `AD_HOC_MAP_KEY_QUERY` and `AD_HOC_VALUE_QUERY`; `getValuesFromResponse` reads the first field by position.
- [x] 1.3 Update the README `Map` column keys guardrail note.

## 2. Tests

- [x] 2.1 Update the pinned map-key and value statements in `src/ast.test.ts` and assert neither statement aliases its result column.
- [x] 2.2 Add a `src/datasource.test.ts` case that `getTagKeys` keeps the discovered keys in query order.
- [x] 2.3 Add the `e2e.adhoc_map_topk` fixture (152 map keys, primary key named `key`) to `testdata/containers/initdb.sql`.
- [x] 2.4 Add a `tests/adHocMapKeys.spec.ts` case asserting discovery returns 100 keys with `hot` then `warm` first, and the dropdown offers `attributes['hot']` before `attributes['warm']`.
