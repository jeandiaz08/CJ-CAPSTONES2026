"use client";

import type { ReactNode } from "react";
import { Badge } from "@/components/StatusBadge";
import type { Profile } from "@/services/clients";
import { dateLabel } from "@/features/orders/components/OrderDetail";

// Panel «Detalle de perfil {nombre}». Rol siempre Cliente, sin permisos internos. Los botones de
// cambio aparecen solo con «Gestionar perfiles», y Activar o Desactivar según el estado.
// `actions` reemplaza los botones de Administración (por ejemplo, las solicitudes del Cliente).
export function ProfileDetail({ profile, canManage, inModal = false, actions, onEdit, onToggle, onPassword }: {
  profile: Profile;
  canManage: boolean;
  inModal?: boolean;
  actions?: ReactNode;
  onEdit: () => void;
  onToggle: () => void;
  onPassword: () => void;
}) {
  const facts: [string, string][] = [
    ["Nombre completo", profile.name],
    ["Correo", profile.email || "—"],
    ["Teléfono", profile.phone || "—"],
    ["Cargo", profile.jobTitle || "—"],
    ["Rol", profile.role === "cliente" ? "Cliente" : profile.role],
    ["Permisos", profile.permissions.length ? profile.permissions.join(", ") : "Ninguno"],
    ["Empresa", profile.companyName || "—"],
    ["Fecha de creación", dateLabel(profile.createdAt)],
    ...(profile.lastLoginAt ? [["Último acceso", dateLabel(profile.lastLoginAt)] as [string, string]] : []),
  ];
  return (
    <div className="order-detail company-detail">
      {!inModal && <h2>Detalle de perfil {profile.name}</h2>}
      <div className="order-detail-content">
        <div className="detail-status-row"><Badge tone={profile.active ? "ok" : "neutral"} label={profile.active ? "Activo" : "Inactivo"} /></div>
        <section>
          <dl className="registry-facts">
            {facts.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}
          </dl>
        </section>
      </div>
      {actions && <div className="detail-actions">{actions}</div>}
      {!actions && canManage && (
        <div className="detail-actions">
          {profile.active
            ? <button type="button" className="btn danger" onClick={onToggle}>Desactivar</button>
            : <button type="button" className="btn success" onClick={onToggle}>Activar</button>}
          <div>
            <button type="button" className="btn ghost" onClick={onEdit}>Editar</button>
            <button type="button" className="btn ghost" disabled={!profile.active || !profile.linked || !profile.hasEmail}
              title={!profile.active ? "Activa el perfil antes de restablecer su contraseña" : !profile.linked ? "El perfil aún no tiene cuenta de acceso" : !profile.hasEmail ? "El perfil no tiene correo" : undefined}
              onClick={onPassword}>Restablecer contraseña</button>
          </div>
        </div>
      )}
    </div>
  );
}
