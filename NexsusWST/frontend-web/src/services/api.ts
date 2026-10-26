// Cliente HTTP del NAVEGADOR: siempre rutas relativas a través de Traefik
// (/api/core, /api/orders). Nunca nombres internos de Docker (DAS §7.2).
import { getToken } from "@/state/auth";

export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
    public code?: string,
  ) {
    super(message);
  }
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
    const fields = Array.isArray(body.fields) ? body.fields.map((f: { message: string }) => f.message).join(" · ") : "";
    throw new ApiError(fields || body.message || `HTTP ${res.status}`, res.status, body.error);
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
