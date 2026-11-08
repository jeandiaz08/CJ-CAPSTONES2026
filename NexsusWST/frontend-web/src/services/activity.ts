import { api, downloadFile } from "./api";
import type { RegistryActor } from "./registry";

// Registro de acciones (Tarea 06, RF-28): GET /v1/activity. Solo Administrador y Jefe de Operaciones.
export type ActivitySection = "orders" | "packing" | "dispatch" | "users";

export type ActivityItem = {
  id: string;
  occurredAt: string;
  actor: RegistryActor; // el perfil es el ACTUAL del usuario
  module: ActivitySection;
  action: string;
  actionLabel: string;
  entity: { type: "order" | "manifest" | "user"; id: string; label: string };
  companyId?: number;
  companyName?: string;
  reason?: string;
  details: Record<string, unknown>;
  statusChange?: string;
  courierName?: string;
  transportCompany?: string;
  observation?: string;
  detailLines: string[]; // el antes y el después completo
};

export type ActivityFilters = {
  module: ActivitySection;
  q?: string;
  from?: string;
  to?: string;
  actor?: string;
  role?: string;
  kind?: string;
  limit?: number;
  offset?: number;
};

function activityParams(f: ActivityFilters) {
  const p = new URLSearchParams({ module: f.module });
  for (const key of ["q", "from", "to", "actor", "role", "kind"] as const) if (f[key]) p.set(key, f[key]!);
  return p;
}

export function listActivity(f: ActivityFilters) {
  const p = activityParams(f);
  p.set("limit", String(f.limit ?? 50));
  p.set("offset", String(f.offset ?? 0));
  return api<{ items: ActivityItem[]; total: number }>("orders", `/v1/activity?${p}`);
}

// Excel con lo filtrado del apartado actual, con sus columnas.
export const exportActivity = (f: ActivityFilters) =>
  downloadFile(`/v1/activity/export?${activityParams(f)}`, `registro-acciones-${f.module}.xlsx`);
