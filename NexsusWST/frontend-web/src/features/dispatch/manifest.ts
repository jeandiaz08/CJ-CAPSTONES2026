import type { ManifestLine } from "@/services/dispatch";

// Una orden sale en el manifiesto mientras su línea esté «included».
export const activeLines = (lines: ManifestLine[]) => lines.filter((l) => l.status === "included");

export function lineProgress(line: ManifestLine) {
  const scanned = line.packages.filter((p) => p.scanned).length;
  return { scanned, total: line.packages.length, missing: line.packages.filter((p) => !p.scanned).map((p) => p.code) };
}

// «Despachar igual»: autorización de una orden con bultos faltantes.
export type DispatchAuthorization = {
  orderId: number;
  externalRef: string;
  missing: string[];
  pickerName: string;
  authorizedBy: string;
  at: string; // ISO
  reason: string;
};

const dateTime = (iso: string) => new Date(iso).toLocaleString("es-CL", { dateStyle: "short", timeStyle: "short" });

// TODO(api): pasar a `dispatchAnyway[{orderId, reason}]` de /close cuando llegue la rama de Cristopher.
// Mientras tanto la autorización viaja en `notes`, una línea por orden.
export const authorizationNote = (a: DispatchAuthorization) =>
  `Autorizado sin bulto · #${a.externalRef} · ${a.missing.join(", ")} · Retira: ${a.pickerName} · Autoriza: ${a.authorizedBy} · ${dateTime(a.at)} · Motivo: ${a.reason}`;

const COMPANY_PREFIX = "Empresa del transporte: ";

// TODO(api): la empresa del transporte no tiene campo propio en /close; va como primera línea de `notes`.
export function closeNotes(company: string, notes: string, authorizations: DispatchAuthorization[]) {
  return [company.trim() && COMPANY_PREFIX + company.trim(), notes.trim(), ...authorizations.map(authorizationNote)].filter(Boolean).join("\n");
}

export function transportCompany(notes?: string) {
  const line = notes?.split("\n").find((l) => l.startsWith(COMPANY_PREFIX));
  return line ? line.slice(COMPANY_PREFIX.length) : "";
}

// Las autorizaciones sobreviven a una recarga hasta confirmar la entrega.
const storageKey = (manifestId: number) => `nexus:dispatch:manifest:${manifestId}`;

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
