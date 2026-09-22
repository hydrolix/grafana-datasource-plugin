import path from 'path';
import type { Configuration } from 'webpack';
import grafanaConfig, { Env } from './.config/webpack/webpack.config';

/**
 * Externals that Grafana only publishes to its SystemJS shared-dependency map
 * from 12.3.0 onwards (`public/app/features/plugins/loader/sharedDependencies.ts`).
 *
 * The create-plugin scaffold externalises them unconditionally. On Grafana
 * <= 12.2 the resulting `define([... "react/jsx-runtime" ...])` header makes
 * SystemJS request `/react/jsx-runtime`, get a 404, and fail the whole plugin
 * load — the datasource then renders as "Type: undefined" with no config or
 * query editor at all.
 *
 * `src/` compiles with the classic JSX transform and never emits these
 * imports, but pre-built dependencies do (`@grafana/assistant` ships JSX
 * compiled with the automatic runtime). Bundling the runtime shim instead
 * costs ~2 KB and keeps `grafanaDependency: ">=11.0.0"` in `src/plugin.json`
 * truthful.
 *
 * Drop this override once the plugin's minimum supported Grafana is >= 12.3.
 */
const JSX_RUNTIME_EXTERNALS = ['react/jsx-runtime', 'react/jsx-dev-runtime'];

/**
 * ...but bundle *our* runtime, not React's.
 *
 * Resolving these requests to `node_modules/react` pulls in React 18.3.1's
 * implementation, which reads
 * `React.__SECRET_INTERNALS_DO_NOT_USE_OR_YOU_WILL_BE_FIRED.ReactCurrentOwner`
 * at module scope. React 19 removed that object, so the bundle throws on load
 * under Grafana >= 13.2 (React-19-only) and the plugin never renders. The shim
 * uses only `React.createElement`, so a single bundle works on React 16-19.
 * See `src/compat/jsxRuntime.ts`.
 */
const JSX_RUNTIME_SHIM = path.resolve(__dirname, 'src', 'compat', 'jsxRuntime.ts');

const config = async (env: Env): Promise<Configuration> => {
  const baseConfig = await grafanaConfig(env);

  return {
    ...baseConfig,
    externals: (baseConfig.externals as unknown[]).filter(
      (external) => !(typeof external === 'string' && JSX_RUNTIME_EXTERNALS.includes(external))
    ) as Configuration['externals'],
    resolve: {
      ...baseConfig.resolve,
      alias: {
        ...baseConfig.resolve?.alias,
        ...Object.fromEntries(JSX_RUNTIME_EXTERNALS.map((request) => [`${request}$`, JSX_RUNTIME_SHIM])),
      },
    },
  };
};

export default config;
