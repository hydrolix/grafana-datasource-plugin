/**
 * React-version-agnostic stand-in for `react/jsx-runtime` and
 * `react/jsx-dev-runtime`.
 *
 * `webpack.config.ts` un-externalizes both runtimes so the plugin still loads
 * on Grafana <= 12.2, which does not publish them in its SystemJS
 * shared-dependency map. Without this shim webpack bundles the copy from local
 * `node_modules` — React 18.3.1 — whose implementation dereferences
 * `React.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED.ReactCurrentOwner`
 * at module scope. React 19 removed that object, so on Grafana >= 13.2
 * (React-19-only; 13.0/13.1 still default to React 18) evaluating
 * `dist/module.js` throws
 * `TypeError: Cannot read properties of undefined (reading 'ReactCurrentOwner')`
 * and Grafana renders "Could not load plugin" instead of the datasource.
 *
 * Building the runtime on `React.createElement` instead uses only public API,
 * so one bundle works on React 16 through 19 and `grafanaDependency` can stay
 * at `>=10.4.0`.
 *
 * `createElement` reads `children` straight off the props object and extracts
 * `key`, so both runtimes reduce to a single call. `jsxs` (static children,
 * already an array) needs no separate handling for the same reason, and
 * `jsxDEV`'s trailing `isStaticChildren` / `source` / `self` arguments are
 * development-only diagnostics that are safe to drop.
 */
import { createElement, Fragment, type ReactElement } from 'react';

type Props = Record<string, unknown>;

// The public overloads assume statically-known props; the runtime signature is
// deliberately opaque here because callers are compiler output.
const create = createElement as unknown as (type: unknown, props: Props) => ReactElement;

export function jsx(type: unknown, props: Props, key?: unknown): ReactElement {
  return create(type, key === undefined ? props : { ...props, key });
}

export const jsxs = jsx;

export function jsxDEV(type: unknown, props: Props, key?: unknown): ReactElement {
  return jsx(type, props, key);
}

export { Fragment };

const jsxRuntime = { Fragment, jsx, jsxs, jsxDEV };

export default jsxRuntime;
