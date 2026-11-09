// Clientes (Tarea 07). Fase 1: menú, rutas y permiso «Gestionar perfiles».
// Usa el seed de desarrollo: admin@nexus.local con el permiso y admin.lectura@nexus.local sin él.
import { expect, test, type Page } from "@playwright/test";
import { coreApi, login } from "./helpers";

const sidebar = (page: Page) => page.locator(".sidebar");

async function openClientes(page: Page) {
  await sidebar(page).getByRole("button", { name: "Clientes" }).click();
}

test.describe("Clientes · Fase 1: permiso", () => {
  test("API: escribir usuarios exige «Gestionar perfiles»; sin él, el Administrador solo consulta", async () => {
    const admin = await coreApi("admin@nexus.local");
    const reader = await coreApi("admin.lectura@nexus.local");
    const jefe = await coreApi("jefe@nexus.local");

    const me = await admin<{ permissions: string[] }>("GET", "/v1/auth/me");
    expect(me.body.permissions).toContain("gestionar_perfiles");
    expect((await reader<{ permissions: string[] }>("GET", "/v1/auth/me")).body.permissions).not.toContain("gestionar_perfiles");

    // Sin el permiso: 403 con el nombre del permiso. Con él: pasa el guardia (422 por datos vacíos).
    const denied = await reader<{ error?: { message?: string } }>("POST", "/v1/users", {});
    expect(denied.status).toBe(403);
    expect(JSON.stringify(denied.body)).toContain("Gestionar perfiles");
    expect((await reader("PATCH", "/v1/users/5", { name: "x" })).status).toBe(403);
    expect((await reader("POST", "/v1/users/5/disable", { reason: "prueba" })).status).toBe(403);
    expect((await jefe("POST", "/v1/users", {})).status).toBe(403);
    expect((await admin("POST", "/v1/users", {})).status).toBe(422);

    // La consulta sigue abierta para el Administrador sin el permiso.
    expect((await reader("GET", "/v1/users?limit=1")).status).toBe(200);
  });
});

test.describe("Clientes · Fase 1: menú y rutas", () => {
  test("Administrador sin el permiso: Administración clientes en el menú e Integraciones con la empresa", async ({ page }) => {
    await login(page, "admin.lectura@nexus.local");
    await openClientes(page);
    await expect(sidebar(page).getByRole("link", { name: /Administración clientes/ })).toBeVisible();
    await expect(sidebar(page).getByRole("link", { name: /Perfiles empresa/ })).toHaveCount(0);
    await sidebar(page).getByRole("link", { name: /Administración clientes/ }).click();
    await expect(page.getByRole("heading", { name: "Administración clientes" })).toBeVisible();
    for (const tab of ["Perfiles", "Empresas", "Solicitudes"]) await expect(page.getByRole("link", { name: tab, exact: true })).toBeVisible();

    await expect(sidebar(page).getByRole("link", { name: "Integraciones" })).toBeVisible();
    await page.goto("/integraciones?empresa=1");
    await expect(page.getByRole("heading", { name: "Integraciones" })).toBeVisible();
    await expect(page.getByText("Empresa Demo Uno SpA")).toBeVisible();
    await expect(page.getByText("Próximamente.")).toBeVisible();

    // Perfiles empresa es solo del Cliente: la URL vuelve al Home.
    await page.goto("/clientes/perfiles-empresa");
    await page.waitForURL((url) => url.pathname === "/");
  });

  test("Jefe de Operaciones: ya no ve Clientes ni entra por URL", async ({ page }) => {
    await login(page, "jefe@nexus.local");
    await expect(sidebar(page).getByRole("button", { name: "Clientes" })).toHaveCount(0);
    await expect(sidebar(page).getByRole("link", { name: "Integraciones" })).toHaveCount(0);
    await page.goto("/clientes/administracion");
    await page.waitForURL((url) => url.pathname === "/");
  });

  test("Cliente: solo Perfiles empresa", async ({ page }) => {
    await login(page, "cliente1@nexus.local");
    await openClientes(page);
    await expect(sidebar(page).getByRole("link", { name: /Perfiles empresa/ })).toBeVisible();
    await expect(sidebar(page).getByRole("link", { name: /Administración clientes/ })).toHaveCount(0);
    await sidebar(page).getByRole("link", { name: /Perfiles empresa/ }).click();
    await expect(page.getByRole("link", { name: "Mis perfiles" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Mis solicitudes" })).toBeVisible();
    await page.goto("/clientes/administracion");
    await page.waitForURL((url) => url.pathname === "/");
  });
});
