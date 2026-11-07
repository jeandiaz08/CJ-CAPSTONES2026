// Retiro y entrega al courier (H-13, CU-05). Requiere el permiso Despacho.
import { api, withCompany } from "./api";

export type CourierQueue = { courier: string; orders: number; packages: number };

export type ManifestBundle = { id: number; code: string; scanned: boolean };
export type ManifestLine = {
  orderId: number;
  companyId: number;
  externalRef: string;
  recipientName: string;
  addressLine: string;
  commune: string;
  status: "included" | "moved" | "cancelled" | "pending_packages" | "delivered" | "removed";
  movedTo?: string;
  packages: ManifestBundle[];
};
export type Manifest = {
  id: number;
  courier: string;
  status: "open" | "closed";
  createdBy: string;
  createdAt: string;
  closedBy?: string;
  closedAt?: string;
  pickerName?: string;
  pickerId?: string;
  vehiclePlate?: string;
  notes?: string;
  lines: ManifestLine[];
};

const post = <T>(path: string, body: unknown) => api<T>("orders", path, { method: "POST", body: JSON.stringify(body) });

export const getDispatchCouriers = (companyId: number | null) =>
  api<{ data: CourierQueue[] }>("orders", withCompany("/v1/dispatch/couriers", companyId));
// TODO(api): el servidor no filtra por courier ni por empresa; se filtra aquí.
export const listManifests = (status: "open" | "closed") => api<{ data: Manifest[] }>("orders", `/v1/dispatch/manifests?status=${status}`);
export const createManifest = (courier: string, orderIds: number[]) =>
  post<{ manifest: Manifest; excluded: string[] }>("/v1/dispatch/manifests", { courier, orderIds });
export const scanPackage = (manifestId: number, code: string) => post<ManifestLine>(`/v1/dispatch/manifests/${manifestId}/scan`, { code });
