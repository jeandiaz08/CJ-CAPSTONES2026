import { CompactTable } from "@/components/CompactTable";
import type { OrderTracking } from "@/services/orders";

const dateTime = (iso: string) => new Date(iso).toLocaleString("es-CL", { dateStyle: "short", timeStyle: "short" });

// Seguimiento posterior al retiro (RF-25): solo lo que entrega el courier, sin inventar estados.
export function TrackingSection({ tracking }: { tracking?: OrderTracking }) {
  const events = tracking?.available ? tracking.events : [];
  return (
    <section>
      <h3>Seguimiento</h3>
      {events.length === 0 ? (
        <p className="muted">Sin seguimiento disponible</p>
      ) : (
        <CompactTable label="Seguimiento del courier" columns={[{ label: "Estado" }, { label: "Origen" }, { label: "Fecha" }]}
          rows={events.map((e) => ({ key: e.id, cells: [e.description ? `${e.status} · ${e.description}` : e.status, e.provider, dateTime(e.occurredAt)] }))} />
      )}
    </section>
  );
}
