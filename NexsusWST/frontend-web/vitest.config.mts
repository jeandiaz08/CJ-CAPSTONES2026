// Pruebas de comportamiento (npm test): componentes con happy-dom y servicios simulados.
import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

const local = (path: string) => fileURLToPath(new URL(path, import.meta.url));

export default defineConfig({
  esbuild: { jsx: "automatic" },
  resolve: {
    alias: [
      { find: /^@\//, replacement: local("./src/") },
      // Next no corre en las pruebas: navegación, imágenes, enlaces y Auth.js van simulados.
      { find: "next/navigation", replacement: local("./tests/stubs/navigation.js") },
      { find: "next/image", replacement: local("./tests/stubs/image.js") },
      { find: "next/link", replacement: local("./tests/stubs/link.js") },
      { find: "next-auth/react", replacement: local("./tests/stubs/nextauth.js") },
    ],
  },
  test: { environment: "happy-dom", include: ["tests/**/*.test.jsx"], testTimeout: 20000 },
});
