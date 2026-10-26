"use client";

import { PageHeader } from "@/components/PageHeader";
import { LiveBadge } from "@/components/StatusBadge";
import { OrdersTable } from "@/features/orders/OrdersTable";
import { useOrders } from "@/hooks/useOrders";

// Intervención del Jefe de Operaciones (RF-15, RF-20; CU-03).
// Incidencia: al resolver, la orden vuelve al estado donde ocurrió.
// Anulada (sin stock): se puede revalidar o cancelar.
export default function IncidentsPage() {
  const inc = useOrders({ status: "incident", limit: 200 });
  const voided = useOrders({ status: "voided", limit: 200 });
  return (
    <>
      <PageHeader title="Incidencias" subtitle="Órdenes detenidas que requieren intervención" actions={<LiveBadge live={inc.live} />} />
      <h2 className="section-title">Con incidencia ({inc.orders.length})</h2>
      <section className="card flush">
        <OrdersTable orders={inc.orders} loading={inc.loading} onChanged={inc.reload} empty="Sin incidencias abiertas." />
      </section>
      <h2 className="section-title">Anuladas por falta de stock ({voided.orders.length})</h2>
      <section className="card flush">
        <OrdersTable orders={voided.orders} loading={voided.loading} onChanged={voided.reload} empty="Sin órdenes anuladas." />
      </section>
    </>
  );
}
