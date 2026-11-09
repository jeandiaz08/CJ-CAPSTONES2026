// Antes de cada corrida e2e se recarga el seed de demostración, para que las pruebas no dependan
// de lo que dejó la corrida anterior (retiro.spec entrega órdenes, clientes.spec crea empresas…).
// El seed vive solo en la máquina de Jean (respaldo/seed/seed_demo.sql, fuera de git): si no está,
// se avisa y se sigue, y las specs que lo necesitan lo dicen al fallar. E2E_SKIP_SEED=1 lo omite.
import { execFileSync } from "node:child_process";
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

const ROOT = path.resolve(__dirname, "..", "..");
const SEED = path.join(ROOT, "respaldo", "seed", "seed_demo.sql");

export default function globalSetup() {
  if (process.env.E2E_SKIP_SEED === "1") {
    console.log("[e2e] E2E_SKIP_SEED=1: no se recarga el seed de demostración.");
    return;
  }
  if (!existsSync(SEED)) {
    console.warn(`[e2e] No existe ${SEED}: se corre sin recargar el seed de demostración.`);
    return;
  }
  // Primero el de desarrollo (perfiles, empresas y permisos base), luego el de demostración.
  execFileSync("docker", ["compose", "run", "--rm", "seed-dev"], { cwd: ROOT, stdio: "ignore" });
  execFileSync(
    "docker",
    ["compose", "exec", "-T", "postgres", "sh", "-c", 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -q -f -'],
    { cwd: ROOT, input: readFileSync(SEED), stdio: ["pipe", "ignore", "inherit"] },
  );
  console.log("[e2e] Seed de demostración recargado.");
}
