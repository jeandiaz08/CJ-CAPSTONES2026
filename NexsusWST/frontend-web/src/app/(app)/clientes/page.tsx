"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/state/auth";

// /clientes no es una pantalla: lleva al submenú que corresponde al perfil
// (Cliente → Perfiles empresa; Administrador o quien gestione perfiles → Administración clientes).
export default function Page() {
  const { session } = useAuth();
  const router = useRouter();
  const role = session?.user.role;
  useEffect(() => {
    if (role) router.replace(role === "cliente" ? "/clientes/perfiles-empresa" : "/clientes/administracion");
  }, [role, router]);
  return <p className="muted">Cargando…</p>;
}
