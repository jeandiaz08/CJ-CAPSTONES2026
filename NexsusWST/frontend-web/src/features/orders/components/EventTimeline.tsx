import type { OrderEvent } from "@/services/orders";
import { actorName } from "../format";
import { dateLabel } from "./OrderDetail";

// Eventos que no cambian el estado: se muestran con su nombre.
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
const CLOSED = new Set(["incident", "voided", "cancelled"]);

// Línea de tiempo de una orden: los cambios de estado con punto lleno, el resto con punto vacío.
export function EventTimeline({ events, labels }: { events: OrderEvent[]; labels: Record<string, string> }) {
  if (!events.length) return <p className="muted">Sin eventos registrados</p>;
  // Por fecha y no por id: un evento cargado después (seed, migración) queda en su lugar.
  const sorted = [...events].sort((a, b) => Date.parse(a.occurredAt) - Date.parse(b.occurredAt) || a.id - b.id);
  return (
    <ol className="event-timeline">
      {sorted.map((e) => {
        const status = e.type === "status_change" || e.type === "created";
        const title = e.type === "created" ? `Creada · ${labels[e.toStatus] ?? e.toStatus}` : status ? (labels[e.toStatus] ?? e.toStatus) : (EVENT_LABELS[e.event] ?? e.event);
        return (
          <li key={e.id} className={`${status ? "is-status" : "is-event"}${status && CLOSED.has(e.toStatus) ? " is-danger" : ""}`}>
            <strong>{title}</strong>
            <small>{actorName(e.actor)} · {dateLabel(e.occurredAt)}</small>
            {e.reason && <small className="event-reason">{e.reason}</small>}
          </li>
        );
      })}
    </ol>
  );
}
