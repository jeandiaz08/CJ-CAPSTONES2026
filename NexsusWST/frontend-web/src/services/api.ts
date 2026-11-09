// Cliente HTTP del NAVEGADOR: siempre rutas relativas al BFF de Next (/bff/core, /bff/orders),
// que agrega el token en el servidor. Nunca nombres internos de Docker (DAS §7.2).

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
  const res = await fetch(`/bff/${service}${path}`, {
    cache: "no-store",
    ...init,
    headers: {
      // Con FormData el navegador pone el Content-Type con su boundary.
      ...(init?.body instanceof FormData ? {} : { "Content-Type": "application/json" }),
      ...(init?.headers ?? {}),
    },
  });
  if (res.status === 204) return undefined as T;
  const body = await res.json().catch(() => ({}));
  if (res.status === 401 && typeof window !== "undefined") {
    // El BFF ya intentó renovar una vez: sesión terminada o usuario deshabilitado.
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

// El WebSocket no pasa por el BFF: se abre directo a Órdenes con un ticket de un solo uso.
export function wsUrl(path: string, ticket: string): string {
  const proto = window.location.protocol === "https:" ? "wss" : "ws";
  return `${proto}://${window.location.host}/api/orders${path}?ticket=${encodeURIComponent(ticket)}`;
}

export function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

// Pide un archivo con la sesión. Conserva el mensaje del servidor si falla (por ejemplo, el tope de filas).
async function fetchWithSession(path: string, service: Service = "orders") {
  const response = await fetch(`/bff/${service}${path}`, { cache: "no-store" });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    if (response.status === 401) window.dispatchEvent(new CustomEvent("nexus:unauthorized", { detail: { code: body.error } }));
    throw new ApiError(body.message || "No se pudo descargar el archivo.", response.status, body.error, [], body.ref);
  }
  return response;
}

export async function downloadFile(path: string, fallbackName: string, service: Service = "orders") {
  const response = await fetchWithSession(path, service);
  const name = response.headers.get("Content-Disposition")?.match(/filename="?([^";]+)"?/)?.[1];
  saveBlob(await response.blob(), name ?? fallbackName);
}

const PRINT_LOAD_TIMEOUT_MS = 8000;
const PRINT_URL_LIFETIME_MS = 60000;

// Si no se puede imprimir: pestaña nueva y, si también la bloquea, descarga.
function openPdfFallback(url: string, blob: Blob, filename: string) {
  if (!window.open(url, "_blank")) saveBlob(blob, filename);
}

// Abre el diálogo de impresión del navegador con el PDF, desde un iframe oculto.
export async function printPdf(path: string, fallbackName: string) {
  const response = await fetchWithSession(path);
  const blob = new Blob([await response.blob()], { type: "application/pdf" });
  const url = URL.createObjectURL(blob);
  const iframe = document.createElement("iframe");
  iframe.title = "Impresión";
  iframe.style.cssText = "position:fixed;right:0;bottom:0;width:0;height:0;border:0";
  let settled = false;
  const fallback = () => {
    if (settled) return;
    settled = true;
    openPdfFallback(url, blob, fallbackName);
  };
  const timer = setTimeout(fallback, PRINT_LOAD_TIMEOUT_MS);
  iframe.onload = () => {
    if (settled) return;
    clearTimeout(timer);
    try {
      iframe.contentWindow?.focus();
      iframe.contentWindow?.print();
      settled = true;
    } catch {
      fallback();
    }
  };
  iframe.src = url;
  document.body.append(iframe);
  // El diálogo no avisa cuándo termina: se limpia cuando ya no hace falta el archivo.
  setTimeout(() => {
    iframe.remove();
    URL.revokeObjectURL(url);
  }, PRINT_URL_LIFETIME_MS);
}

export async function fetchDocument(path:string){const response=await fetch(`/bff/orders${path}`,{cache:"no-store"});if(!response.ok)throw new ApiError("No se pudo descargar el documento",response.status);return response.blob()}
