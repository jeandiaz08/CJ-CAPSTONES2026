"use client";

// Estructura común de la aplicación: sidebar + barra superior + contenido.
// Todas las páginas dentro de src/app/(app)/ usan este layout y exigen sesión.
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Sidebar } from "@/components/Sidebar";
import { Topbar } from "@/components/Topbar";
import { useAuth } from "@/state/auth";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const { session, ready, logout } = useAuth();
  const router = useRouter();
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    if (ready && !session) router.replace("/login");
  }, [ready, session, router]);

  useEffect(() => {
    // 401 del servidor: usuario deshabilitado o sesión inválida -> se cierra
    // la sesión también en Keycloak. Un token recién vencido lo renueva auth.tsx.
    const onUnauthorized = (e: Event) => {
      if ((e as CustomEvent<{ code?: string }>).detail?.code !== "token_expired") logout();
    };
    window.addEventListener("nexus:unauthorized", onUnauthorized);
    return () => window.removeEventListener("nexus:unauthorized", onUnauthorized);
  }, [logout]);

  if (!ready || !session) return <p className="muted pad">Cargando…</p>;

  return (
    <div className="shell">
      <Sidebar open={menuOpen} onClose={() => setMenuOpen(false)} />
      <div className="main">
        <Topbar onMenu={() => setMenuOpen(true)} />
        <div className="content">{children}</div>
      </div>
    </div>
  );
}
