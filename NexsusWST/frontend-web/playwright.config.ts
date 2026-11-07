// Pruebas de punta a punta (npm run test:e2e) contra el stack de desarrollo levantado y con
// el seed de demostración recargado: modifican datos (manifiestos, incidencias).
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 180_000,
  workers: 1,
  reporter: "list",
  use: {
    baseURL: process.env.E2E_BASE ?? "http://localhost",
    channel: process.env.E2E_CHANNEL ?? "chrome", // el Chrome instalado; sin descargar navegadores
    headless: true,
  },
});
