import type { DispatchAnyway, ManifestLine } from "@/services/dispatch";

// Una orden sale en el manifiesto mientras su línea esté «included».
export const activeLines = (lines: ManifestLine[]) => lines.filter((l) => l.status === "included");

export function lineProgress(line: ManifestLine) {
  const scanned = line.packages.filter((p) => p.scanned).length;
  return { scanned, total: line.packages.length, missing: line.packages.filter((p) => !p.scanned).map((p) => p.code) };
}

// «Despachar igual»: autorización de una orden con bultos faltantes. Viaja en `dispatchAnyway`
// de /close y el servidor la entrega al courier (lo valida el permiso Despacho).
export const MIN_DISPATCH_REASON = 5;
export type DispatchAuthorization = {
  orderId: number;
  externalRef: string;
  missing: string[];
  pickerName: string;
  authorizedBy: string;
  at: string; // ISO
  reason: string;
};

export const dispatchAnyway = (list: DispatchAuthorization[]): DispatchAnyway[] => list.map((a) => ({ orderId: a.orderId, reason: a.reason }));

// Observación opcional de una orden del manifiesto (por ejemplo, un producto que faltó dentro de un bulto).
export type OrderObservation = { orderId: number; externalRef: string; text: string };

// TODO(api): campo propio por orden en /close; hoy va en `notes`, una línea por orden.
export const observationNote = (o: OrderObservation) => `Observación · #${o.externalRef} · ${o.text}`;

// TODO(api): la empresa del transporte no tiene campo propio en /close; va como primera línea de `notes`.
export function closeNotes(company: string, notes: string, observations: OrderObservation[] = []) {
  return [company.trim() && `Empresa del transporte: ${company.trim()}`, notes.trim(), ...observations.map(observationNote)].filter(Boolean).join("\n");
}

// Autorizaciones y observaciones sobreviven a una recarga hasta confirmar la entrega.
const storageKey = (manifestId: number) => `nexus:dispatch:manifest:${manifestId}`;
const observationsKey = (manifestId: number) => `nexus:dispatch:observations:${manifestId}`;

export function readObservations(manifestId: number): OrderObservation[] {
  try {
    const saved = JSON.parse(sessionStorage.getItem(observationsKey(manifestId)) ?? "[]");
    return Array.isArray(saved) ? saved : [];
  } catch {
    return [];
  }
}

export function saveObservations(manifestId: number, list: OrderObservation[]) {
  try {
    if (list.length) sessionStorage.setItem(observationsKey(manifestId), JSON.stringify(list));
    else sessionStorage.removeItem(observationsKey(manifestId));
  } catch {
    // Sin almacenamiento solo se pierden al recargar.
  }
}

export function readAuthorizations(manifestId: number): DispatchAuthorization[] {
  try {
    const saved = JSON.parse(sessionStorage.getItem(storageKey(manifestId)) ?? "[]");
    return Array.isArray(saved) ? saved : [];
  } catch {
    return [];
  }
}

export function saveAuthorizations(manifestId: number, list: DispatchAuthorization[]) {
  try {
    if (list.length) sessionStorage.setItem(storageKey(manifestId), JSON.stringify(list));
    else sessionStorage.removeItem(storageKey(manifestId));
  } catch {
    // Sin almacenamiento solo se pierden al recargar.
  }
}
