import type { OrderStatus } from "@/services/orders";
const tones: Record<OrderStatus, string> = { pending: "neutral", validating: "processing", validated: "info", picking: "processing", picked: "info", packing: "warn", ready_pickup: "ok", handed_over: "ok", incident: "danger", voided: "warn", cancelled: "neutral" };
export function StatusBadge({ status, label }: {
    status: OrderStatus;
    label?: string;
}) { return <span className={`badge tone-${tones[status]}`}>{label ?? status}</span>; }
export function LiveBadge({ live }: {
    live: boolean;
}) { return <span className={`badge ${live ? "tone-ok" : "tone-warn"}`} role="status">● {live ? "En vivo" : "Reconectando…"}</span>; }
