"use client";

import Link from "next/link";
import { PageHeader } from "@/components/PageHeader";
import { useOrderResource } from "@/features/orders";
import { getOrder } from "@/services/orders";
import { useAuth } from "@/state/auth";

// Orden tomada. La verificación de productos, los bultos y la etiqueta llegan en la Fase 2.
export function PackingOrder({ id }: { id: number }) {
  const { companyId } = useAuth();
  const resource = useOrderResource(() => getOrder(id, companyId), `packing-order:${id}:${companyId}`);
  const order = resource.data?.order;
  return (
    <div className="orders-view packing-view">
      <PageHeader title={order ? `Orden #${order.externalRef} · En empaque` : "En empaque"} />
      {resource.error && <p role="alert">No se pudo cargar la orden: {resource.error}</p>}
      {order && <p className="form-notice">La verificación de productos, los bultos y la etiqueta se agregan en la siguiente fase.</p>}
      <p><Link className="text-button" href="/ordenes/empaque?tab=mine">Volver a Mi empaque</Link></p>
    </div>
  );
}
