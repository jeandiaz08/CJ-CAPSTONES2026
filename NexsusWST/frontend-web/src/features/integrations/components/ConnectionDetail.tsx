"use client";

import { useState } from "react";
import { Copy, RefreshCw } from "lucide-react";
import { Badge } from "@/components/StatusBadge";
import type { Integration } from "@/services/integrations";
import { dateLabel } from "@/features/orders/components/OrderDetail";
import { KIND_LABELS, providerName } from "../providers";

// «user:12 Ana Pérez» → «Ana Pérez».
export const actorName = (actor: string) => actor.replace(/^user:\d+\s*/, "") || actor;

// Traefik publica /webhooks en el mismo host que la aplicación (docker-compose.yml, router
// orders-webhooks), así que la URL completa es el origen de esta página más la ruta del backend.
export const webhookUrl = (path: string) => (typeof window === "undefined" ? path : window.location.origin + path);

// Panel de una conexión. El secreto nunca llega del backend en la lista: siempre va enmascarado.
// Activar o Desactivar: solo el que corresponde al estado.
export function ConnectionDetail({ connection, companyName, inModal = false, onToggle, onRotate }: {
  connection: Integration;
  companyName: string;
  inModal?: boolean;
  onToggle: () => void;
  onRotate: () => void;
}) {
  const [copied, setCopied] = useState(false);
  const url = webhookUrl(connection.webhookPath);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };
  const facts: [string, string][] = [
    ["Empresa", companyName],
    ["Tipo", KIND_LABELS[connection.kind] ?? connection.kind],
    ["Proveedor", providerName(connection.provider, connection.kind)],
    ...(connection.kind === "channel" ? [["Canal", connection.channel], ["Courier por defecto", connection.defaultCourier || "—"]] as [string, string][] : []),
    ["Creada", `${dateLabel(connection.createdAt)} · ${actorName(connection.createdBy)}`],
  ];
  return (
    <div className="order-detail company-detail connection-detail">
      {!inModal && <h2>{companyName} · {providerName(connection.provider, connection.kind)}</h2>}
      <div className="order-detail-content">
        <div className="detail-status-row"><Badge tone={connection.active ? "ok" : "neutral"} label={connection.active ? "Activa" : "Inactiva"} /></div>
        <section>
          <dl className="registry-facts">
            {facts.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}
          </dl>
        </section>
        <section>
          <h3>URL del webhook</h3>
          <p className="secret-row"><code>{url}</code>
            <button type="button" className="text-button" onClick={copy}><Copy size={16} aria-hidden="true" />{copied ? "Copiada" : "Copiar"}</button>
          </p>
        </section>
        <section>
          <h3>Secreto</h3>
          <p className="secret-row"><code aria-label="Secreto oculto">••••••••••••</code>
            <button type="button" className="text-button" onClick={onRotate}><RefreshCw size={16} aria-hidden="true" />Rotar</button>
          </p>
          <small className="muted">Al rotar, el secreto nuevo se muestra una sola vez.</small>
        </section>
        <section>
          <h3>Últimos eventos</h3>
          {/* TODO(api): falta un endpoint con los últimos eventos de la conexión (webhook_inbox). */}
          <p className="muted">Aún no disponible.</p>
        </section>
      </div>
      <div className="detail-actions">
        {connection.active
          ? <button type="button" className="btn danger" onClick={onToggle}>Desactivar</button>
          : <button type="button" className="btn success" onClick={onToggle}>Activar</button>}
      </div>
    </div>
  );
}
