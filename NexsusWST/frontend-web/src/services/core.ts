// Llamadas al Core (backend-core, Go): clientes, productos, catálogos.
import { api, withCompany } from "./api";

export type Company = { id: number; name: string; taxId: string; wmsCode: string; active: boolean };
export type Product = {
  id: number;
  companyId: number;
  sku: string;
  description: string;
  type: string;
  active: boolean;
  createdAt: string;
};
export type CatalogItem = { code: string; name: string };

export const listCompanies = () => api<{ data: Company[] }>("core", "/v1/clients");

export const listProducts = (search: string, companyId: number | null) =>
  api<{ data: Product[] }>("core", withCompany(`/v1/products?limit=100&search=${encodeURIComponent(search)}`, companyId));

export const createProduct = (body: { companyId: number; sku: string; description: string; type: string }) =>
  api<Product>("core", "/v1/products", { method: "POST", body: JSON.stringify(body) });

// Catálogos cacheados en el Core (Redis + ETag): el navegador los reutiliza.
export const getCatalog = (name: "couriers" | "regions" | "communes" | "dispatch_types") => api<{ data: CatalogItem[] }>("core", `/v1/catalogs/${name}`, { cache: "default" });
