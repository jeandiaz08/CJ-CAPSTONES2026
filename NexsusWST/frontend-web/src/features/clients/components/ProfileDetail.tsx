"use client";

import type { ReactNode } from "react";
import { Badge } from "@/components/StatusBadge";
import type { Profile, ProfileScope } from "@/services/clients";
import { dateLabel } from "@/features/orders/components/OrderDetail";
import { PERMISSION_LABELS, PROFILE_ROLE_LABELS } from "../access";

// Panel «Detalle de perfil {nombre}». Cliente: empresa, rol Cliente y sin permisos. WeStorage
// (scope «internal»): correo o usuario, rol y permisos. Los botones de cambio aparecen solo con
// «Gestionar perfiles», y Activar o Desactivar según el estado. `actions` reemplaza los botones de
// Administración (por ejemplo, las solicitudes del Cliente).
export function ProfileDetail({ profile, scope = "client", canManage, inModal = false, actions, onEdit, onToggle, onPassword }: {
  profile: Profile;
  scope?: ProfileScope;
  canManage: boolean;
  inModal?: boolean;
  actions?: ReactNode;
  onEdit: () => void;
  onToggle: () => void;
  onPassword: () => void;
}) {
  const internal = scope === "internal";
  const perms = profile.permissions.map((p) => PERMISSION_LABELS[p] ?? p);
  const facts: [string, string][] = [
    ["Nombre completo", profile.name],
    profile.hasEmail || !internal ? ["Correo", profile.email || "—"] : ["Usuario", profile.username || "—"],
    ["Teléfono", profile.phone || "—"],
    ["Cargo", profile.jobTitle || "—"],
    ["Rol", PROFILE_ROLE_LABELS[profile.role] ?? profile.role],
    ["Permisos", perms.length ? perms.join(", ") : "Ninguno"],
    ...(internal ? [] : [["Empresa", profile.companyName || "—"] as [string, string]]),
    // TODO(PV-21): «Bodegas» cuando exista la tabla de bodegas del usuario.
    ["Fecha de creación", dateLabel(profile.createdAt)],
    ...(profile.lastLoginAt ? [["Último acceso", dateLabel(profile.lastLoginAt)] as [string, string]] : []),
  ];
  // Restablecer: con correo, enlace al correo; un perfil interno sin correo recibe una contraseña temporal.
  const passwordBlock = !profile.active ? "Activa el perfil antes de restablecer su contraseña"
    : !profile.linked ? "El perfil aún no tiene cuenta de acceso"
    : !profile.hasEmail && !internal ? "El perfil no tiene correo" : undefined;
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
            <button type="button" className="btn ghost" disabled={!!passwordBlock} title={passwordBlock} onClick={onPassword}>Restablecer contraseña</button>
          </div>
        </div>
      )}
    </div>
  );
}
