import type { Order, Package } from "@/services/orders";

// Una fila de la Lista de retiro: la orden, sus bultos (cuando ya se cargaron) y, si ya está
// en el manifiesto abierto del courier, los bultos que el servidor tiene escaneados.
export type PickupRow = {
  order: Order;
  packages?: Package[];
  manifestId?: number;
  serverScanned: string[];
};

export type ScanNotice = { tone: "ok" | "warn" | "error"; text: string };
export type LocalMarks = { marked: string[]; selected: number[] };

// Sin etiqueta no se entrega, salvo los couriers «Buscar en oficina».
export function labelBlock(order: Order): string | null {
  const { strategy, status } = order.label;
  if (strategy === "office" || status === "ready") return null;
  return status === "failed" ? "Etiqueta no disponible" : status === "pending" ? "Etiqueta pendiente" : "Sin etiqueta";
}

export const normalizeCode = (code: string) => code.trim().toUpperCase();

export function scannedCodes(row: PickupRow, marks: LocalMarks) {
  const codes = (row.packages ?? []).map((p) => p.code);
  return codes.filter((c) => row.serverScanned.includes(c) || marks.marked.includes(c));
}

export function totalPackages(row: PickupRow) {
  return row.packages?.length ?? row.order.packagesCount;
}

export type ScanResult =
  | { kind: "local"; code: string; row: PickupRow; notice: ScanNotice }
  | { kind: "server"; code: string; row: PickupRow; manifestId: number }
  | { kind: "rejected"; notice: ScanNotice };

// Decide qué hacer con una lectura. No cambia nada: quien llama aplica el resultado.
export function readCode(rows: PickupRow[], marks: LocalMarks, raw: string, courierName: string): ScanResult {
  const code = normalizeCode(raw);
  const row = rows.find((r) => r.packages?.some((p) => p.code.toUpperCase() === code));
  if (!row) {
    if (rows.some((r) => !r.packages)) return { kind: "rejected", notice: { tone: "warn", text: "Aún se están cargando los bultos. Vuelve a leer en un momento." } };
    return { kind: "rejected", notice: { tone: "error", text: `El código ${code} no pertenece a la lista de ${courierName}.` } };
  }
  const block = labelBlock(row.order);
  if (block) return { kind: "rejected", notice: { tone: "error", text: `La orden #${row.order.externalRef} no se puede entregar: ${block.toLowerCase()}.` } };
  const done = scannedCodes(row, marks);
  if (done.includes(code)) return { kind: "rejected", notice: { tone: "warn", text: `El bulto ${code} ya estaba marcado.` } };
  if (row.manifestId) return { kind: "server", code, row, manifestId: row.manifestId };
  return { kind: "local", code, row, notice: { tone: "ok", text: `Bulto ${code} · orden #${row.order.externalRef} (${done.length + 1}/${totalPackages(row)}).` } };
}

// Lo marcado antes de generar el manifiesto sobrevive a una recarga, por courier.
const storageKey = (courier: string) => `nexus:dispatch:${courier}`;

export function readMarks(courier: string): LocalMarks {
  try {
    const saved = JSON.parse(sessionStorage.getItem(storageKey(courier)) ?? "{}") as Partial<LocalMarks>;
    return { marked: Array.isArray(saved.marked) ? saved.marked : [], selected: Array.isArray(saved.selected) ? saved.selected : [] };
  } catch {
    return { marked: [], selected: [] };
  }
}

export function saveMarks(courier: string, marks: LocalMarks) {
  try {
    if (marks.marked.length || marks.selected.length) sessionStorage.setItem(storageKey(courier), JSON.stringify(marks));
    else sessionStorage.removeItem(storageKey(courier));
  } catch {
    // Sin almacenamiento solo se pierde lo marcado al recargar.
  }
}

export function clearMarks(courier: string) {
  try {
    sessionStorage.removeItem(storageKey(courier));
  } catch {
    // Nada que limpiar.
  }
}

// Quita lo que ya no está en la lista (la orden cambió de courier o pasó a Incidencia).
export function pruneMarks(marks: LocalMarks, rows: PickupRow[]): LocalMarks {
  const ids = new Set(rows.filter((r) => !labelBlock(r.order) && !r.manifestId).map((r) => r.order.id));
  const codes = new Set(rows.filter((r) => !r.manifestId).flatMap((r) => (r.packages ?? []).map((p) => p.code)));
  const loaded = rows.every((r) => r.packages);
  return {
    marked: loaded ? marks.marked.filter((c) => codes.has(c)) : marks.marked,
    selected: marks.selected.filter((id) => ids.has(id)),
  };
}
