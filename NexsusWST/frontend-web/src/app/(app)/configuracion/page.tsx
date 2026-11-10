"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

// /configuracion no es una pantalla: lleva a Usuarios WeStorage («Ajustes generales» es «Próximamente»).
export default function Page() {
  const router = useRouter();
  useEffect(() => {
    router.replace("/configuracion/usuarios");
  }, [router]);
  return <p className="muted">Cargando…</p>;
}
