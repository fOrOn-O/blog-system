import { defineConfig } from '@playwright/test'
import path from 'node:path'

const python = process.env.AGENT_TEST_PYTHON || 'python'
export default defineConfig({
  testDir: './e2e', workers: 1, fullyParallel: false, timeout: 45000,
  outputDir: '../tmp/task13-playwright',
  use: { baseURL: 'http://127.0.0.1:15173', viewport: { width: 1440, height: 1000 },
    launchOptions: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE ? { executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE } : {} },
  webServer: [
    { command: 'go test ./internal/router -run ^TestTask13E2EServer$ -count=1 -timeout=10m', cwd: path.resolve('../backend'), env: { TASK13_E2E: '1' }, url: 'http://127.0.0.1:18080/health', timeout: 120000, reuseExistingServer: false },
    { command: `"${python}" -m uvicorn e2e_app:app --app-dir tests --host 127.0.0.1 --port 18000`, cwd: path.resolve('../agent-service'), url: 'http://127.0.0.1:18000/health', timeout: 60000, reuseExistingServer: false },
    { command: 'npm run dev -- --host 127.0.0.1 --port 15173 --strictPort', env: {
      BLOG_PROXY_TARGET: 'http://127.0.0.1:18080', AGENT_PROXY_TARGET: 'http://127.0.0.1:18000',
      VITE_API_BASE_URL: '/api/v1', VITE_AGENT_API_BASE_URL: '/agent-api/api/v1/agent'
    }, url: 'http://127.0.0.1:15173', timeout: 60000, reuseExistingServer: false }
  ]
})
