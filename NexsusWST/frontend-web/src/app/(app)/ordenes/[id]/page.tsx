"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { ChevronRight } from "lucide-react";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { OrderActions } from "@/features/orders/OrderActions";
import { STATUS } from "@/features/orders/statuses";
import { useOrdersSocket } from "@/hooks/useOrdersSocket";
import { getOrder, type Order, type OrderEvent, type OrderStatus } from "@/services/orders";
import { useAuth } from "@/state/auth";

// Detalle e historial de una orden (RF-11).
// TODO (RF-11): destino, referencias externas, estado WMS y estado del courier por separado.
export default function OrderDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { companyId } = useAuth();
  const [order, setOrder] = useState<Order | null>(null);
  const [events, setEvents] = useState<OrderEvent[]>([]);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(() => {
    getOrder(Number(id), companyId)
      .then((r) => {
        setOrder(r.order);
        setEvents(r.events);
        setError(null);
      })
      .catch((e) => setError(e.message));
  }, [id, companyId]);

  useEffect(load, [load]);
  useOrdersSocket((e) => {
    if (e.orderId === Number(id) || e.type === "reconnected") load();
  });

  if (error) return <p className="error">{error}</p>;
  if (!order) return <p className="muted pad">Cargando…</p>;

  const label = (s: string) => (s ? (STATUS[s as OrderStatus]?.label ?? s) : "");
  const EVENT_LABELS: Record<string, string> = {
    prioritize: "Priorizada (primero en la cola)",
    unprioritize: "Prioridad quitada",
    assign_wave: "Asignación a ola solicitada",
    edit: "Orden editada",
    register: "Bultos registrados",
    add: "Agregada al manifiesto",
    attach: "Documento adjunto",
    label: "Etiqueta impresa",
    retry: "Trabajo reencolado",
    duplicate: "Reintento del canal (no se duplicó)",
    no_wave: "Regla sin ola disponible",
  };

  return (
    <>
      <p className="crumbs">
        <Link href="/ordenes">Órdenes</Link> <ChevronRight size={14} /> #{order.id}
      </p>
      <PageHeader title={`Orden #${order.id} · ${order.externalRef}`} subtitle={<StatusBadge status={order.status} />} actions={<OrderActions order={order} onDone={load} />} />
      {order.note && <p className="callout">{order.note}</p>}
      <div className="detail-grid">
        <section className="card">
          <h2>Datos</h2>
          <dl className="kv">
            <dt>Empresa</dt>
            <dd>#{order.companyId}</dd>
            <dt>Canal</dt>
            <dd>{order.channel}</dd>
            <dt>Courier</dt>
            <dd>{order.courier || "—"}</dd>
            <dt>Prioridad</dt>
            <dd>{order.priority ? "Sí (primero en la cola)" : "Normal"}</dd>
            <dt>Ola</dt>
            <dd>{order.waveId ?? "Sin ola"}</dd>
            <dt>OS Invas</dt>
            <dd>{order.wmsRef ?? "—"}</dd>
            {order.prevStatus && (
              <>
                <dt>Volverá a</dt>
                <dd>{label(order.prevStatus)}</dd>
              </>
            )}
          </dl>
          <h2>Productos</h2>
          <table>
            <thead>
              <tr>
                <th>SKU</th>
                <th>Cantidad</th>
              </tr>
            </thead>
            <tbody>
              {order.items?.map((it) => (
                <tr key={it.sku}>
                  <td>{it.sku}</td>
                  <td>{it.quantity}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
        <section className="card">
          <h2>Historial</h2>
          <ol className="timeline">
            {events.map((e) => (
              <li key={e.id}>
                <span className="when">{new Date(e.occurredAt).toLocaleString("es-CL")}</span>
                <strong>
                  {e.type === "status_change" ? `${label(e.fromStatus)} → ${label(e.toStatus)}` : e.type === "created" ? "Orden creada" : (EVENT_LABELS[e.event] ?? e.event)}
                </strong>
                <span className="muted small">{e.actor}</span>
                {e.reason && <span className="small">{e.reason}</span>}
              </li>
            ))}
          </ol>
        </section>
      </div>
    </>
  );
}
