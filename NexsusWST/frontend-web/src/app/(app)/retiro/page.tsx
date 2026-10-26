"use client";

import { PageHeader } from "@/components/PageHeader";
import { LiveBadge } from "@/components/StatusBadge";
import { OrdersTable } from "@/features/orders/OrdersTable";
import { useOrders } from "@/hooks/useOrders";

// Retiro / entrega al courier (RF-24, CU-05). Requiere el permiso Despacho.
// TODO (RF-23/24): agrupar por courier, mostrar el manifiesto y registrar bultos y evidencia.
export default function PickupPage() {
  const { orders, loading, error, reload, live } = useOrders({ status: "ready_pickup", limit: 200 });
  return (
    <>
      <PageHeader title="Retiro courier" subtitle="Órdenes listas para entregar al courier" actions={<LiveBadge live={live} />} />
      {error && <p className="error">{error}</p>}
      <section className="card flush">
        <OrdersTable orders={orders} loading={loading} onChanged={reload} empty="No hay órdenes listas para retiro." />
      </section>
    </>
  );
}
