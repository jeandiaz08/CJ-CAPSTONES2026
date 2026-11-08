import Link from "next/link";
import type { ReactNode } from "react";
import type { ActivityItem, ActivitySection } from "@/services/activity";
import { ROLE_LABELS, type Role } from "@/state/auth";
import { dateLabel } from "@/features/orders/components/OrderDetail";

// Columnas de cada apartado del registro de acciones (Tarea 06, Fase 3). Cada una con su ancho
// mínimo: la tabla se desplaza entera en horizontal y las celdas largas van con elipsis.
export type ActivityColumn = { label: string; width: number; text: (it: ActivityItem) => string; cell?: (it: ActivityItem) => ReactNode };

export const SECTIONS: { value: ActivitySection; label: string }[] = [
  { value: "orders", label: "Órdenes" },
  { value: "packing", label: "Empaque" },
  { value: "dispatch", label: "Retiro y entrega" },
  { value: "users", label: "Usuarios" },
];

const value = (v: unknown) => (v === undefined || v === null || v === "" ? "—" : Array.isArray(v) ? v.join(", ") : String(v));
const roleLabel = (role?: string) => (role ? ROLE_LABELS[role as Role] ?? role : "—");

// La orden abre el detalle de Todas las órdenes (o su registro, si ya está en el histórico).
export const orderLink = (it: ActivityItem) => <Link className="activity-link" href={`/ordenes/${it.entity.id}`} onClick={(e) => e.stopPropagation()}>{it.entity.label}</Link>;
export const manifestNumber = (it: ActivityItem) => (it.entity.type === "manifest" ? it.entity.id : it.details.manifestId ? String(it.details.manifestId) : "");
const manifestLink = (it: ActivityItem) => {
  const id = manifestNumber(it);
  return id ? <Link className="activity-link" href={`/ordenes/registro?tab=acciones&section=dispatch&q=%23${id}`} onClick={(e) => e.stopPropagation()}>#{id}</Link> : "—";
};

const when: ActivityColumn = { label: "Fecha y hora", width: 150, text: (it) => dateLabel(it.occurredAt) };
const actor: ActivityColumn = { label: "Actor", width: 180, text: (it) => it.actor.name };
const profile: ActivityColumn = { label: "Perfil", width: 160, text: (it) => roleLabel(it.actor.role) };
const action: ActivityColumn = { label: "Acción", width: 200, text: (it) => it.actionLabel };
const order: ActivityColumn = { label: "N° orden", width: 140, text: (it) => it.entity.label, cell: (it) => (it.entity.type === "order" ? orderLink(it) : "—") };
const client: ActivityColumn = { label: "Cliente", width: 180, text: (it) => value(it.companyName) };
const detail: ActivityColumn = { label: "Detalle", width: 280, text: (it) => value(it.detailLines.join(" · ")) };

export const COLUMNS: Record<ActivitySection, ActivityColumn[]> = {
  orders: [when, actor, profile, { label: "Humano/Sistema", width: 130, text: (it) => (it.actor.kind === "system" ? "Sistema" : "Humano") }, action, order, client,
    { label: "Estado anterior → nuevo", width: 230, text: (it) => value(it.statusChange) },
    { label: "Motivo u observación", width: 260, text: (it) => value(it.reason) }, detail],
  packing: [when, actor, profile, action, order, client,
    { label: "Bultos", width: 80, text: (it) => value(it.details.packages) },
    { label: "Código de etiqueta", width: 160, text: (it) => value(it.details.labelCode) },
    { label: "Observación", width: 260, text: (it) => value(it.reason) }],
  dispatch: [when, actor, profile, action,
    { label: "N° manifiesto", width: 130, text: (it) => (manifestNumber(it) ? `#${manifestNumber(it)}` : "—"), cell: manifestLink },
    { label: "Courier", width: 150, text: (it) => value(it.courierName) },
    { label: "Órdenes", width: 90, text: (it) => value(it.details.orders) },
    { label: "Bultos", width: 80, text: (it) => value(it.details.packages ?? it.details.packagesTotal) },
    { label: "Quien retira", width: 170, text: (it) => value(it.details.pickerName) },
    { label: "RUT", width: 130, text: (it) => value(it.details.pickerId) },
    { label: "Patente", width: 110, text: (it) => value(it.details.vehiclePlate) },
    { label: "Empresa del transporte", width: 190, text: (it) => value(it.transportCompany) },
    { label: "Observación", width: 260, text: (it) => value(it.observation || it.reason) }],
  users: [when, actor, profile, action,
    { label: "Usuario afectado", width: 190, text: (it) => it.entity.label },
    { label: "Perfil afectado", width: 170, text: (it) => roleLabel(it.details.targetRole as string | undefined) }, { ...detail, width: 340 }],
};
