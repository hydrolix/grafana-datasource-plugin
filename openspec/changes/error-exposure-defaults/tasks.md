## 1. Implementation

- [x] 1.1 Add `resolveExposeErrors(exposeErrors?)` to `src/defaultConfigs.ts`: defaults merged key by key, `undefined` values dropped
- [x] 1.2 `src/datasource.ts`: build the `ErrorExposer` with `resolveExposeErrors(jsonData?.exposeErrors)`
- [x] 1.3 `src/components/ConfigEditor.tsx`: read the switch, field visibility and field values from the resolved block; every handler writes `{ ...resolved, <key>: value }`
- [x] 1.4 `src/errors/errorExposer.ts`: return before formatting when the dashboard has no variable named `variableName`

## 2. Unit tests

- [x] 2.1 `resolveExposeErrors`: missing block, partial block, block without `enabled`, explicit `0` / `false`, explicit `undefined`
- [x] 2.2 `datasource.test.ts`: the exposer receives the resolved configuration for a missing and a partial block
- [x] 2.3 `ConfigEditor.test.tsx`: missing block renders on with defaults and no `onOptionsChange`; turning it off writes the complete block; a stored off block renders off with fields hidden
- [x] 2.4 New `errorExposer.test.ts`: no write without the variable, write with it, no write when disabled

## 3. E2E

- [x] 3.1 `tests/configEditor.spec.ts`: a data source created through the API without `exposeErrors` shows the Error Exposure switch on with the default values

## 4. Verification

- [x] 4.1 `npm run typecheck`, `npm run lint`, `npm run test:ci` pass (test count ≠ 0)
- [x] 4.2 `npm run build` produces a clean `dist/`; run the new e2e locally once (`e2e-dev`)
