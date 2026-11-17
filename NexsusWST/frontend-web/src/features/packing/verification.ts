import type { OrderItem } from "@/services/orders";

export type Counts = Record<string, number>;
export type ScanNotice = { tone: "ok" | "warn" | "error"; text: string };

// Una lectura del lector (o el ingreso manual) suma 1 al SKU. La regla definitiva la aplica el servidor.
export function scanSku(items: OrderItem[], counts: Counts, code: string): { counts: Counts; notice: ScanNotice } {
  const value = code.trim();
  const item = items.find((i) => i.sku.toLowerCase() === value.toLowerCase());
  if (!item) return { counts, notice: { tone: "error", text: `El SKU ${value} no es de esta orden.` } };
  const current = counts[item.sku] ?? 0;
  if (current >= item.quantity) return { counts, notice: { tone: "warn", text: `Ya verificaste todas las unidades de ${item.sku}.` } };
  return { counts: { ...counts, [item.sku]: current + 1 }, notice: { tone: "ok", text: `${item.sku}: ${current + 1} de ${item.quantity}.` } };
}

export const allVerified = (items: OrderItem[], counts: Counts) => items.length > 0 && items.every((i) => (counts[i.sku] ?? 0) === i.quantity);

// Lo verificado sobrevive a una recarga de la pestaña, pero no se comparte con otro dispositivo.
const storageKey = (orderId: number) => `nexus:packing:${orderId}`;

export function readCounts(orderId: number, items: OrderItem[]): Counts {
  try {
    const saved = JSON.parse(sessionStorage.getItem(storageKey(orderId)) ?? "{}") as Counts;
    return Object.fromEntries(items.map((i) => [i.sku, Math.min(Math.max(Number(saved[i.sku]) || 0, 0), i.quantity)]));
  } catch {
    return {};
  }
}

export function saveCounts(orderId: number, counts: Counts) {
  try {
    sessionStorage.setItem(storageKey(orderId), JSON.stringify(counts));
  } catch {
    // Sin almacenamiento solo se pierde lo verificado al recargar.
  }
}

export function clearCounts(orderId: number) {
  try {
    sessionStorage.removeItem(storageKey(orderId));
  } catch {
    // Nada que limpiar.
  }
}
