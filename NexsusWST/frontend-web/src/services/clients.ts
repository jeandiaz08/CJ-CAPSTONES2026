// Clientes › Administración clientes (Core, Tarea 07). Contrato en docs/API.md, «Empresas cliente».
import { api, downloadFile } from "./api";

export type AdminCompany = {
  id: number;
  name: string;
  taxId: string;
  email: string;
  phone: string;
  waveCode: string | null; // null = «General (según reglas)»
  wmsCode?: string; // solo para el Administrador
  profilesCount: number;
  active: boolean;
  createdAt: string;
  updatedAt: string;
};

export type CompanyFilters = {
  q?: string;
  status?: "active" | "inactive";
  wave?: string; // "general" = sin ola fija
  from?: string;
  to?: string;
  ids?: number[];
  limit?: number;
  offset?: number;
};

export type CompanyInput = {
  name: string;
  taxId: string;
  email: string;
  phone: string;
  waveCode: string | null;
  wmsCode?: string;
};

function query(f: CompanyFilters) {
  const p = new URLSearchParams();
  for (const [key, value] of Object.entries(f)) {
    if (value === undefined || value === "" || (Array.isArray(value) && value.length === 0)) continue;
    p.set(key, Array.isArray(value) ? value.join(",") : String(value));
  }
  const s = p.toString();
  return s ? `?${s}` : "";
}

export const listAdminCompanies = (f: CompanyFilters) =>
  api<{ data: AdminCompany[]; total: number; limit: number; offset: number }>("core", `/v1/companies${query(f)}`);
export const getAdminCompany = (id: number) => api<AdminCompany>("core", `/v1/companies/${id}`);
export const createCompany = (body: CompanyInput) => api<AdminCompany>("core", "/v1/companies", { method: "POST", body: JSON.stringify(body) });
export const updateCompany = (id: number, body: CompanyInput) =>
  api<AdminCompany>("core", `/v1/companies/${id}`, { method: "PATCH", body: JSON.stringify(body) });
export const disableCompany = (id: number, reason: string) =>
  api<AdminCompany>("core", `/v1/companies/${id}/disable`, { method: "POST", body: JSON.stringify({ reason }) });
export const enableCompany = (id: number, reason: string) =>
  api<AdminCompany>("core", `/v1/companies/${id}/enable`, { method: "POST", body: JSON.stringify({ reason }) });
// Excel «Empresas»: mismos filtros de la tabla, o solo las seleccionadas (ids).
export const exportCompanies = (f: Omit<CompanyFilters, "limit" | "offset">) =>
  downloadFile(`/v1/companies/export${query(f)}`, "empresas.xlsx", "core");
