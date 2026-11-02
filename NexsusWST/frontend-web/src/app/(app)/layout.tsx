"use client";

// Estructura común de la aplicación: sidebar + barra superior + contenido.
// Todas las páginas dentro de src/app/(app)/ usan este layout y exigen sesión.
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Sidebar } from "@/components/Sidebar";
import { Toaster } from "@/components/Toast";
import { ChevronRight } from "lucide-react";
import { useAuth } from "@/state/auth";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const { session, ready, logout } = useAuth();
  const router = useRouter();
  const [menuOpen, setMenuOpen] = useState(false);
  const [collapsed,setCollapsed]=useState(false);
  useEffect(()=>{try{setCollapsed(localStorage.getItem("nexus.sidebar.collapsed")==="true")}catch{}},[]);
  const toggleCollapsed=()=>setCollapsed(value=>{try{localStorage.setItem("nexus.sidebar.collapsed",String(!value))}catch{}return !value});

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
