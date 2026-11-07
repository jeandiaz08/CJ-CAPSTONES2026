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

// API de Órdenes con Bearer (direct grant de desarrollo), para preparar y comprobar datos.
export async function ordersApi(email: string) {
  const res = await fetch(`${BASE}/auth/realms/nexuswst/protocol/openid-connect/token`, {
    method: "POST",
    body: new URLSearchParams({ grant_type: "password", client_id: "nexus-web", username: email, password: password(email) }),
  });
  const { access_token: token } = await res.json();
  return async <T = unknown>(method: string, path: string, body?: unknown): Promise<{ status: number; body: T }> => {
    const r = await fetch(`${BASE}/api/orders${path}`, {
      method,
      headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
      body: body ? JSON.stringify(body) : undefined,
    });
    return { status: r.status, body: (await r.json().catch(() => ({}))) as T };
  };
}
