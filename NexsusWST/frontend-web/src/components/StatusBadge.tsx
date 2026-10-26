import type { OrderStatus } from "@/services/orders";
import { STATUS } from "@/features/orders/statuses";

export function StatusBadge({ status }: { status: OrderStatus }) {
  const s = STATUS[status];
  return <span className={`badge tone-${s?.tone ?? "neutral"}`}>{s?.label ?? status}</span>;
}

export function LiveBadge({ live }: { live: boolean }) {
  return (
    <span className={`badge ${live ? "tone-ok" : "tone-danger"}`} title="Actualización en vivo (RF-26)">
      {live ? "En vivo" : "Sin conexión"}
    </span>
  );
}
