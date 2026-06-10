import { defineConfig } from "eslint/config";
import baseConfig from "./.config/eslint.config.mjs";

export default defineConfig([
  {
    ignores: [
      "**/dist/",
      "**/node_modules/",
      "**/playwright-report/",
      "**/playwright/",
      "**/test-results/",
      "**/coverage/",
      "**/hydrolix-hydrolix-datasource/",
      "**/.vscode/",
      "**/.idea/",
      "**/.venv/",
      "**/.eslintcache",
      "**/.bra.toml",
      "**/.nvmrc",
      "**/junit.xml",
      "**/error_panel/",
    ],
  },
  ...baseConfig,
  {
    rules: {
      "eol-last": "off",
      "no-var": "warn",
      "react-hooks/refs": "warn",
      "react-hooks/immutability": "warn",
    },
  },
]);
