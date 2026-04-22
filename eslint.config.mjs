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
      "**/.vscode/",
      "**/.idea/",
      "**/.venv/",
      "**/.eslintcache",
      "**/.bra.toml",
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
