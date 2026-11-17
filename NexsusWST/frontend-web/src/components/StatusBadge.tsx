import type { OrderStatus } from "@/services/orders";
const tones: Record<OrderStatus, string> = { pending: "neutral", validating: "processing", validated: "info", picking: "processing", picked: "info", packing: "warn", ready_pickup: "ok", handed_over: "ok", incident: "danger", voided: "danger", cancelled: "danger" };
export function StatusBadge({ status, label }: {
    status: OrderStatus;
    label?: string;
}) { return <span className={`badge tone-${tones[status]}`}>{label ?? status}</span>; }
export function LiveBadge({ live }: {
    live: boolean;
}) { return <span className={`badge ${live ? "tone-ok" : "tone-warn"}`} role="status">● {live ? "En vivo" : "Reconectando…"}</span>; }
// Insignia con tono explícito, para estados que no son de órdenes (empresas, perfiles, solicitudes).
export type BadgeTone = "neutral" | "processing" | "info" | "warn" | "ok" | "danger";
export function Badge({ tone, label }: {
    tone: BadgeTone;
    label: string;
}) { return <span className={`badge tone-${tone}`}>{label}</span>; }
