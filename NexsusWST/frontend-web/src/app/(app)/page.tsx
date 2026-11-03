"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { PageHeader } from "@/components/PageHeader";
import { LiveBadge } from "@/components/StatusBadge";
import { STATUS, STATUS_ORDER } from "@/features/orders/statuses";
import { useOrdersSocket } from "@/hooks/useOrdersSocket";
import { getSummary, type OrderStatus } from "@/services/orders";
import { useAuth } from "@/state/auth";
import { canAccessPath } from "@/state/menuAccess";

// Inicio: conteo de órdenes por estado, actualizado en vivo.
export default function HomePage() {
  const { session, companyId } = useAuth();
  const [counts, setCounts] = useState<Partial<Record<OrderStatus, number>>>({});
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    getSummary(companyId)
      .then((r) => {
        setCounts(r.data);
        setError(null);
      })
      .catch((e) => setError(e.message));
  }, [companyId]);

  useEffect(load, [load]);
  const live = useOrdersSocket(load);
  // Empaque solo llega a su bandeja; quien no ve órdenes, solo el saludo.
  const user = session?.user;
  const tiles: { status: OrderStatus; href: string }[] = user && canAccessPath(user, "/ordenes/todas/listado")
    ? STATUS_ORDER.filter((s) => s !== "cancelled").map((status) => ({ status, href: `/ordenes/todas/listado?status=${status}` }))
    : user && canAccessPath(user, "/ordenes/empaque")
      ? [{ status: "picked", href: "/ordenes/empaque" }, { status: "packing", href: "/ordenes/empaque?tab=mine" }]
      : [];

  return (
    <>
      <PageHeader title={`Hola, ${session?.user.name.split(" ")[0] ?? ""}`} subtitle="Resumen de órdenes por estado" actions={<LiveBadge live={live} />} />
      {error && <p className="error">{error}</p>}
      <div className="stat-grid">
        {tiles.map(({ status: s, href }) => (
          <Link key={s} href={href} className={`stat tone-${STATUS[s].tone}`}>
            <span className="stat-value">{counts[s] ?? 0}</span>
            <span className="stat-label">{STATUS[s].label}</span>
          </Link>
        ))}
      </div>
    </>
  );
}
