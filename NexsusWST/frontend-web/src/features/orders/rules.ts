import type { Order } from "@/services/orders";
import type { Role } from "@/state/auth";

// Solo ocultan o deshabilitan en la interfaz: quien decide es el servidor.
const CANCELLABLE = ["pending", "validating", "validated", "incident", "voided"];
const IN_PREPARATION = ["picking", "picked", "packing", "ready_pickup", "handed_over"];
const canOperate = (role: Role) => role === "admin" || role === "jefe_operaciones";

// Devuelve por qué no se puede cancelar, o undefined si se puede.
export function cancelBlockReason(order: Pick<Order, "status" | "prevStatus">, role: Role): string | undefined {
  if (!canOperate(role) && role !== "cliente") return "Sin permiso para cancelar órdenes.";
  if (order.status === "cancelled" || order.status === "handed_over") return "La orden ya está cancelada o entregada.";
  if (!CANCELLABLE.includes(order.status)) return "Ya está en preparación: repórtala como incidencia para cancelarla";
  if (role === "cliente" && order.status === "incident" && IN_PREPARATION.includes(order.prevStatus ?? "")) {
    return "Desde En preparación, el Cliente no puede cancelar la orden.";
  }
  return undefined;
}

export function priorityBlockReason(order: Pick<Order, "status" | "priority">): string | undefined {
  if (order.status === "cancelled" || order.status === "handed_over") return "La orden ya terminó su recorrido.";
  if (order.priority) return "Ya tiene prioridad.";
  return undefined;
}

export function retryBlockReason(order: Pick<Order, "status">): string | undefined {
  return order.status === "voided" ? undefined : "Solo se reintentan órdenes Anuladas.";
}

export { canOperate };
