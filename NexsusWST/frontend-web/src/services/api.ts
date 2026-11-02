// Cliente HTTP del NAVEGADOR: siempre rutas relativas a través de Traefik
// (/api/core, /api/orders). Nunca nombres internos de Docker (DAS §7.2).
import { getToken } from "@/state/auth";

export type FieldError = { field: string; message: string };

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public code?: string,
    public fields: FieldError[] = [],
    public ref?: string,
  ) {
    super(message);
  }
}

// Texto para el usuario según el código de error (AGENTS: 403, 409, 500 con ref).
export function apiErrorMessage(error: unknown): string {
  if (!(error instanceof ApiError)) return (error as Error)?.message || "Ocurrió un error inesperado.";
  if (error.status === 403) return "Sin permiso para realizar esta acción.";
  if (error.status === 409) return `Otra persona cambió esta orden. ${error.message}`;
  if (error.status >= 500 && error.ref && !error.message.includes(error.ref)) return `${error.message} (ref: ${error.ref})`;
  return error.message;
}

type Service = "core" | "orders";

export async function api<T>(service: Service, path: string, init?: RequestInit): Promise<T> {
  const token = getToken();
  const res = await fetch(`/api/${service}${path}`, {
    cache: "no-store",
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init?.headers ?? {}),
    },
  });
  if (res.status === 204) return undefined as T;
  const body = await res.json().catch(() => ({}));
  if (res.status === 401 && typeof window !== "undefined") {
    // Token inválido o vencido, o usuario deshabilitado (perfil inactivo)
    window.dispatchEvent(new CustomEvent("nexus:unauthorized", { detail: { code: body.error } }));
  }
  if (!res.ok) {
    // 422 de validación: el servidor indica el motivo por campo
    const fields: FieldError[] = Array.isArray(body.fields) ? body.fields : [];
    const summary = fields.map((f) => f.message).join(" · ");
    throw new ApiError(summary || body.message || `HTTP ${res.status}`, res.status, body.error, fields, body.ref);
  }
  return body as T;
}

/** Agrega company_id si hay una empresa elegida en la barra superior. */
export function withCompany(path: string, companyId: number | null): string {
  if (companyId == null) return path;
  return path + (path.includes("?") ? "&" : "?") + `company_id=${companyId}`;
}

export function wsUrl(path: string): string {
  const proto = window.location.protocol === "https:" ? "wss" : "ws";
  return `${proto}://${window.location.host}/api/orders${path}?access_token=${encodeURIComponent(getToken() ?? "")}`;
}

export async function fetchDocument(path:string){const token=getToken();const response=await fetch(`/api/orders${path}`,{headers:token?{Authorization:`Bearer ${token}`}:{},cache:"no-store"});if(!response.ok)throw new ApiError("No se pudo descargar el documento",response.status);return response.blob()}
