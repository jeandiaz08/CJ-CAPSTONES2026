// Llamadas al servicio de Órdenes (orders-service, Go).
import { api, downloadFile, withCompany } from "./api";

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
  packagesCount: number;
  warehouse: string;
  assignmentSituation: "en_cola" | "invas_agotado" | "fuera_corte" | "sin_regla" | null;
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
  itemsEditable?: boolean;
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
  handedOverAt?: string;
  cancelledAt?: string;
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

export type Wave = { id: string; code: string; name: string; type: string };

export type WaveRule = { id: number; name: string; position: number; active: boolean; courier: string; companyId: number | null; channel: string; dispatchType: string; warehouse: string; waveCode: string };
export type CourierSettings = { courier: string; maxPackages: number | null; coversAll: boolean; communes: string[] };

export type OrderFilters = {
  view?: "asignacion" | "registro" | "all";
  status?: string;
  q?: string;
  courier?: string;
  channel?: string;
  source?: string;
  situation?: string;
  from?: string;
  to?: string;
  companyId?: number;
  priority?: boolean;
  withoutWave?: boolean;
  limit?: number;
  offset?: number;
};

export function listOrders(f: OrderFilters, companyId: number | null) {
  const p = new URLSearchParams();
  if (f.view) p.set("view", f.view);
  if (f.status) p.set("status", f.status);
  if (f.q) p.set("q", f.q);
  if (f.courier) p.set("courier", f.courier);
  if (f.channel) p.set("channel", f.channel);
  if (f.source) p.set("source", f.source);
  if (f.situation) p.set("situation", f.situation);
  if (f.from) p.set("from", f.from);
  if (f.to) p.set("to", f.to);
  if (f.priority) p.set("priority", "true");
  if (f.withoutWave) p.set("without_wave", "true");
  p.set("limit", String(f.limit ?? 50));
  p.set("offset", String(f.offset ?? 0));
  return api<{ data: Order[]; total: number }>("orders", withCompany(`/v1/orders?${p}`, companyId));
}

export type Package = { manifestId: number | null; id: number; code: string; weightKg?: number };
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

export type OrderChanges = {
  courier?: string;
  destination?: { recipientName: string; recipientPhone: string; recipientEmail: string; addressLine: string; commune: string; regionCode: string };
  dispatchType?: string;
  customerNotes?: string;
  items?: OrderItem[];
  reason?: string;
};

export const updateOrder = (id: number, changes: OrderChanges) => api<Order>("orders", `/v1/orders/${id}`, { method: "PATCH", body: JSON.stringify(changes) });

export function uploadDocument(id: number, file: File, kind: string) {
  const body = new FormData();
  body.set("file", file);
  body.set("kind", kind);
  return api<OrderDocument>("orders", `/v1/orders/${id}/documents`, { method: "POST", body });
}

export type ImportResult = { importId: number; totalRows: number; ordersCreated: number; orderIds: number[]; failedRows: number; errors: { row: number; ref: string; message: string }[] };

export function importOrders(companyId: number, file: File) {
  const body = new FormData();
  body.set("file", file);
  body.set("companyId", String(companyId));
  return api<ImportResult>("orders", "/v1/orders/import", { method: "POST", body });
}

export const downloadImportTemplate = (companyId: number | null) => downloadFile(`/v1/orders/import/template${companyId ? `?company_id=${companyId}` : ""}`, "plantilla-ordenes.xlsx");
export const exportOrders = (params: URLSearchParams, fallbackName: string) => downloadFile(`/v1/orders/export?${params}`, fallbackName);

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
export type ResolveCorrections = { addressLine?: string; commune?: string; regionCode?: string; courier?: string };
export const resolveIncident = (id: number, action: "resume" | "retry" | "cancel", note: string, corrections?: ResolveCorrections) =>
  post<Order>(`/v1/orders/${id}/resolve`, { action, note, ...(corrections ? { corrections } : {}) });

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
  api<{ queued: number[]; skipped: number }>("orders", "/v1/waves/assign", { method: "POST", body: JSON.stringify({ orderIds, waveId }) });

export const listWaveRules = () => api<{ data: WaveRule[] }>("orders", "/v1/wave-rules");
export const saveWaveRule = (rule: WaveRule) => api<WaveRule>("orders", rule.id ? `/v1/wave-rules/${rule.id}` : "/v1/wave-rules", { method: rule.id ? "PUT" : "POST", body: JSON.stringify(rule) });
export const deleteWaveRule = (id: number) => api<void>("orders", `/v1/wave-rules/${id}`, { method: "DELETE" });
export const reorderWaveRules = (ids: number[]) => api<void>("orders", "/v1/wave-rules/reorder", { method: "POST", body: JSON.stringify({ ids }) });
export const getWarehouses = () => api<{ data: string[] }>("orders", "/v1/orders/warehouses");
export const listCourierSettings = () => api<{ data: CourierSettings[] }>("orders", "/v1/couriers/settings");
export const saveCourierSettings = (courier: string, body: Omit<CourierSettings, "courier">) => api<void>("orders", `/v1/couriers/${courier}/settings`, { method: "PUT", body: JSON.stringify(body) });

export const getOrderMeta = () => api<{statuses:{code:OrderStatus;label:string}[]}>("orders","/v1/meta");

export async function downloadOrderDocument(orderId:number,document:OrderDocument){
 const {fetchDocument}=await import("./api");const blob=await fetchDocument(`/v1/orders/${orderId}/documents/${document.id}/download`);const url=URL.createObjectURL(blob);const a=window.document.createElement("a");a.href=url;a.download=document.filename;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
}

export type Cutoff = {id:number;number:number;name:string|null;cutoffTime:string;couriers:{code:string;name:string;arrivalTime:string}[]};
export const getCutoffs=()=>api<{data:Cutoff[]}>("orders","/v1/cutoffs");
export const saveCutoff = (id: number | null, body: { name: string | null; cutoffTime: string; couriers: { code: string; arrivalTime: string }[] }) => api<Cutoff>("orders", id ? `/v1/cutoffs/${id}` : "/v1/cutoffs", { method: id ? "PUT" : "POST", body: JSON.stringify(body) });
export const deleteCutoff = (id: number) => api<void>("orders", `/v1/cutoffs/${id}`, { method: "DELETE" });
export const saveAssignmentDeadline = (assignmentDeadline: string) => api<void>("orders", "/v1/ops-settings/assignment-deadline", { method: "PUT", body: JSON.stringify({ assignmentDeadline }) });
export const getAssignmentDeadline=()=>api<{data:{assignmentDeadline:string}}>("orders","/v1/ops-settings/assignment-deadline");
