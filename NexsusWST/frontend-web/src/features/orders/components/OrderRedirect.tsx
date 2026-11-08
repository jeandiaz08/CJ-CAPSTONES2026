"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { listRegistry } from "@/services/registry";

// /ordenes/[id]: abre el detalle de Todas las órdenes; si la orden ya está en el Registro
// histórico, abre su registro. Reemplaza la página antigua de detalle (Tarea 06, Extra).
export function OrderRedirect({ id }: { id: number }) {
  const router = useRouter();
  useEffect(() => {
    let current = true;
    const listing = `/ordenes/todas/listado?order=${id}`;
    listRegistry({ ids: [id], limit: 1 }, null)
      .then((r) => (r.data.length ? `/ordenes/registro?order=${id}` : listing))
      .catch(() => listing)
      .then((href) => { if (current) router.replace(href); });
    return () => { current = false; };
  }, [id, router]);
  return <p className="muted pad" role="status">Abriendo la orden…</p>;
}
