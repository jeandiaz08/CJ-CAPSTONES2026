// Capturas de pantalla para documentación (docs/pantallas/). No son pruebas: se corren aparte con
//   npx playwright test -c playwright.capturas.config.ts
// Reutiliza la configuración e2e (Chrome instalado, baseURL, recarga del seed en global-setup).
import { defineConfig } from "@playwright/test";
import base from "./playwright.config";

export default defineConfig({
  ...base,
  testDir: "./capturas",
  timeout: 600_000,
});
