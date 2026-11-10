// Integraciones (Tarea 10, H-06: SCRUM-122 y 123). Solo frontend sobre /v1/integrations.
// El backend no tiene borrado de integraciones: las conexiones que crean estas pruebas (Empresa
// Demo Uno, id 1) quedan desactivadas al terminar, y las aserciones no dependen de cuántas haya.
import { expect, test, type Page } from "@playwright/test";
import { BASE, login, ordersApi } from "./helpers";

type Integration = { id: number; companyId: number; provider: string; kind: string; active: boolean; webhookPath: string; secret?: string };

const sidebar = (page: Page) => page.locator(".sidebar");
const card = (page: Page, name: string) => page.locator(".integration-card").filter({ has: page.getByRole("heading", { name, exact: true }) });

test.describe("Integraciones · Fase 1", () => {
  test("API: solo el Administrador; el webhook se publica en el mismo host", async () => {
    const admin = await ordersApi("admin@nexus.local");
    expect((await admin("GET", "/v1/integrations")).status).toBe(200);
    for (const email of ["jefe@nexus.local", "empaque@nexus.local", "cliente1@nexus.local"]) {
      const api = await ordersApi(email);
      expect((await api("GET", "/v1/integrations")).status).toBe(403);
      expect((await api("POST", "/v1/integrations", { companyId: 1, kind: "channel", provider: "generic" })).status).toBe(403);
    }
    // Traefik enruta /webhooks a Órdenes (no a Next): sin firma responde 401, no 404.
    const created = await admin<Integration>("POST", "/v1/integrations", { companyId: 1, kind: "channel", provider: "generic" });
    expect(created.status).toBe(201);
    const hook = await fetch(`${BASE}${created.body.webhookPath}`, { method: "POST", body: "{}", headers: { "Content-Type": "application/json" } });
    expect(hook.status).toBe(401);
    expect((await admin("POST", `/v1/integrations/${created.body.id}/disable`)).status).toBe(204);
  });

  test("Administrador: tarjetas, «Ver conexiones», panel y Desactivar / Activar", async ({ page }) => {
    const admin = await ordersApi("admin@nexus.local");
    const created = await admin<Integration>("POST", "/v1/integrations", { companyId: 1, kind: "channel", provider: "mercadolibre" });
    expect(created.status).toBe(201);
    const id = created.body.id;

    await login(page, "admin@nexus.local");
    await sidebar(page).getByRole("link", { name: "Integraciones" }).click();
    await expect(page.getByRole("heading", { name: "Integraciones", level: 1 })).toBeVisible();
    for (const section of ["Canales", "Couriers", "Integraciones"]) await expect(page.getByRole("region", { name: section })).toBeVisible();

    // Un badge por estado; tarjetas fijas sin enlace.
    await expect(card(page, "Mercado Libre").locator(".badge")).toHaveText("Disponible");
    await expect(card(page, "Shopify").locator(".badge")).toHaveText("Próximamente");
    await expect(card(page, "WooCommerce").locator(".badge")).toHaveText("Próximamente");
    await expect(card(page, "Recíbelo")).toContainText("Buscar en oficina");
    await expect(card(page, "Recíbelo").locator(".badge")).toHaveText("Sin integración");
    await expect(card(page, "Mercado Envíos")).toContainText("Llega por el canal Mercado Libre");
    await expect(card(page, "Invas WMS")).toContainText("Configurado en el servidor");
    await expect(page.getByText("API mock")).toHaveCount(0);
    await expect(card(page, "Shopify").getByRole("link")).toHaveCount(0);
    await expect(card(page, "Mercado Libre")).toContainText(/\d+ (conexión activa|conexiones activas)/);

    // «Ver conexiones» abre la pestaña Clientes filtrada por ese proveedor.
    await card(page, "Mercado Libre").getByRole("link", { name: /Ver conexiones/ }).click();
    await page.waitForURL(/tab=clientes/);
    expect(new URL(page.url()).searchParams.get("proveedor")).toBe("mercadolibre");
    const rows = page.locator(".connections-list tbody tr");
    await expect(rows.first()).toBeVisible();
    await expect(page.locator(".connections-list thead th")).toHaveText(["Empresa", "Tipo", "Proveedor", "Estado", "Creada"]);
    for (const text of await rows.locator("td:nth-child(3)").allTextContents()) expect(text).toBe("Mercado Libre");

    // Panel de la conexión recién creada.
    await page.goto(`/integraciones?tab=clientes&conexion=${id}`);
    const panel = page.locator(".clients-panel");
    await expect(panel.getByRole("heading", { level: 2 })).toHaveText("Empresa Demo Uno SpA · Mercado Libre");
    await expect(panel.locator(".secret-row code").first()).toHaveText(`${BASE}/webhooks/mercadolibre/${id}`);
    await expect(panel.getByLabel("Secreto oculto")).toHaveText(/^•+$/);
    await expect(panel).toContainText("Aún no disponible.");
    await expect(panel.getByRole("button", { name: "Activar", exact: true })).toHaveCount(0);

    await panel.getByRole("button", { name: "Desactivar", exact: true }).click();
    await page.locator(".form-modal").getByRole("button", { name: "Desactivar", exact: true }).click();
    await expect(panel.locator(".badge")).toHaveText("Inactiva");
    await expect(panel.getByRole("button", { name: "Desactivar", exact: true })).toHaveCount(0);
    const list = async () => (await admin<{ data: Integration[] }>("GET", "/v1/integrations")).body.data.find((i) => i.id === id)!;
    expect((await list()).active).toBe(false);

    await panel.getByRole("button", { name: "Activar", exact: true }).click();
    await page.locator(".form-modal").getByRole("button", { name: "Activar", exact: true }).click();
    await expect(panel.locator(".badge")).toHaveText("Activa");
    expect((await list()).active).toBe(true);

    // «Ver integraciones» de una empresa: abre Clientes filtrada por ella.
    await page.goto("/integraciones?empresa=1");
    await expect(page.getByRole("link", { name: "Clientes", exact: true })).toHaveAttribute("aria-current", "page");
    await expect(page.getByRole("button", { name: "Filtros (1)" })).toBeVisible();
    for (const text of await page.locator(".connections-list tbody tr td:first-child").allTextContents()) expect(text).toBe("Empresa Demo Uno SpA");

    await admin("POST", `/v1/integrations/${id}/disable`); // no hay borrado: queda inactiva
  });

  test("Jefe de Operaciones: sin Integraciones en el menú ni por URL", async ({ page }) => {
    await login(page, "jefe@nexus.local");
    await expect(sidebar(page).getByRole("link", { name: "Integraciones" })).toHaveCount(0);
    await page.goto("/integraciones");
    await page.waitForURL((url) => url.pathname === "/");
  });
});
