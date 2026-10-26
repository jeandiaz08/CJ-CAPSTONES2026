// Estados de la orden según el DAS v1.0 (PA-04) y el ERS v1.0 §3.2.7.
// La máquina de estados REAL está en Go (orders-service/internal/domain/status.go);
// aquí solo hay etiquetas, colores y qué botón mostrar.
import type { OrderStatus } from "@/services/orders";
import type { Role } from "@/state/auth";

export type Tone = "neutral" | "info" | "ok" | "warn" | "danger" | "muted";

export const STATUS: Record<OrderStatus, { label: string; tone: Tone }> = {
  pending: { label: "Pendiente", tone: "neutral" },
  validating: { label: "En validación", tone: "info" },
  validated: { label: "Validada", tone: "info" },
  picking: { label: "En preparación", tone: "warn" },
  picked: { label: "Preparada", tone: "warn" },
  packing: { label: "En empaque", tone: "warn" },
  ready_pickup: { label: "Lista para retiro", tone: "ok" },
  handed_over: { label: "Entregada al courier", tone: "ok" },
  incident: { label: "Incidencia", tone: "danger" },
  voided: { label: "Anulada", tone: "danger" },
  cancelled: { label: "Cancelada", tone: "muted" },
};

export const STATUS_ORDER: OrderStatus[] = [
  "pending",
  "validating",
  "validated",
  "picking",
  "picked",
  "packing",
  "ready_pickup",
  "handed_over",
  "incident",
  "voided",
  "cancelled",
];

export type ActionDef = {
  event: string;
  label: string;
  roles?: Role[];
  permission?: string;
  needsReason?: boolean;
  danger?: boolean;
  simulated?: boolean; // lo hará Invas cuando exista la integración
};

const INT: Role[] = ["admin", "jefe_operaciones"];
const ACTIVE: OrderStatus[] = ["validating", "validated", "picking", "picked", "packing", "ready_pickup"];

/** Acciones manuales disponibles por estado (el backend valida de nuevo). */
export function actionsFor(status: OrderStatus): ActionDef[] {
  const a: ActionDef[] = [];
  switch (status) {
    case "picking":
      a.push({ event: "pick_confirmed", label: "Confirmar preparación", roles: INT, simulated: true });
      break;
    case "picked":
      a.push({ event: "start_packing", label: "Tomar para empaque", roles: [...INT, "empaque"] });
      break;
    case "packing":
      a.push({ event: "packed", label: "Empaque verificado", roles: [...INT, "empaque"] });
      break;
    case "ready_pickup":
      a.push({ event: "hand_over", label: "Entregar al courier", permission: "despacho", needsReason: true });
      break;
    case "incident":
      a.push({ event: "resolve", label: "Resolver incidencia", roles: INT, needsReason: true });
      break;
    case "voided":
      a.push({ event: "revalidate", label: "Revalidar", roles: INT, needsReason: true });
      break;
  }
  if (ACTIVE.includes(status)) {
    a.push({ event: "incident", label: "Reportar incidencia", roles: [...INT, "empaque"], needsReason: true, danger: true });
  }
  if (["pending", "validating", "validated", "incident", "voided"].includes(status)) {
    a.push({ event: "cancel", label: "Cancelar", roles: ["admin", "jefe_operaciones", "cliente"], needsReason: true, danger: true });
  }
  return a;
}
