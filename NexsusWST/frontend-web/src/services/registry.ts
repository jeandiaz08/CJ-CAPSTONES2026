import { api, downloadFile, withCompany } from "./api";
import type { OrderStatus } from "./orders";
import type { Role } from "@/state/auth";

// Registro histórico (Tarea 06): Canceladas y órdenes con más de 1 año sin cambiar de estado.
export type RegistryActor = { id?: number; name: string; role?: Role; kind: "human" | "system" };

export type RegistryEntry = {
  id: number;
  externalRef: string;
  companyId: number;
  companyName: string;
  courier: string;
  courierName: string;
  status: OrderStatus;
  statusLabel: string;
  previousStatus?: OrderStatus;
  previousStatusLabel?: string;
  closedAt: string;
  closedBy: RegistryActor; // el perfil es el ACTUAL del usuario
  reason: string;
  motive: string;
  stale: boolean; // entró por más de 1 año sin movimiento
};

export type RegistryFilters = {
  q?: string;
  status?: string;
  courier?: string;
  from?: string;
  to?: string;
  ids?: number[];
  limit?: number;
  offset?: number;
};

function registryParams(f: RegistryFilters) {
  const p = new URLSearchParams();
  if (f.q) p.set("q", f.q);
  if (f.status) p.set("status", f.status);
  if (f.courier) p.set("courier", f.courier);
  if (f.from) p.set("from", f.from);
  if (f.to) p.set("to", f.to);
  if (f.ids?.length) p.set("ids", f.ids.join(","));
  return p;
}

export function listRegistry(f: RegistryFilters, companyId: number | null) {
  const p = registryParams(f);
  p.set("limit", String(f.limit ?? 20));
  p.set("offset", String(f.offset ?? 0));
  return api<{ data: RegistryEntry[]; total: number }>("orders", withCompany(`/v1/orders/registry?${p}`, companyId));
}

// «Exportar masivo»: Excel con todo lo filtrado en pantalla.
export const exportRegistry = (f: RegistryFilters, companyId: number | null) =>
  downloadFile(withCompany(`/v1/orders/registry/export?${registryParams(f)}`, companyId), "registro-ordenes.xlsx");

// «Exportar seleccionadas» y «Exportar por filtros» (ids, fechas, cliente, courier, estado, format).
export const exportRegistryFile = (params: URLSearchParams, fallbackName: string) =>
  downloadFile(`/v1/orders/registry/export?${params}`, fallbackName);
