import type { Metadata } from "next";
import { AuthProvider } from "@/state/auth";
import "./globals.css";

export const metadata: Metadata = {
  title: "NexusWST",
  description: "Plataforma logística WeStorage",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="es">
      <body>
        <AuthProvider>{children}</AuthProvider>
      </body>
    </html>
  );
}
