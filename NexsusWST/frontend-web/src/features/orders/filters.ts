import type { OrderFilters } from "@/services/orders";

// Filtros de «Todas las órdenes»: viven en la URL para poder compartir y recargar la vista.
export type FilterValues = {
  companyId: string;
  origin: string;
  courier: string;
  statuses: string[];
  situation: string;
  from: string;
  to: string;
  onlyActive: boolean;
};

export const EMPTY_FILTERS: FilterValues = { companyId: "", origin: "", courier: "", statuses: [], situation: "", from: "", to: "", onlyActive: false };

// Solo Listado usa Origen y «Solo en curso»; solo Asignación usa Situación.
export function readFilters(params: URLSearchParams, assignment: boolean): FilterValues {
  return {
    companyId: params.get("company_id") ?? "",
    origin: assignment ? "" : params.get("origin") ?? "",
    courier: params.get("courier") ?? "",
    statuses: params.get("status")?.split(",").filter(Boolean) ?? [],
    situation: assignment ? params.get("situation") ?? "" : "",
    from: params.get("from") ?? "",
    to: params.get("to") ?? "",
    onlyActive: !assignment && params.get("active") === "1",
  };
}

export function writeFilters(params: URLSearchParams, values: FilterValues) {
  const entries: [string, string][] = [
    ["company_id", values.companyId],
    ["origin", values.origin],
    ["courier", values.courier],
    ["status", values.statuses.join(",")],
    ["situation", values.situation],
    ["from", values.from],
    ["to", values.to],
    ["active", values.onlyActive ? "1" : ""],
  ];
  for (const [key, value] of entries) {
    if (value) params.set(key, value);
    else params.delete(key);
  }
}

export function countFilters(values: FilterValues) {
  return [values.companyId, values.origin, values.courier, values.statuses.length > 0, values.situation, values.from || values.to, values.onlyActive].filter(Boolean).length;
}

const IN_PROGRESS = ["pending", "validating", "validated", "picking", "picked", "packing", "ready_pickup", "incident", "voided"];

export function toOrderFilters(values: FilterValues): OrderFilters {
  const manualOrigin = values.origin === "manual" || values.origin === "bulk";
  // «Solo en curso» oculta las Entregadas: no hay parámetro propio, se resuelve con los estados.
  const kept = values.statuses.filter((status) => status !== "handed_over");
  const statuses = values.onlyActive ? (kept.length ? kept : IN_PROGRESS) : values.statuses;
  return {
    ...(values.companyId ? { companyId: Number(values.companyId) } : {}),
    ...(values.origin ? (manualOrigin ? { source: values.origin } : { source: "webhook", channel: values.origin }) : {}),
    ...(values.courier ? { courier: values.courier } : {}),
    ...(statuses.length ? { status: statuses.join(",") } : {}),
    ...(values.situation ? { situation: values.situation } : {}),
    ...(values.from ? { from: values.from } : {}),
    ...(values.to ? { to: values.to } : {}),
  };
}
