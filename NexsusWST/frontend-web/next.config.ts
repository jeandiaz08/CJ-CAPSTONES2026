import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Genera .next/standalone: imagen de producción pequeña (sin node_modules completos)
  output: "standalone",
  // En dev se accede vía Traefik (http://localhost); permite el HMR por el proxy
  allowedDevOrigins: ["localhost", "127.0.0.1"],
  poweredByHeader: false,
};

export default nextConfig;
