"use client";

import Link from "next/link";
import { Star } from "lucide-react";
import { StatusBadge } from "@/components/StatusBadge";
import type { Order } from "@/services/orders";
import { OrderActions } from "./OrderActions";

type Props = {
  orders: Order[];
  loading?: boolean;
  onChanged?: () => void;
  selectable?: boolean;
  selected?: Set<number>;
  onToggle?: (id: number) => void;
  showActions?: boolean;
  empty?: string;
};

// Tabla reutilizable de órdenes (bandeja, olas, empaque, retiro, incidencias).
export function OrdersTable({ orders, loading, onChanged, selectable, selected, onToggle, showActions = true, empty }: Props) {
  if (loading) return <p className="muted pad">Cargando…</p>;
  if (orders.length === 0) return <p className="muted pad">{empty ?? "No hay órdenes para mostrar."}</p>;
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            {selectable && <th aria-label="Seleccionar" />}
            <th>N°</th>
            <th>Referencia</th>
            <th>Canal</th>
            <th>Courier</th>
            <th>Estado</th>
            <th>Ola / OS</th>
            <th>Detalle</th>
            {showActions && <th>Acciones</th>}
          </tr>
        </thead>
        <tbody>
          {orders.map((o) => (
            <tr key={o.id} className={o.priority ? "row-priority" : undefined}>
              {selectable && (
                <td>
                  <input type="checkbox" checked={selected?.has(o.id) ?? false} onChange={() => onToggle?.(o.id)} />
                </td>
              )}
              <td>
                <Link href={`/ordenes/${o.id}`} className="link">
                  {o.priority && <Star size={13} fill="currentColor" className="prio-star" />}#{o.id}
                </Link>
              </td>
              <td>{o.externalRef}</td>
              <td>{o.channel}</td>
              <td>{o.courier || <span className="muted">—</span>}</td>
              <td>
                <StatusBadge status={o.status} />
              </td>
              <td className="small">
                {o.waveId ?? <span className="muted">—</span>}
                {o.wmsRef && <div className="muted">{o.wmsRef}</div>}
              </td>
              <td className="small note-cell">{o.note || o.lastError || ""}</td>
              {showActions && (
                <td>
                  <OrderActions order={o} onDone={onChanged} compact />
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
