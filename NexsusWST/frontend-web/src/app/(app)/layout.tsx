"use client";

// Estructura común de la aplicación: sidebar + barra superior + contenido.
// Todas las páginas dentro de src/app/(app)/ usan este layout y exigen sesión.
import { useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Sidebar } from "@/components/Sidebar";
import { Toaster } from "@/components/Toast";
import { ChevronRight } from "lucide-react";
import { useAuth } from "@/state/auth";
import { canAccessPath } from "@/state/menuAccess";
import { useNotificationFeed } from "@/hooks/useNotificationFeed";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const { session, ready, profileError, logout } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  useNotificationFeed();
  const permitted = !!session && canAccessPath(session.user, pathname);
  const [menuOpen, setMenuOpen] = useState(false);
  const [collapsed,setCollapsed]=useState(false);
  useEffect(()=>{try{setCollapsed(localStorage.getItem("nexus.sidebar.collapsed")==="true")}catch{}},[]);
  const toggleCollapsed=()=>setCollapsed(value=>{try{localStorage.setItem("nexus.sidebar.collapsed",String(!value))}catch{}return !value});

  useEffect(() => {
    if (ready && !session && !profileError) router.replace("/login");
  }, [ready, session, profileError, router]);

  // Una URL que no le corresponde a este perfil vuelve a su Home. El servidor valida igual.
  useEffect(() => {
    if (session && !permitted) router.replace("/");
  }, [session, permitted, router]);

  // Si el navegador restaura la página desde su caché al ir «atrás», se vuelve a validar la sesión.
  useEffect(() => {
    const onPageShow = (e: PageTransitionEvent) => {
      if (e.persisted) window.location.reload();
    };
    window.addEventListener("pageshow", onPageShow);
    return () => window.removeEventListener("pageshow", onPageShow);
  }, []);

  useEffect(() => {
    // 401 del servidor: el BFF ya intentó renovar. Sesión terminada -> al login; usuario
    // deshabilitado o token inválido -> se cierra también en Keycloak.
    const onUnauthorized = (e: Event) => {
      if ((e as CustomEvent<{ code?: string }>).detail?.code === "session_expired") {
        window.location.replace(`/login?next=${encodeURIComponent(window.location.pathname + window.location.search)}`);
      } else {
        logout();
      }
    };
    window.addEventListener("nexus:unauthorized", onUnauthorized);
    return () => window.removeEventListener("nexus:unauthorized", onUnauthorized);
  }, [logout]);

  if (profileError) {
    return (
      <main className="login">
        <div className="login-card">
          <h1>No se pudo iniciar sesión</h1>
          <p className="error">{profileError}</p>
          <button type="button" className="btn primary block" onClick={logout}>Volver al inicio de sesión</button>
        </div>
      </main>
    );
  }

  if (!ready || !permitted) return <p className="muted pad">Cargando…</p>;

  return (
    <div className={`shell ${collapsed?"sidebar-is-collapsed":""}`}>
      <Sidebar collapsed={collapsed} onCollapse={toggleCollapsed} open={menuOpen} onClose={() => setMenuOpen(false)} />
      {!menuOpen&&<button className="floating-menu" aria-label="Abrir menú" aria-expanded={menuOpen} onClick={()=>setMenuOpen(true)}><ChevronRight/></button>}
      <div className="main">

        <div className="content">{children}</div>
      </div>
      <Toaster />
    </div>
  );
}
