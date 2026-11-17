import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Genera .next/standalone: imagen de producción pequeña (sin node_modules completos)
  output: "standalone",
  // Siempre con valor: así se resuelve al compilar y las cuentas demo no entran al build.
  env: { NEXT_PUBLIC_SHOW_DEMO_ACCOUNTS: process.env.NEXT_PUBLIC_SHOW_DEMO_ACCOUNTS ?? "false" },
  // En dev se accede vía Traefik (http://localhost); permite el HMR por el proxy
  allowedDevOrigins: ["localhost", "127.0.0.1"],
  poweredByHeader: false,
};

export default nextConfig;
