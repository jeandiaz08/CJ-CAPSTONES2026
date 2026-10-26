"use client";

import { PageHeader } from "@/components/PageHeader";
import { LiveBadge } from "@/components/StatusBadge";
import { OrdersTable } from "@/features/orders/OrdersTable";
import { useOrders } from "@/hooks/useOrders";

// Bandeja de empaque (RF-18, RF-19, RF-21; CU-04). El personal toma las
// órdenes Preparadas, verifica y las deja Listas para retiro.
// TODO (RF-21/22): verificación por producto/cantidad, discrepancias y bultos.
export default function PackingPage() {
  const prep = useOrders({ status: "picking" });
  const picked = useOrders({ status: "picked" });
  const packing = useOrders({ status: "packing" });
  return (
    <>
      <PageHeader title="Empaque" subtitle="Preparación (Invas) → empaque → lista para retiro" actions={<LiveBadge live={picked.live} />} />
      <h2 className="section-title">Preparadas — por tomar ({picked.orders.length})</h2>
      <section className="card flush">
        <OrdersTable orders={picked.orders} loading={picked.loading} onChanged={picked.reload} empty="No hay órdenes preparadas." />
      </section>
      <h2 className="section-title">En empaque ({packing.orders.length})</h2>
      <section className="card flush">
        <OrdersTable orders={packing.orders} loading={packing.loading} onChanged={packing.reload} empty="Nadie está empacando." />
      </section>
      <h2 className="section-title">En preparación en Invas ({prep.orders.length})</h2>
      <section className="card flush">
        <OrdersTable orders={prep.orders} loading={prep.loading} onChanged={prep.reload} empty="Sin órdenes en preparación." />
      </section>
    </>
  );
}
