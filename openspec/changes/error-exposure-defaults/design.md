## Context

`DataSource` builds its `ErrorExposer` with `jsonData.exposeErrors || defaultConfigs.exposeErrors`: the defaults apply only when the whole block is absent. `ErrorExposer.addErrorToVariable` returns early when any of `enabled`, `variableName`, `maxCount`, `ttl` is falsy, so a partial block records nothing. Otherwise it looks the variable up in `templateSrv.getVariables()`, appends the error, and writes `var-<variableName>` with `locationService.partial(..., true)`, whether or not the variable exists.

The config editor reads `jsonData.exposeErrors?.enabled ?? false` for the switch and shows the other fields only when that is true; its handlers spread the stored block. It fills defaults only for a brand-new data source (no host, port or protocol).

## Goals / Non-Goals

**Goals:**
- One resolution rule for the effective configuration, used by the runtime and the editor.
- The editor shows what the runtime does, and saving writes a complete block.
- No URL parameter when the dashboard has no matching variable.

**Non-Goals:**
- Changing the defaults themselves or the error text format.
- Migrating stored `jsonData`; a data source without the block keeps working as today.
- Backend changes.

## Decisions

### D1. Missing keys take their default: on, matching today's runtime

`resolveExposeErrors(exposeErrors?)` in `src/defaultConfigs.ts` returns `{ ...defaultConfigs.exposeErrors, ...exposeErrors }` with `undefined` and `null` values dropped first, so neither overrides a default. A `null` is what a blank provisioning key or an API payload stores; kept, it would show the switch on while the exposer's guard records nothing. `DataSource` and `ConfigEditor` both call it.

**Alternative considered:** treat a missing block as disabled and leave the editor as it is. Rejected: it silently switches the feature off for every existing provisioned or pre-0.10.0 data source that uses it today.

Merging key by key also fixes partial blocks. A block missing `enabled` now resolves to enabled, which is the same rule and the likely intent (the README's old `enables: true` typo is exactly that case). Explicit `false`, `0` and `""` are kept and still disable recording.

### D2. The editor edits the resolved block

The switch, the field visibility and the field values read the resolved configuration, and every change handler writes `{ ...resolved, <key>: value }`. The first save after any edit therefore stores a complete block, so the stored data and the editor agree from then on. Opening the page without editing writes nothing, as today.

### D3. Skip the write when the dashboard has no matching variable

`addErrorToVariable` returns before formatting the error when `templateSrv.getVariables()` has no variable named `variableName`. Without the variable nothing on the dashboard reads the parameter, so writing it only pollutes the URL; in Explore there are no dashboard variables, so Explore URLs stay clean.

**Alternative considered:** keep writing the parameter so a variable added later picks up past errors. Rejected: errors expire after `ttl` seconds anyway, and the parameter leaks error text into shared links.

## Risks / Trade-offs

- [A provisioned block that relied on a missing key to stay off is now on] → Only reachable with a malformed block; documented as a behaviour change in the proposal; unit test covers `{variableName}`-only resolving to enabled.
- [A dashboard that created its error variable after the first error loses that error] → Acceptable: the next error is recorded; covered by the exposer test that writes once the variable exists.
- [The editor writes defaults the user never chose on first edit] → They are the values the runtime already used, so behaviour is unchanged; the editor test asserts the written block.
