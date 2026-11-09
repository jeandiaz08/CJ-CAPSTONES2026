// Pruebas de punta a punta (npm run test:e2e) contra el stack de desarrollo levantado. Modifican
// datos (manifiestos, incidencias, empresas): global-setup recarga el seed antes de cada corrida.
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  globalSetup: "./e2e/global-setup.ts", // recarga el seed de demostración antes de cada corrida
  timeout: 180_000,
  workers: 1,
  reporter: "list",
  use: {
    baseURL: process.env.E2E_BASE ?? "http://localhost",
    channel: process.env.E2E_CHANNEL ?? "chrome", // el Chrome instalado; sin descargar navegadores
    headless: true,
  },
});
