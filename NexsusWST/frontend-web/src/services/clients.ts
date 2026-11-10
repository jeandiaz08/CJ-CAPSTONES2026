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

function query(f: Record<string, string | number | number[] | undefined>) {
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

// ---------------------------------------------------------------------------------------------
// Perfiles. scope «client» = perfiles con rol Cliente (Tarea 07); «internal» = Configuración ›
// Usuarios WeStorage (Tarea 08: rol, permisos y usuario sin correo).
// ---------------------------------------------------------------------------------------------
export type ProfileScope = "client" | "internal";

export type Profile = {
  id: number;
  name: string;
  email: string;
  username?: string; // perfil interno sin correo
  hasEmail: boolean;
  phone: string;
  jobTitle: string;
  role: string;
  permissions: string[];
  companyId: number;
  companyName: string;
  active: boolean;
  linked: boolean;
  createdAt: string;
  lastLoginAt: string | null;
  temporaryPassword?: string; // solo al crear un perfil interno sin correo (se muestra una vez)
};

export type ProfileFilters = {
  q?: string;
  company_id?: number;
  status?: "active" | "inactive";
  from?: string;
  to?: string;
  ids?: number[];
  limit?: number;
  offset?: number;
};

// companyId solo en clientes; username, role y permissions solo en el alcance interno.
export type ProfileInput = { name: string; companyId?: number; phone: string; jobTitle: string; email?: string; username?: string; role?: string; permissions?: string[] };

export type ImportResult = {
  rows: number;
  created: { row: number; id: number; name: string; email: string }[];
  errors: { row: number; field?: string; message: string }[];
};

const withScope = (scope: ProfileScope, path: string) => path + (path.includes("?") ? "&" : "?") + `scope=${scope}`;

export const listProfiles = (scope: ProfileScope, f: ProfileFilters) =>
  api<{ data: Profile[]; total: number; limit: number; offset: number }>("core", withScope(scope, `/v1/profiles${query(f)}`));
export const getProfile = (scope: ProfileScope, id: number) => api<Profile>("core", withScope(scope, `/v1/profiles/${id}`));
export const createProfile = (scope: ProfileScope, body: ProfileInput) =>
  api<Profile>("core", withScope(scope, "/v1/profiles"), { method: "POST", body: JSON.stringify(body) });
export const updateProfile = (scope: ProfileScope, id: number, body: Omit<ProfileInput, "email" | "username">) =>
  api<Profile>("core", withScope(scope, `/v1/profiles/${id}`), { method: "PATCH", body: JSON.stringify(body) });
export const disableProfile = (scope: ProfileScope, id: number, reason: string) =>
  api<Profile>("core", withScope(scope, `/v1/profiles/${id}/disable`), { method: "POST", body: JSON.stringify({ reason }) });
export const enableProfile = (scope: ProfileScope, id: number, reason: string) =>
  api<Profile>("core", withScope(scope, `/v1/profiles/${id}/enable`), { method: "POST", body: JSON.stringify({ reason }) });
export const resetProfilePassword = (scope: ProfileScope, id: number) =>
  api<{ emailSent: boolean; temporaryPassword?: string }>("core", withScope(scope, `/v1/profiles/${id}/password`), { method: "POST" });
export const exportProfiles = (scope: ProfileScope, f: Omit<ProfileFilters, "limit" | "offset">) =>
  downloadFile(withScope(scope, `/v1/profiles/export${query(f)}`), "perfiles.xlsx", "core");
export const downloadProfilesTemplate = (scope: ProfileScope) =>
  downloadFile(withScope(scope, "/v1/profiles/template"), "plantilla-perfiles.xlsx", "core");
export const importProfiles = (scope: ProfileScope, file: File) => {
  const form = new FormData();
  form.append("file", file);
  return api<ImportResult>("core", withScope(scope, "/v1/profiles/import"), { method: "POST", body: form });
};

// ---------------------------------------------------------------------------------------------
// Solicitudes de perfiles (Tarea 07, Fase 4). El Cliente crea y ve las de su empresa (lo filtra el
// servidor); Administración clientes ve todas y, con «Gestionar perfiles», acepta o rechaza.
// ---------------------------------------------------------------------------------------------
export type RequestKind = "new_profile" | "activation" | "deactivation";
export type RequestStatus = "pending" | "approved" | "rejected";

export type ProfileRequest = {
  id: number;
  companyId: number;
  companyName: string;
  kind: RequestKind;
  kindLabel: string;
  targetUserId: number | null;
  profile: { name: string; email: string; phone: string; jobTitle: string };
  reason: string;
  status: RequestStatus;
  requestedBy: { id: number; name: string };
  requestedAt: string;
  reviewedBy: { id: number; name: string } | null;
  reviewedAt: string | null;
  rejectionReason: string;
  createdUserId: number | null;
};

export type RequestFilters = { q?: string; status?: RequestStatus; kind?: RequestKind; from?: string; to?: string; ids?: number[]; limit?: number; offset?: number };
export type RequestCounts = { pending: number; approved: number; rejected: number };
export type NewRequest = { kind: RequestKind; targetUserId?: number; name?: string; email?: string; phone?: string; jobTitle?: string; reason: string };

export const listRequests = (f: RequestFilters) =>
  api<{ data: ProfileRequest[]; total: number; counts: RequestCounts; limit: number; offset: number }>("core", `/v1/profile-requests${query(f)}`);
export const getRequest = (id: number) => api<ProfileRequest>("core", `/v1/profile-requests/${id}`);
export const createRequest = (body: NewRequest) => api<ProfileRequest>("core", "/v1/profile-requests", { method: "POST", body: JSON.stringify(body) });
// Sin cuerpo acepta tal cual; con los datos del perfil es «Editar y aceptar».
export const approveRequest = (id: number, edits?: { name: string; email: string; phone: string; jobTitle: string }) =>
  api<ProfileRequest>("core", `/v1/profile-requests/${id}/approve`, { method: "POST", ...(edits ? { body: JSON.stringify(edits) } : {}) });
export const rejectRequest = (id: number, reason: string) =>
  api<ProfileRequest>("core", `/v1/profile-requests/${id}/reject`, { method: "POST", body: JSON.stringify({ reason }) });
