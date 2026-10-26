"use client";

import { Suspense, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Plus, Search } from "lucide-react";
import { PageHeader } from "@/components/PageHeader";
import { LiveBadge } from "@/components/StatusBadge";
import { CreateOrderDialog } from "@/features/orders/CreateOrderDialog";
import { OrdersTable } from "@/features/orders/OrdersTable";
import { STATUS, STATUS_ORDER } from "@/features/orders/statuses";
import { useOrders } from "@/hooks/useOrders";
import { useAuth } from "@/state/auth";

// Bandeja unificada de órdenes (RF-10). Punto de partida del módulo Órdenes.
// TODO (RF-10): filtros por bodega, fecha y courier; exportar a Excel; paginación.
function OrdersPage() {
  const params = useSearchParams();
  const router = useRouter();
  const { can } = useAuth();
  const status = params.get("status") ?? "";
  const [q, setQ] = useState("");
  const [onlyPriority, setOnlyPriority] = useState(false);
  const [creating, setCreating] = useState(false);
  const { orders, total, loading, error, reload, live } = useOrders({ status, q, priority: onlyPriority });

  return (
    <>
      <PageHeader
        title="Órdenes"
        subtitle={`${total} orden(es)`}
        actions={
          <>
            <LiveBadge live={live} />
            {can({ roles: ["admin", "jefe_operaciones", "cliente"] }) && (
              <button className="btn primary" onClick={() => setCreating(true)}>
                <Plus size={16} /> Nueva orden
              </button>
            )}
          </>
        }
      />
      <div className="filters card">
        <label className="search">
          <Search size={16} />
          <input placeholder="Buscar por referencia o N°" value={q} onChange={(e) => setQ(e.target.value)} />
        </label>
        <select value={status} onChange={(e) => router.replace(e.target.value ? `/ordenes?status=${e.target.value}` : "/ordenes")}>
          <option value="">Todos los estados</option>
          {STATUS_ORDER.map((s) => (
            <option key={s} value={s}>
              {STATUS[s].label}
            </option>
          ))}
        </select>
        <label className="check">
          <input type="checkbox" checked={onlyPriority} onChange={(e) => setOnlyPriority(e.target.checked)} /> Solo prioritarias
        </label>
      </div>
      {error && <p className="error">{error}</p>}
      <section className="card flush">
        <OrdersTable orders={orders} loading={loading} onChanged={reload} />
      </section>
      {creating && <CreateOrderDialog onClose={() => setCreating(false)} onCreated={reload} />}
    </>
  );
}

export default function Page() {
  return (
    <Suspense>
      <OrdersPage />
    </Suspense>
  );
}
