// Home por perfil (Tarea 11). Solo frontend: los números salen de /v1/orders/summary,
// /v1/integrations y /v1/profile-requests, y los recordatorios de /v1/me/notes (Core).
// Las notas que crean estas pruebas se borran al terminar.
import { expect, test, type Page } from "@playwright/test";
import { coreApi, login, ordersApi } from "./helpers";

type Summary = { data: Record<string, number> };
type Note = { id: number; body: string; done: boolean };

const home = (page: Page) => page.locator(".home-view");
const cardTitles = (page: Page) => home(page).locator(".home-card-title").allTextContents();
const quick = (page: Page) => home(page).locator(".home-quick a").allTextContents();
const cardValue = (page: Page, key: string) => home(page).locator(`[data-card="${key}"] strong`);
const sum = (s: Summary, statuses: string[]) => statuses.reduce((n, st) => n + (s.data[st] ?? 0), 0);
const IN_PROGRESS = ["pending", "validating", "validated", "picking", "picked", "packing", "ready_pickup", "incident"];

async function openHome(page: Page, email: string) {
  await login(page, email);
  await page.goto("/");
  await expect(home(page).getByRole("heading", { name: "Home", level: 1 })).toBeVisible();
  await expect(home(page)).toContainText("Bienvenido a NexusWST");
  await expect(home(page)).toContainText("Aquí tienes un resumen de tu operación de hoy");
}

test.describe("Home por perfil", () => {
  test("Administrador: tarjetas con datos reales, accesos y enlace filtrado", async ({ page }) => {
    const api = await ordersApi("admin@nexus.local");
    const summary = (await api<Summary>("GET", "/v1/orders/summary")).body;
    const integrations = (await api<{ data: { active: boolean }[] }>("GET", "/v1/integrations")).body.data;
    await openHome(page, "admin@nexus.local");
    expect(await cardTitles(page)).toEqual(["Órdenes pendientes", "Integraciones activas", "Solicitudes de perfiles pendientes"]);
    expect(await quick(page)).toEqual(["Todas las órdenes", "Órdenes manuales", "Solicitudes", "Registro de acciones", "Integraciones"]);
    await expect(cardValue(page, "pending")).toHaveText(String(sum(summary, ["pending"])));
    await expect(cardValue(page, "integrations")).toHaveText(`${integrations.filter((i) => i.active).length}/${integrations.length}`);
    await expect(cardValue(page, "requests")).toHaveText(/^\d+$/);
    await home(page).locator('[data-card="pending"]').click();
    await expect(page).toHaveURL(/\/ordenes\/todas\/listado\?status=pending$/, { timeout: 60_000 }); // primera compilación en desarrollo
  });

  test("Administrador sin «Gestionar perfiles»: Incidencias abiertas en vez de Solicitudes", async ({ page }) => {
    await openHome(page, "admin.lectura@nexus.local");
    expect(await cardTitles(page)).toEqual(["Órdenes pendientes", "Integraciones activas", "Incidencias abiertas"]);
  });

  test("Jefe de Operaciones: sus tarjetas y accesos, sin Integraciones", async ({ page }) => {
    const summary = (await (await ordersApi("jefe@nexus.local"))<Summary>("GET", "/v1/orders/summary")).body;
    await openHome(page, "jefe@nexus.local");
    expect(await cardTitles(page)).toEqual(["Órdenes por validar", "Órdenes con incidencia", "Listas para retiro"]);
    expect(await quick(page)).toEqual(["Todas las órdenes", "Órdenes manuales", "Retiro y entrega", "Registro histórico"]);
    await expect(cardValue(page, "to-validate")).toHaveText(String(sum(summary, ["pending", "validating"])));
    await expect(cardValue(page, "incident")).toHaveText(String(sum(summary, ["incident"])));
    await expect(cardValue(page, "ready")).toHaveText(String(sum(summary, ["ready_pickup"])));
    await expect(home(page).getByText("Integraciones")).toHaveCount(0);
  });

  test("Empaque (con Despacho): Por empacar y Listas para retiro; no ve Integraciones ni órdenes", async ({ page }) => {
    await openHome(page, "empaque@nexus.local");
    expect(await cardTitles(page)).toEqual(["Por empacar", "Listas para retiro"]);
    expect(await quick(page)).toEqual(["Empaque", "Retiro y entrega"]);
    await expect(cardValue(page, "to-pack")).toHaveText(/^\d+$/);
    for (const hidden of ["Integraciones", "Todas las órdenes", "Solicitudes"]) await expect(home(page).getByText(hidden)).toHaveCount(0);
    await home(page).locator('[data-card="to-pack"]').click();
    await expect(page).toHaveURL(/\/ordenes\/empaque$/, { timeout: 60_000 });
  });

  test("Cliente: solo números de su empresa", async ({ page }) => {
    const mine = (await (await ordersApi("cliente1@nexus.local"))<Summary>("GET", "/v1/orders/summary")).body;
    const all = (await (await ordersApi("admin@nexus.local"))<Summary>("GET", "/v1/orders/summary")).body;
    // El seed tiene órdenes de otras empresas: el total del Cliente es menor que el de WeStorage.
    expect(sum(mine, IN_PROGRESS)).toBeLessThan(sum(all, IN_PROGRESS));
    await openHome(page, "cliente1@nexus.local");
    expect(await cardTitles(page)).toEqual(["Mis órdenes en curso", "Con incidencia", "Solicitudes pendientes de tu empresa"]);
    expect(await quick(page)).toEqual(["Mis órdenes", "Perfiles empresa", "Mis solicitudes", "Productos"]);
    await expect(cardValue(page, "in-progress")).toHaveText(String(sum(mine, IN_PROGRESS)));
    await expect(cardValue(page, "incident")).toHaveText(String(sum(mine, ["incident"])));
    const requests = (await (await coreApi("cliente1@nexus.local"))<{ counts: { pending: number } }>("GET", "/v1/profile-requests?limit=1")).body;
    await expect(cardValue(page, "requests")).toHaveText(String(requests.counts.pending));
    await expect(home(page).getByText("Integraciones")).toHaveCount(0);
  });

  test("Recordatorios: crear con contador, marcar hecha y borrar; otro usuario no la ve", async ({ page, browser }) => {
    const body = `Nota e2e ${Date.now()}`;
    const adminApi = await coreApi("admin@nexus.local");
    const jefeApi = await coreApi("jefe@nexus.local");
    // Notas que haya dejado una corrida anterior interrumpida.
    for (const old of (await adminApi<{ data: Note[] }>("GET", "/v1/me/notes")).body.data.filter((n) => n.body.startsWith("Nota e2e "))) await adminApi("DELETE", `/v1/me/notes/${old.id}`);
    await openHome(page, "admin@nexus.local");
    const panel = home(page).locator(".reminders-panel");
    // Seed: cuatro notas, una hecha (tachada).
    await expect(panel.locator(".reminders-list li.is-done").filter({ hasText: "Revisar integración WooCommerce" })).toBeVisible();

    await panel.getByRole("button", { name: "Nueva nota" }).click();
    await expect(panel.locator("#note-count")).toHaveText("0/140");
    await panel.getByLabel("Nota", { exact: true }).fill("x".repeat(141));
    await expect(panel.locator("#note-count")).toContainText("141/140");
    await expect(panel.getByRole("button", { name: "Guardar" })).toBeDisabled();
    await panel.getByLabel("Nota", { exact: true }).fill(body);
    await panel.getByRole("radio", { name: "Mañana" }).check();
    await panel.getByRole("button", { name: "Guardar" }).click();
    const row = panel.locator(".reminders-list li").filter({ hasText: body });
    await expect(row.locator(".reminders-due")).toHaveText("Mañana");

    // El servidor rechaza más de 140 caracteres aunque se salte el formulario.
    expect((await adminApi("POST", "/v1/me/notes", { body: "x".repeat(141) })).status).toBeGreaterThanOrEqual(400);

    // Otro usuario: ni por API ni en su Home, y no puede cambiarla ni borrarla.
    const note = (await adminApi<{ data: Note[] }>("GET", "/v1/me/notes")).body.data.find((n) => n.body === body)!;
    expect((await jefeApi<{ data: Note[] }>("GET", "/v1/me/notes")).body.data.some((n) => n.body === body)).toBe(false);
    expect((await jefeApi("PATCH", `/v1/me/notes/${note.id}`, { done: true })).status).toBe(404);
    expect((await jefeApi("DELETE", `/v1/me/notes/${note.id}`)).status).toBe(404);
    const other = await browser.newContext();
    const otherPage = await other.newPage();
    await openHome(otherPage, "jefe@nexus.local");
    await expect(home(otherPage).locator(".reminders-list li").first()).toBeVisible();
    await expect(home(otherPage).getByText(body)).toHaveCount(0);
    await other.close();

    await row.getByRole("checkbox").click(); // se marca cuando responde el servidor
    await expect(row).toHaveClass(/is-done/);
    await row.getByRole("button", { name: `Borrar nota: ${body}` }).click();
    await expect(panel.getByText(body)).toHaveCount(0);
    expect((await adminApi<{ data: Note[] }>("GET", "/v1/me/notes")).body.data.some((n) => n.body === body)).toBe(false);
  });

  test("Teléfono (390 px): las tarjetas van una debajo de otra, sin scroll horizontal", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await openHome(page, "admin@nexus.local");
    await expect(home(page).locator(".home-card")).toHaveCount(3);
    const boxes = await home(page).locator(".home-card").evaluateAll((els) => els.map((e) => { const r = e.getBoundingClientRect(); return { x: r.x, y: r.y, bottom: r.bottom }; }));
    expect(new Set(boxes.map((b) => Math.round(b.x))).size).toBe(1);
    expect(boxes[1].y).toBeGreaterThanOrEqual(boxes[0].bottom);
    expect(boxes[2].y).toBeGreaterThanOrEqual(boxes[1].bottom);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  });
});
