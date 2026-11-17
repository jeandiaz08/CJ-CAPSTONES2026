"use client";

import { StatusBadge } from "@/components/StatusBadge";
import { getOrder } from "@/services/orders";
import type { RegistryEntry } from "@/services/registry";
import { ROLE_LABELS } from "@/state/auth";
import { EventTimeline } from "@/features/orders/components/EventTimeline";
import { dateLabel } from "@/features/orders/components/OrderDetail";
import { useOrderResource } from "@/features/orders/hooks/useOrderResources";

export const closedByLabel = (entry: RegistryEntry) => {
  const role = entry.closedBy.role ? ROLE_LABELS[entry.closedBy.role] : undefined;
  return role ? `${entry.closedBy.name} · ${role}` : entry.closedBy.name;
};

// Panel «Registro orden #N»: quién la cerró, por qué, dónde estaba y su historial de eventos.
export function RegistryDetail({ entry, labels, revision, inModal = false }: {
  entry: RegistryEntry;
  labels: Record<string, string>;
  revision: number;
  inModal?: boolean;
}) {
  const history = useOrderResource(() => getOrder(entry.id, null), `${entry.id}:${revision}`, String(entry.id));
  const facts: [string, string][] = [
    ["Cliente", entry.companyName || `Empresa ${entry.companyId}`],
    ["Cerrada por", closedByLabel(entry)],
    ["Fecha", dateLabel(entry.closedAt)],
    ["Motivo", entry.motive || "—"],
    ["Estaba en", entry.previousStatusLabel || "—"],
    ["Courier", entry.courierName || entry.courier || "—"],
  ];
  return (
    <div className="order-detail registry-detail">
      {!inModal && <h2>Registro orden #{entry.externalRef}</h2>}
      <div className="order-detail-content">
        <div className="detail-status-row"><StatusBadge status={entry.status} label={entry.statusLabel} /></div>
        <section>
          <dl className="registry-facts">
            {facts.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}
          </dl>
        </section>
        <section>
          <h3>Historial de eventos</h3>
          {history.loading && !history.data ? <div className="detail-skeleton registry-history-skeleton" aria-label="Cargando historial" />
            : history.error && !history.data ? <p role="alert">No se pudo cargar el historial: {history.error}</p>
            : <EventTimeline events={history.data?.events ?? []} labels={labels} />}
        </section>
      </div>
    </div>
  );
}
