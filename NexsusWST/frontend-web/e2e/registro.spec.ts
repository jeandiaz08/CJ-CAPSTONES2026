// Registro histórico · «Registro órdenes» (Tarea 06, Fase 1). Requiere el seed de demostración:
// 48257769242 (Entregada hace 14 meses), 48257769243 (Anulada hace 13), 48257769244 (Incidencia hace 15)
// entran por tiempo; 48257769245 (Entregada hace 2 meses) es el control y sigue en la bandeja.
import { expect, test, type Page } from "@playwright/test";
import { login, ordersApi } from "./helpers";

type Entry = { id: number; externalRef: string; status: string; motive: string; stale: boolean; previousStatusLabel?: string; closedBy: { name: string; role?: string; kind: string } };
type Order = { id: number; externalRef: string; status: string };

const STALE = ["48257769242", "48257769243", "48257769244"];
const CONTROL = "48257769245";

test.describe.serial("Registro histórico", () => {
  let page: Page;
  let jefe: Awaited<ReturnType<typeof ordersApi>>;

  test.beforeAll(async ({ browser }) => {
    jefe = await ordersApi("jefe@nexus.local");
    page = await browser.newPage();
    await login(page, "jefe@nexus.local");
  });
  test.afterAll(async () => page.close());

  test("API: la regla manda y la bandeja excluye lo mismo", async () => {
    const reg = (await jefe<{ data: Entry[]; total: number }>("GET", "/v1/orders/registry?limit=200")).body;
    const refs = reg.data.map((e) => e.externalRef);
    for (const ref of STALE) expect(refs, "recarga el seed de demostración").toContain(ref);
    expect(refs).not.toContain(CONTROL);
    const delivered = reg.data.find((e) => e.externalRef === "48257769242")!;
    expect(delivered).toMatchObject({ status: "handed_over", stale: true, motive: "Más de 1 año sin movimiento · Retiro confirmado", previousStatusLabel: "Lista para retiro" });
    expect(delivered.closedBy).toMatchObject({ name: "Jefe de Operaciones Demo", role: "jefe_operaciones", kind: "human" });
    expect(reg.data.find((e) => e.externalRef === "48257769243")!.closedBy).toMatchObject({ name: "Sistema", kind: "system" });
    expect(reg.data.filter((e) => e.status === "cancelled").every((e) => !e.stale)).toBe(true);

    const tray = (await jefe<{ data: Order[] }>("GET", "/v1/orders?limit=200")).body.data.map((o) => o.externalRef);
    for (const ref of refs) expect(tray).not.toContain(ref);
    expect(tray).toContain(CONTROL);
  });

  test("API: Empaque recibe 403; el Cliente ve solo lo de su empresa", async () => {
    const empaque = await ordersApi("empaque@nexus.local");
    expect((await empaque("GET", "/v1/orders/registry")).status).toBe(403);
    expect((await empaque("GET", "/v1/orders/registry/export")).status).toBe(403);

    // El Cliente 1 crea y cancela una orden propia: entra al registro de inmediato, cerrada por él.
    const cliente1 = await ordersApi("cliente1@nexus.local");
    const ref = `REG-E2E-${Date.now()}`;
    const created = await cliente1<Order>("POST", "/v1/orders", { companyId: 1, externalRef: ref, courier: "starken", recipientName: "Prueba Registro", addressLine: "Calle 1", commune: "Santiago", items: [{ sku: "SKU-001", quantity: 1 }] });
    expect(created.status).toBe(201);
    expect((await cliente1("POST", `/v1/orders/${created.body.id}/cancel`, { reason: "Prueba del registro (e2e)" })).status).toBe(200);

    const own = (await cliente1<{ data: Entry[] }>("GET", `/v1/orders/registry?q=${ref}`)).body.data;
    expect(own).toHaveLength(1);
    expect(own[0]).toMatchObject({ status: "cancelled", stale: false, motive: "Prueba del registro (e2e)" });
    expect(own[0].closedBy).toMatchObject({ name: "Cliente Demo Uno", role: "cliente" });
    expect((await cliente1<{ data: Entry[] }>("GET", "/v1/orders/registry?limit=200")).body.data.every((e) => !STALE.includes(e.externalRef))).toBe(true);
    const cliente2 = await ordersApi("cliente2@nexus.local");
    expect((await cliente2<{ total: number }>("GET", `/v1/orders/registry?q=${ref}`)).body.total).toBe(0);
    expect((await cliente2("GET", "/v1/orders/registry?company_id=1")).status).toBe(403);
  });

  test("API /v1/activity: solo Admin y Jefe, por apartado, sin secretos ni nada de más de 1 año", async () => {
    type Item = { occurredAt: string; module: string; action: string; actor: { name: string; kind: string }; entity: { type: string; label: string }; details: Record<string, unknown> };
    for (const email of ["cliente1@nexus.local", "empaque@nexus.local"]) expect((await (await ordersApi(email))("GET", "/v1/activity")).status).toBe(403);
    expect((await jefe("GET", "/v1/activity?module=config")).status).toBe(422);
    const admin = await ordersApi("admin@nexus.local");
    const yearAgo = Date.now() - 365 * 24 * 3600 * 1000;
    for (const section of ["orders", "packing", "dispatch", "users"]) {
      const res = await admin<{ items: Item[]; total: number }>("GET", `/v1/activity?module=${section}&limit=200`);
      expect(res.status).toBe(200);
      expect(res.body.total, `${section} tiene acciones en el seed`).toBeGreaterThan(0);
      expect(res.body.items.every((i) => i.module === section && Date.parse(i.occurredAt) >= yearAgo)).toBe(true);
      // Sin credenciales en los datos de cada acción (el nombre de una acción, como «password_email_sent», no es un secreto).
      expect(JSON.stringify(res.body.items.map((i) => [i.details, (i as Item & { detailLines?: string[] }).detailLines]))).not.toMatch(/keycloakId|password|token|secret/i);
    }
    // El seed tiene una entrega de hace 14 meses: el registro de acciones ya no la devuelve.
    const old = (await jefe<{ items: Item[] }>("GET", "/v1/activity?module=orders&q=48257769242&limit=200")).body.items;
    expect(old.map((i) => i.action)).toEqual([]);
    const system = (await jefe<{ items: Item[] }>("GET", "/v1/activity?module=orders&kind=system&limit=50")).body.items;
    expect(system.length).toBeGreaterThan(0);
    expect(system.every((i) => i.actor.kind === "system" && i.actor.name === "Sistema")).toBe(true);
  });

  test("Jefe: menú, pestañas, tabla, panel y filtros", async () => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/ordenes/historial");
    await expect(page).toHaveURL(/\/ordenes\/registro$/, { timeout: 90_000 }); // la primera visita compila la ruta
    await page.locator(".sidebar-nav .nav-link", { hasText: "Órdenes" }).click();
    await expect(page.locator('.nav-submenu a[href="/ordenes/registro"]')).toHaveText(/Registro histórico/);
    await expect(page.locator(".page-header h1")).toHaveText("Registro histórico");
    await expect(page.locator(".view-tabs a")).toHaveText(["Registro órdenes", "Registro de acciones"]);
    await expect(page.locator(".registry-list thead th")).toHaveText(["", "N° orden", "Cliente", "Cerrada por", "Motivo", "Estado", "Fecha"], { timeout: 60_000 });

    const row = page.locator(".registry-list tbody tr", { hasText: "48257769242" });
    await expect(row).toContainText("Jefe de Operaciones Demo");
    await expect(row.locator("td").nth(4)).toHaveAttribute("title", "Más de 1 año sin movimiento · Retiro confirmado");
    await expect(row.locator(".badge")).toHaveText("Entregada al courier");
    for (const ref of ["48257769243", "48257769244"]) await expect(page.locator(".registry-list tbody tr", { hasText: ref }).locator(".badge")).toHaveClass(/tone-danger/);
    await expect(page.locator(".registry-list tbody tr", { hasText: CONTROL })).toHaveCount(0);

    await row.click();
    await expect(page).toHaveURL(/order=\d+/);
    const panel = page.locator(".registry-panel");
    await expect(panel.locator("h2")).toHaveText("Registro orden #48257769242");
    const facts = Object.fromEntries(await panel.locator(".registry-facts > div").evaluateAll((divs) => divs.map((d) => [d.querySelector("dt")!.textContent, d.querySelector("dd")!.textContent])));
    expect(facts).toMatchObject({ "Cerrada por": "Jefe de Operaciones Demo · Jefe de Operaciones", "Estaba en": "Lista para retiro", Courier: "Mercado Envíos", Motivo: "Más de 1 año sin movimiento · Retiro confirmado" });
    await expect(panel.locator(".event-timeline li")).toHaveCount(2);
    await expect(panel.locator(".event-timeline li").last()).toContainText("Entregada");

    // Un enlace con ?order= abre el registro aunque la orden no esté en la página.
    const id = Number(new URL(page.url()).searchParams.get("order"));
    await page.goto(`/ordenes/registro?limit=20&page=0&status=cancelled&order=${id}`);
    await expect(page.locator(".registry-panel h2")).toHaveText("Registro orden #48257769242");

    await page.goto("/ordenes/registro");
    await page.getByPlaceholder("Buscar n° de orden").fill("48257769243");
    await expect(page.locator(".registry-list tbody tr")).toHaveCount(1);
    await expect(page).toHaveURL(/q=48257769243/);
    await page.getByRole("button", { name: "Limpiar filtros" }).click();
    await expect(page).not.toHaveURL(/q=/);
    await page.getByRole("button", { name: /^Filtros/ }).click();
    await page.getByRole("checkbox", { name: "Incidencia" }).check();
    await page.getByRole("button", { name: "Aplicar" }).click();
    await expect(page).toHaveURL(/status=incident/);
    await expect(page.locator(".registry-list tbody tr")).toHaveCount(1);
    await expect(page.locator(".registry-list tbody tr")).toContainText("48257769244");

    await page.getByRole("button", { name: "Excel acciones" }).click();
    await expect(page.locator(".dropdown-menu button")).toHaveText(["Exportar seleccionadas", "Exportar por filtros", "Exportar masivo"]);
    const inside = async () => page.locator(".dropdown-menu").evaluate((m) => { const b = m.getBoundingClientRect(); return b.left >= 0 && b.right <= window.innerWidth; });
    expect(await inside(), "el menú de Excel queda dentro de la pantalla").toBe(true);
    const [download] = await Promise.all([page.waitForEvent("download"), page.locator(".dropdown-menu button", { hasText: "Exportar masivo" }).click()]);
    expect(download.suggestedFilename()).toMatch(/^registro-ordenes-.*\.xlsx$/);
    await page.getByRole("button", { name: "Excel acciones" }).click();
    await page.locator(".dropdown-menu button", { hasText: "Exportar por filtros" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Desde *").fill("2024-01-01");
    await dialog.getByLabel("Hasta *").fill("2030-12-31");
    await dialog.getByRole("radio", { name: "csv" }).check();
    const [csv] = await Promise.all([page.waitForEvent("download"), dialog.getByRole("button", { name: "Exportar" }).click()]);
    expect(csv.suggestedFilename()).toMatch(/^registro-ordenes-.*\.csv$/);
  });

  test("Teléfono: el registro abre en un modal de pantalla completa, sin scroll de la página", async () => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/ordenes/registro");
    await page.locator(".registry-list tbody tr", { hasText: "48257769244" }).locator(".order-number").click();
    const modal = page.locator(".modal.registry-modal");
    await expect(modal).toBeVisible();
    await expect(modal).toContainText("Registro orden #48257769244");
    const box = (await modal.boundingBox())!;
    expect([Math.round(box.width), Math.round(box.height)]).toEqual([390, 844]);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await modal.getByRole("button", { name: /Cerrar/ }).first().click();
    await expect(modal).toHaveCount(0);
    await expect(page).not.toHaveURL(/order=/);
    await page.getByRole("button", { name: "Excel acciones" }).click();
    expect(await page.locator(".dropdown-menu").evaluate((m) => { const b = m.getBoundingClientRect(); return b.left >= 0 && b.right <= window.innerWidth; })).toBe(true);
  });

  test("Registro de acciones: chips, título con total, columnas por apartado, fila desplegable y Excel", async () => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/ordenes/registro");
    await page.locator(".view-tabs a", { hasText: "Registro de acciones" }).click();
    await expect(page).toHaveURL(/tab=acciones/);
    const chips = page.locator(".activity-chips .packing-chip");
    await expect(chips).toHaveText(["Órdenes", "Empaque", "Retiro y entrega", "Usuarios", "Productos"], { timeout: 60_000 });
    await expect(chips.first()).toHaveAttribute("aria-pressed", "true");
    await expect(page.locator(".activity-title")).toHaveText(/^Órdenes · \d+ acci(ón|ones)$/);
    await expect(page.locator(".activity-list thead th")).toHaveText(["Fecha y hora", "Actor", "Perfil", "Humano/Sistema", "Acción", "N° orden", "Cliente", "Estado anterior → nuevo", "Motivo u observación", "Detalle"]);
    const first = page.locator(".activity-list tbody tr").first();
    await first.click();
    await expect(page.locator(".activity-detail-row")).toHaveCount(1);
    await expect(page.locator(".activity-detail-row")).toContainText("Acción");
    const [download] = await Promise.all([page.waitForEvent("download"), page.getByRole("button", { name: "Exportar registro" }).click()]);
    expect(download.suggestedFilename()).toMatch(/^registro-acciones-orders-.*\.xlsx$/);

    // Solo procesos: todas las filas dicen «Sistema».
    await page.getByRole("button", { name: /^Filtros/ }).click();
    await page.getByLabel("Humano/Sistema").selectOption("system");
    await page.getByRole("button", { name: "Aplicar" }).click();
    await expect(page).toHaveURL(/kind=system/);
    await expect(page.locator(".activity-list tbody tr td:nth-child(4)").first()).toHaveText("Sistema");
    expect(new Set(await page.locator(".activity-list tbody tr td:nth-child(4)").allTextContents())).toEqual(new Set(["Sistema"]));

    // Retiro y entrega: el seed deja un manifiesto abierto; su número filtra el apartado.
    await page.getByRole("button", { name: "Limpiar filtros" }).click();
    await expect(page).not.toHaveURL(/kind=/);
    await chips.nth(2).click();
    await expect(page).toHaveURL(/section=dispatch/);
    await expect(page.locator(".activity-list thead th")).toHaveText(["Fecha y hora", "Actor", "Perfil", "Acción", "N° manifiesto", "Courier", "Órdenes", "Bultos", "Quien retira", "RUT", "Patente", "Empresa del transporte", "Observación"]);
    const manifest = page.locator(".activity-list tbody tr", { hasText: "Creó manifiesto" }).first().locator("a.activity-link");
    const number = (await manifest.textContent())!;
    await manifest.click();
    await expect(page).toHaveURL(new RegExp(`q=%23${number.slice(1)}`));
    await expect(page.locator(".activity-list tbody tr", { hasText: "Creó manifiesto" })).toHaveCount(1);

    await chips.nth(3).click();
    await expect(page.locator(".activity-title")).toHaveText(/^Usuarios · /);
    await expect(page.locator(".activity-list thead th, .activity-list p.pad").first()).toBeVisible();
  });

  test("N° de orden: abre el detalle de Todas las órdenes; /ordenes/[id] redirige (o al registro)", async () => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto("/ordenes/registro?tab=acciones&section=orders");
    const link = page.locator(".activity-list tbody tr", { hasText: "48257769201" }).first().locator("a.activity-link");
    await link.click();
    await expect(page).toHaveURL(/\/ordenes\/todas\/listado$/, { timeout: 60_000 });
    await expect(page.locator(".orders-detail-panel h2")).toHaveText("Detalle de orden #48257769201");

    const registry = (await jefe<{ data: Entry[] }>("GET", "/v1/orders/registry?q=48257769242")).body.data[0];
    await page.goto(`/ordenes/${registry.id}`);
    await expect(page).toHaveURL(`/ordenes/registro?order=${registry.id}`, { timeout: 60_000 });
    await expect(page.locator(".registry-panel h2")).toHaveText("Registro orden #48257769242");
  });

  test("Teléfono: los chips del registro de acciones se desplazan en una fila, sin scroll de la página", async () => {
    await page.setViewportSize({ width: 360, height: 800 });
    await page.goto("/ordenes/registro?tab=acciones&section=dispatch");
    const chips = page.locator(".activity-chips");
    await expect(chips).toBeVisible({ timeout: 60_000 });
    expect(await chips.evaluate((c) => [getComputedStyle(c).flexWrap, getComputedStyle(c).overflowX])).toEqual(["nowrap", "auto"]);
    const tops = await page.locator(".activity-chips .packing-chip").evaluateAll((els) => els.map((e) => Math.round(e.getBoundingClientRect().top)));
    expect(new Set(tops).size).toBe(1);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  });

  test("Cliente: solo «Registro órdenes»; Empaque no tiene el submenú y vuelve al Home", async ({ browser }) => {
    const cliente = await browser.newPage();
    await login(cliente, "cliente1@nexus.local");
    await cliente.goto("/ordenes/registro?tab=acciones");
    await expect(cliente.locator(".view-tabs a")).toHaveText(["Registro órdenes"]);
    await expect(cliente.locator(".registry-list thead th").nth(3)).toHaveText("Cerrada por");
    await cliente.close();

    const empaque = await browser.newPage();
    await login(empaque, "empaque@nexus.local");
    await expect(empaque.locator('a[href="/ordenes/registro"]')).toHaveCount(0);
    await empaque.goto("/ordenes/registro");
    await expect(empaque).toHaveURL(/\/$/);
    await empaque.close();
  });
});
