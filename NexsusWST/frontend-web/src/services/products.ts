// Productos (Core, Tarea 09, H-04). Contrato en docs/API.md, «Productos».
// listProducts de services/core.ts (formulario de órdenes) usa la misma ruta GET /v1/products.
import { api, downloadFile } from "./api";

export type ProductType = "simple" | "kit";

export type CatalogProduct = {
  id: number;
  companyId: number;
  companyName: string;
  sku: string;
  description: string;
  type: ProductType;
  active: boolean;
  waveCode: string | null; // null = «General (según reglas)»
  createdAt: string;
  updatedAt: string;
  updatedBy: string; // quién hizo la última acción (vacío si no hay)
  // Solo en el detalle de un kit: su composición (un kit no tiene stock propio).
  components?: { productId: number; sku: string; description: string; quantity: number }[];
};

export type ProductFilters = {
  q?: string;
  company_id?: number;
  type?: ProductType;
  status?: "active" | "inactive";
  wave?: string; // "general" = sin ola propia
  ids?: number[];
  limit?: number;
  offset?: number;
};

export type ComponentInput = { sku: string; quantity: number };
export type ProductInput = { companyId: number; sku: string; description: string; type: ProductType; waveCode: string | null; components?: ComponentInput[] };
export type ProductEdit = { description: string; waveCode: string | null; components?: ComponentInput[] };

function query(f: ProductFilters) {
  const p = new URLSearchParams();
  for (const [key, value] of Object.entries(f)) {
    if (value === undefined || value === "" || (Array.isArray(value) && value.length === 0)) continue;
    p.set(key, Array.isArray(value) ? value.join(",") : String(value));
  }
  const s = p.toString();
  return s ? `?${s}` : "";
}

export const listCatalog = (f: ProductFilters) =>
  api<{ data: CatalogProduct[]; total: number; limit: number; offset: number }>("core", `/v1/products${query(f)}`);
export const getCatalogProduct = (id: number) => api<CatalogProduct>("core", `/v1/products/${id}`);
export const createCatalogProduct = (body: ProductInput) =>
  api<CatalogProduct>("core", "/v1/products", { method: "POST", body: JSON.stringify(body) });
export const updateCatalogProduct = (id: number, body: ProductEdit) =>
  api<CatalogProduct>("core", `/v1/products/${id}`, { method: "PATCH", body: JSON.stringify(body) });
export const disableProduct = (id: number, reason: string) =>
  api<CatalogProduct>("core", `/v1/products/${id}/disable`, { method: "POST", body: JSON.stringify({ reason }) });
export const enableProduct = (id: number, reason: string) =>
  api<CatalogProduct>("core", `/v1/products/${id}/enable`, { method: "POST", body: JSON.stringify({ reason }) });

// Excel (Fase 2): exportar lo autorizado (filtros o seleccionados), plantilla e importación de artículos.
export type ProductImportResult = {
  rows: number;
  created: { row: number; id: number; name: string }[];
  errors: { row: number; field?: string; message: string }[];
};
export const exportProducts = (f: Omit<ProductFilters, "limit" | "offset">) => downloadFile(`/v1/products/export${query(f)}`, "productos.xlsx", "core");
export const downloadProductsTemplate = () => downloadFile("/v1/products/template", "plantilla-productos.xlsx", "core");
export const importProducts = (file: File) => {
  const form = new FormData();
  form.append("file", file);
  return api<ProductImportResult>("core", "/v1/products/import", { method: "POST", body: form });
};
