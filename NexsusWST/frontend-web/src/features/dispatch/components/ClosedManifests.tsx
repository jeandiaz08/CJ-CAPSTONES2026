"use client";

import { useOrderResource } from "@/features/orders";
import { listManifests, type ManifestLine } from "@/services/dispatch";
import { transportCompany } from "../manifest";
import { ManifestAccordions, type LineInfo } from "./ManifestAccordions";

const dateTime = (iso?: string) => (iso ? new Date(iso).toLocaleString("es-CL", { dateStyle: "short", timeStyle: "short" }) : "—");

// Manifiestos cerrados, en lectura.
export function ClosedManifests({ courierName, info, productName }: {
  courierName: (code: string) => string;
  info: (line: ManifestLine) => LineInfo;
  productName: (companyId: number, sku: string) => string;
}) {
  // TODO(api): el servidor no filtra por empresa ni pagina; trae los últimos 50.
  const closed = useOrderResource(() => listManifests("closed"), "dispatch-closed");
  if (closed.error) return <p role="alert">No se pudieron cargar los manifiestos cerrados: {closed.error}</p>;
  if (!closed.data) return <div className="detail-skeleton dispatch-skeleton" aria-label="Cargando manifiestos" />;
  if (closed.data.data.length === 0) return <p className="pad">Aún no hay manifiestos cerrados.</p>;
  return (
    <div className="closed-manifests">
      {closed.data.data.map((m) => {
        const delivered = m.lines.filter((l) => l.status === "delivered");
        const notes = m.notes?.split("\n").filter((l) => l !== `Empresa del transporte: ${transportCompany(m.notes)}`).join("\n");
        return (
          <details key={m.id} className="dispatch-panel closed-manifest">
            <summary>
              <strong>Manifiesto #{m.id} · {courierName(m.courier)}</strong>
              <span>Cerrado {dateTime(m.closedAt)} · {delivered.length} de {m.lines.length} {m.lines.length === 1 ? "orden entregada" : "órdenes entregadas"}</span>
            </summary>
            <dl className="manifest-facts">
              <div><dt>Retira</dt><dd>{m.pickerName || "—"}{m.pickerId ? ` (${m.pickerId})` : ""}</dd></div>
              <div><dt>Patente</dt><dd>{m.vehiclePlate || "—"}</dd></div>
              <div><dt>Empresa del transporte</dt><dd>{transportCompany(m.notes) || "—"}</dd></div>
              <div><dt>Cerró</dt><dd>{m.closedBy || "—"}</dd></div>
            </dl>
            {notes && <p className="closed-notes">{notes}</p>}
            <ManifestAccordions lines={m.lines} info={info} productName={productName} open={false} />
          </details>
        );
      })}
    </div>
  );
}
