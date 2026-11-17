"use client";

import Link from "next/link";
import { Badge } from "@/components/StatusBadge";
import type { AdminCompany } from "@/services/clients";
import { dateLabel } from "@/features/orders/components/OrderDetail";
import { waveLabel } from "../waves";

// Panel «Detalle de empresa {nombre}» (el mockup decía «Detalle de perfil»). Solo muestra lo que se
// guarda. Los botones de cambio aparecen solo con «Gestionar perfiles», y Activar o Desactivar
// según el estado (nunca los dos).
export function CompanyDetail({ company, canManage, showWms, inModal = false, onEdit, onToggle }: {
  company: AdminCompany;
  canManage: boolean;
  showWms: boolean;
  inModal?: boolean;
  onEdit: () => void;
  onToggle: () => void;
}) {
  const facts: [string, string][] = [
    ["Nombre empresa", company.name],
    ["RUT", company.taxId],
    ["Correo", company.email || "—"],
    ["Teléfono", company.phone || "—"],
    ["Ola", waveLabel(company.waveCode)],
    ["N° de perfiles", String(company.profilesCount)],
    ["Fecha de creación", dateLabel(company.createdAt)],
    ...(showWms ? [["Código Invas", company.wmsCode || "—"] as [string, string]] : []),
  ];
  return (
    <div className="order-detail company-detail">
      {!inModal && <h2>Detalle de empresa {company.name}</h2>}
      <div className="order-detail-content">
        <div className="detail-status-row"><Badge tone={company.active ? "ok" : "neutral"} label={company.active ? "Activa" : "Inactiva"} /></div>
        <section>
          <dl className="registry-facts">
            {facts.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}
          </dl>
        </section>
      </div>
      <div className="detail-actions">
        <Link className="btn order-primary" href={`/integraciones?empresa=${company.id}`}>Ver integraciones</Link>
        {canManage && (
          <div>
            <button type="button" className="btn ghost" onClick={onEdit}>Editar</button>
            {company.active
              ? <button type="button" className="btn danger" onClick={onToggle}>Desactivar</button>
              : <button type="button" className="btn success" onClick={onToggle}>Activar</button>}
          </div>
        )}
      </div>
    </div>
  );
}
