import type { PluginOptions } from '@grafana/plugin-e2e';
import {defineConfig, devices} from '@playwright/test';
import baseConfig from './.config/playwright.config';

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
// require('dotenv').config();

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig<PluginOptions>(baseConfig, {
  // Add your own configuration here.
  // See https://grafana.com/developers/plugin-tools/how-to-guides/extend-configurations#extend-the-playwright-config for further info.

  /* test timeout */
  timeout: 60000,

  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: process.env.E2E_GRAFANA_URL || 'http://localhost:3000',

    screenshot: 'only-on-failure',
  },

  /* Configure projects for major browsers */
  projects: [
    // 2. Run tests in Google Chrome. Every test will start authenticated as admin user.
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        storageState: 'playwright/.auth/admin.json',
        viewport: { width: 1280, height: 1280 },
        timezoneId: 'UTC',
      },
      dependencies: ['auth'],
    },
  ],
});
