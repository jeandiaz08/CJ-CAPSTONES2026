// Llamadas al servicio de Órdenes (orders-service, Go).
import { api, withCompany } from "./api";

export type OrderStatus =
  | "pending"
  | "validating"
  | "validated"
  | "picking"
  | "picked"
  | "packing"
  | "ready_pickup"
  | "handed_over"
  | "incident"
  | "voided"
  | "cancelled";

export type OrderItem = { sku: string; quantity: number };

export type Order = {
  id: number;
  companyId: number;
  externalRef: string;
  channel: string;
  source?: "manual" | "bulk" | "webhook";
  courier: string;
  courierCode?: string;
  recipientName?: string;
  recipientPhone?: string;
  recipientEmail?: string;
  addressLine?: string;
  commune?: string;
  regionCode?: string;
  dispatchType?: string;
  customerNotes?: string;
  incidentKind?: string;
  incidentLabel?: string;
  claimedBy?: number;
  claimedByName?: string;
  editable?: boolean;
  status: OrderStatus;
  statusLabel: string;
  prevStatus?: OrderStatus;
  priority: boolean;
  waveId?: string;
  wmsRef?: string;
  note?: string;
  lastError?: string;
  validationAttempts: number;
  version: number;
  items?: OrderItem[];
  createdAt: string;
  updatedAt: string;
};

export type OrderEvent = {
  id: number;
  type: string;
  event: string;
  fromStatus: string;
  toStatus: string;
  actor: string;
  reason: string;
  occurredAt: string;
};

export type Wave = { id: string; name: string; type: string };

export type OrderFilters = {
  status?: string;
  q?: string;
  priority?: boolean;
  withoutWave?: boolean;
  limit?: number;
  offset?: number;
};

export function listOrders(f: OrderFilters, companyId: number | null) {
  const p = new URLSearchParams();
  if (f.status) p.set("status", f.status);
  if (f.q) p.set("q", f.q);
  if (f.priority) p.set("priority", "true");
  if (f.withoutWave) p.set("without_wave", "true");
  p.set("limit", String(f.limit ?? 50));
  p.set("offset", String(f.offset ?? 0));
  return api<{ data: Order[]; total: number }>("orders", withCompany(`/v1/orders?${p}`, companyId));
}

export type Package = { id: number; code: string; weightKg?: number };
export type OrderDocument = { id: number; kind: string; filename: string; contentType: string; sizeBytes: number; uploadedAt: string };

export const getOrder = (id: number, companyId: number | null) =>
  api<{ order: Order; events: OrderEvent[]; packages: Package[]; documents: OrderDocument[] }>(
    "orders",
    withCompany(`/v1/orders/${id}`, companyId),
  );

export const getSummary = (companyId: number | null) =>
  api<{ data: Partial<Record<OrderStatus, number>> }>("orders", withCompany("/v1/orders/summary", companyId));

export type NewOrder = {
  companyId: number;
  externalRef?: string; // vacía = Nexus genera una
  channel?: string;
  courier: string;
  recipientName: string;
  recipientPhone?: string;
  recipientEmail?: string;
  addressLine: string;
  commune: string;
  regionCode?: string;
  dispatchType?: string;
  customerNotes?: string;
  items: OrderItem[];
};

export const createOrder = (body: NewOrder) => api<Order>("orders", "/v1/orders", { method: "POST", body: JSON.stringify(body) });

const post = <T>(path: string, body: unknown = {}) => api<T>("orders", path, { method: "POST", body: JSON.stringify(body) });

// Acciones por caso de uso (API v1.1). Ver orders-service/API.md.
export const takeForPacking = (id: number) => post<Order>(`/v1/orders/${id}/packing/take`);
export const setPackages = (id: number, packages: { weightKg?: number }[]) =>
  api<{ data: Package[] }>("orders", `/v1/orders/${id}/packages`, { method: "PUT", body: JSON.stringify({ packages }) });
export const confirmPacking = (id: number, items: OrderItem[], version: number) =>
  post<{ order: Order; discrepancies: string[] }>(`/v1/orders/${id}/packing/confirm`, { items, version });
export const cancelOrder = (id: number, reason: string) => post<Order>(`/v1/orders/${id}/cancel`, { reason });
export const recoverVoided = (id: number, note: string) => post<Order>(`/v1/orders/${id}/recover`, { note });
export const reportIncident = (id: number, kind: string, reason: string) => post<Order>(`/v1/orders/${id}/incident`, { kind, reason });
export const resolveIncident = (id: number, action: "resume" | "retry" | "cancel", note: string) =>
  post<Order>(`/v1/orders/${id}/resolve`, { action, note });

/**
 * Puente para los botones actuales (OrderActions). Las pantallas definitivas
 * de empaque y retiro (Jean) usan las funciones de arriba directamente.
 */
export async function applyEvent(order: Order, event: string, reason = ""): Promise<unknown> {
  const id = order.id;
  switch (event) {
    case "pick_confirmed": // SIMULADO: en producción lo confirma Invas
      return post(`/v1/orders/${id}/simulate-picked`);
    case "start_packing":
      return takeForPacking(id);
    case "packed": {
      // Atajo: 1 bulto y verificación igual a la orden (la pantalla de
      // empaque real cuenta productos y registra cada bulto).
      const d = await api<{ packages: Package[] }>("orders", `/v1/orders/${id}`);
      if (d.packages.length === 0) await setPackages(id, [{}]);
      const fresh = await api<{ order: Order }>("orders", `/v1/orders/${id}`);
      return confirmPacking(id, order.items ?? [], fresh.order.version);
    }
    case "hand_over": {
      // Atajo: manifiesto de una orden, escaneo de sus bultos y cierre.
      const m = await post<{ manifest: { id: number; lines: { orderId: number; packages: { code: string }[] }[] } }>(
        "/v1/dispatch/manifests",
        { courier: order.courier, orderIds: [id] },
      );
      for (const p of m.manifest.lines.find((l) => l.orderId === id)?.packages ?? []) {
        await post(`/v1/dispatch/manifests/${m.manifest.id}/scan`, { code: p.code });
      }
      return post(`/v1/dispatch/manifests/${m.manifest.id}/close`, { pickerName: reason });
    }
    case "resolve":
      return resolveIncident(id, "resume", reason);
    case "revalidate":
      return recoverVoided(id, reason);
    case "incident":
      return reportIncident(id, "otro", reason);
    case "cancel":
      return cancelOrder(id, reason);
  }
  throw new Error("Acción desconocida: " + event);
}

export const setPriority = (id: number, priority: boolean, reason = "") =>
  api<Order>("orders", `/v1/orders/${id}/priority`, { method: "POST", body: JSON.stringify({ priority, reason }) });

export const listWaves = () => api<{ data: Wave[] }>("orders", "/v1/waves");

export const assignWave = (orderIds: number[], waveId: string) =>
  api<{ queued: number[] }>("orders", "/v1/waves/assign", { method: "POST", body: JSON.stringify({ orderIds, waveId }) });
