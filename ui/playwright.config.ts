import { defineConfig, devices } from '@playwright/test';
import path from 'node:path';

const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? 'http://127.0.0.1:32180';
const repoRoot = path.resolve(__dirname, '..');

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  timeout: 90_000,
  reporter: [
    ['list'],
    ['html', { open: 'never', outputFolder: 'playwright-report' }],
  ],
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        browserName: 'chromium',
      },
    },
  ],
  webServer: {
    command: './scripts/e2e/start-stack.sh',
    cwd: repoRoot,
    url: `${baseURL}/api/v1/health`,
    timeout: 90_000,
    reuseExistingServer: false,
    stdout: 'pipe',
    stderr: 'pipe',
    env: {
      ...process.env,
      DAGU_E2E_STATE_DIR:
        process.env.DAGU_E2E_STATE_DIR ??
        path.resolve(__dirname, 'test-results/e2e-stack'),
      DAGU_E2E_SERVER_PORT: process.env.DAGU_E2E_SERVER_PORT ?? '32180',
      DAGU_E2E_COORDINATOR_PORT:
        process.env.DAGU_E2E_COORDINATOR_PORT ?? '32181',
      DAGU_E2E_WORKER_ID: process.env.DAGU_E2E_WORKER_ID ?? 'worker-1',
    },
  },
});
