## Why

Map-column key discovery selected every distinct key in the lookback window,
so a map such as `requestHeaders` filled the ad-hoc key dropdown with every
key it had ever held. Value preload has been bounded to an approximate top
100 since the topK guardrails change; key discovery had no bound at all.

## What Changes

- Map-key discovery returns up to 100 of the most common keys in the
  trailing window, most common first, using an approximate aggregation.
- The map-key and value preload statements no longer alias their result
  column, so a table column named `key` or `value` keeps working in an
  ad-hoc filter or as the primary key.
- README documents that rare map keys may be missing from the dropdown and
  need **Allow custom values**.
- Not **BREAKING**: the time filter, ad-hoc filter and guardrail settings of
  both statements are unchanged.
- Covered by frontend unit tests and Playwright e2e against a fixture with
  more than 100 map keys and a `key` primary key.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `adhoc-value-preload`: map-key discovery is bounded and ordered by
  frequency.
