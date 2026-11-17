// Utilidades de las pruebas de punta a punta. Usan los usuarios de prueba de Keycloak (solo
// desarrollo) y nunca imprimen contraseñas ni tokens.
import { readFileSync } from "node:fs";
import { expect, type Page } from "@playwright/test";

type DevUser = { username: string; credentials: { value: string }[] };
const users: DevUser[] = JSON.parse(readFileSync("../infrastructure/keycloak/dev-users/nexuswst-users-0.json", "utf8")).users;
export const BASE = process.env.E2E_BASE ?? "http://localhost";

export const password = (email: string) => {
  const user = users.find((u) => u.username === email);
  if (!user) throw new Error(`Usuario de prueba desconocido: ${email}`);
  return user.credentials[0].value;
};

export async function login(page: Page, email: string) {
  await page.goto("/login");
  await page.getByRole("button", { name: "Ingresar" }).click();
  await page.waitForURL(/\/auth\/realms\//);
  await page.fill('input[name="username"]', email);
  await page.fill('input[name="password"]', password(email));
  await page.locator('#kc-form-login button[type="submit"]').first().click();
  await expect(page.locator(".sidebar")).toBeVisible({ timeout: 90_000 });
}

// Correos de prueba en Mailpit (solo desarrollo). Devuelve los asuntos de los correos a esa dirección.
export const MAILPIT = process.env.E2E_MAILPIT ?? "http://127.0.0.1:8025";
export async function mailSubjects(to: string): Promise<string[]> {
  const res = await fetch(`${MAILPIT}/api/v1/search?query=${encodeURIComponent(`to:"${to}"`)}`);
  const body = (await res.json()) as { messages?: { Subject: string }[] };
  return (body.messages ?? []).map((m) => m.Subject);
}

// API del Core con Bearer (direct grant de desarrollo), igual que ordersApi.
export async function coreApi(email: string) {
  return serviceApi("core", email);
}

// API de Órdenes con Bearer (direct grant de desarrollo), para preparar y comprobar datos.
export async function ordersApi(email: string) {
  return serviceApi("orders", email);
}

async function serviceApi(service: "core" | "orders", email: string) {
  const res = await fetch(`${BASE}/auth/realms/nexuswst/protocol/openid-connect/token`, {
    method: "POST",
    body: new URLSearchParams({ grant_type: "password", client_id: "nexus-web", username: email, password: password(email) }),
  });
  const { access_token: token } = await res.json();
  return async <T = unknown>(method: string, path: string, body?: unknown): Promise<{ status: number; body: T }> => {
    const form = body instanceof FormData; // multipart (importaciones): el boundary lo pone fetch
    const r = await fetch(`${BASE}/api/${service}${path}`, {
      method,
      headers: { Authorization: `Bearer ${token}`, ...(form ? {} : { "Content-Type": "application/json" }) },
      body: form ? body : body ? JSON.stringify(body) : undefined,
    });
    return { status: r.status, body: (await r.json().catch(() => ({}))) as T };
  };
}
