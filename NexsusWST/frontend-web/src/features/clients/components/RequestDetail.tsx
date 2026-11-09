"use client";

import { Badge, type BadgeTone } from "@/components/StatusBadge";
import type { ProfileRequest, RequestStatus } from "@/services/clients";
import { dateLabel } from "@/features/orders/components/OrderDetail";

export const STATUS_LABELS: Record<RequestStatus, string> = { pending: "Pendiente", approved: "Aprobada", rejected: "Rechazada" };
export const STATUS_TONES: Record<RequestStatus, BadgeTone> = { pending: "warn", approved: "ok", rejected: "danger" };

// Panel «Detalle de solicitud»: datos del perfil solicitado, tipo, motivo, estado y fecha; si ya fue
// respondida, quién la revisó, cuándo y el motivo del rechazo. Solo lo que guarda la solicitud (sin rol
// ni permisos: un perfil de cliente siempre es Cliente). Las acciones van solo si `actions` viene.
export function RequestDetail({ request, inModal = false, actions }: {
  request: ProfileRequest;
  inModal?: boolean;
  actions?: { onAccept: () => void; onEditAccept?: () => void; onReject: () => void };
}) {
  const facts: [string, string][] = [
    ["Nombre completo", request.profile.name],
    ["Correo", request.profile.email || "—"],
    ["Teléfono", request.profile.phone || "—"],
    ["Cargo", request.profile.jobTitle || "—"],
    ["Empresa", request.companyName],
    ["Tipo", request.kindLabel],
    ["Solicitado por", request.requestedBy.name || "—"],
    ["Fecha de solicitud", dateLabel(request.requestedAt)],
    ["Motivo", request.reason],
    ["Revisado por", request.reviewedBy?.name ?? "—"],
    ["Fecha de respuesta", request.reviewedAt ? dateLabel(request.reviewedAt) : "—"],
    ...(request.status === "rejected" ? [["Motivo del rechazo", request.rejectionReason] as [string, string]] : []),
  ];
  return (
    <div className="order-detail company-detail">
      {!inModal && <h2>Detalle de solicitud</h2>}
      <div className="order-detail-content">
        <div className="detail-status-row"><Badge tone={STATUS_TONES[request.status]} label={STATUS_LABELS[request.status]} /></div>
        <section>
          <dl className="registry-facts">
            {facts.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}
          </dl>
        </section>
      </div>
      {actions && request.status === "pending" && (
        <div className="detail-actions">
          <button type="button" className="btn success" onClick={actions.onAccept}>Aceptar solicitud</button>
          <div>
            {actions.onEditAccept && <button type="button" className="btn ghost" onClick={actions.onEditAccept}>Editar y aceptar</button>}
            <button type="button" className="btn danger" onClick={actions.onReject}>Rechazar</button>
          </div>
        </div>
      )}
    </div>
  );
}
