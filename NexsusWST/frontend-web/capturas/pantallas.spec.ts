// Capturas de pantalla del sistema para documentación y evidencia (docs/pantallas/).
// No es una prueba funcional: recorre las pantallas con cada perfil del seed y guarda PNG.
//   npx playwright test -c playwright.capturas.config.ts
// Seguridad: nunca se escribe una contraseña en pantalla antes de capturar, no se abre el diálogo
// que revela secretos de Integraciones y, antes de cada captura, se comprueba que no haya campos
// de contraseña con valor ni secretos visibles.
import { mkdirSync } from "node:fs";
import path from "node:path";
import { expect, test, type Browser, type Locator, type Page } from "@playwright/test";
import { login, ordersApi, password } from "../e2e/helpers";

const OUT = path.resolve(__dirname, "..", "..", "docs", "pantallas");
const SIZES = {
  desktop: { width: 1440, height: 900 },
  tablet: { width: 768, height: 1024 },
  movil: { width: 390, height: 844 },
} as const;
type Size = keyof typeof SIZES;
const ALL: Size[] = ["desktop", "tablet", "movil"];
const DESKTOP: Size[] = ["desktop"];

const USERS = {
  admin: "admin@nexus.local",
  jefe: "jefe@nexus.local",
  empaque: "empaque@nexus.local",
  cliente: "cliente1@nexus.local",
} as const;

// Sin el indicador de desarrollo de Next ni cursores parpadeando: solo la aplicación. También se
// oculta la lista de usuarios de prueba de /login (solo existe con NEXT_PUBLIC_SHOW_DEMO_ACCOUNTS
// en desarrollo y muestra la contraseña de prueba): así se ve igual que en producción.
const CLEAN_CSS = "nextjs-portal{display:none!important} *{caret-color:transparent!important} .demo-users{display:none!important}";
const PASSWORDS = [...new Set(Object.values(USERS).map(password))];

// Mientras alguno de estos exista, la pantalla todavía está cargando.
const LOADING = [
  ".orders-skeleton", ".detail-skeleton", ".packing-skeleton", '[aria-busy="true"]',
  '[aria-label^="Cargando"]',
].join(", ");

async function newSession(browser: Browser, user?: string) {
  const context = await browser.newContext({ viewport: SIZES.desktop, locale: "es-CL", timezoneId: "America/Santiago" });
  await context.addInitScript((css) => {
    const add = () => { const s = document.createElement("style"); s.textContent = css; document.head.appendChild(s); };
    if (document.head) add(); else document.addEventListener("DOMContentLoaded", add);
  }, CLEAN_CSS);
  const page = await context.newPage();
  if (user) await login(page, user);
  return page;
}

// Espera a que no quede nada cargando: red quieta, sin skeletons ni textos «Cargando…», fuentes listas.
async function settle(page: Page) {
  await page.waitForLoadState("networkidle", { timeout: 20_000 }).catch(() => {});
  await expect(page.locator(LOADING)).toHaveCount(0, { timeout: 60_000 });
  await expect(page.getByText(/^Cargando/)).toHaveCount(0, { timeout: 60_000 });
  await page.evaluate(() => document.fonts.ready);
  // Las imágenes con carga diferida (logos de couriers) solo cargan al entrar al viewport: se
  // recorre la página y se espera a que todas terminen antes de la captura de página completa.
  await page.evaluate(async () => {
    const root = document.scrollingElement!;
    for (let y = 0; y < root.scrollHeight; y += window.innerHeight / 2) {
      window.scrollTo(0, y);
      await new Promise((r) => setTimeout(r, 60));
    }
    window.scrollTo(0, 0);
    document.querySelectorAll("img").forEach((img) => { img.loading = "eager"; });
  });
  await page.waitForFunction(() => [...document.images].every((img) => img.complete), null, { timeout: 30_000 });
  await page.waitForTimeout(600); // transiciones de modales y paneles
}

// Nada sensible a la vista: campos de contraseña vacíos y ningún secreto revelado.
async function assertNoSecrets(page: Page) {
  const filled = await page.locator('input[type="password"]').evaluateAll((els) => els.filter((e) => (e as HTMLInputElement).value).length);
  expect(filled, "campo de contraseña con valor").toBe(0);
  await expect(page.locator(".secret-label")).toHaveCount(0); // diálogo «Secreto nuevo» de Integraciones
  const text = await page.locator("body").innerText(); // solo lo visible
  expect(text).not.toMatch(/eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}/); // ningún JWT
  for (const pw of PASSWORDS) expect(text.includes(pw), "contraseña de prueba a la vista").toBe(false);
}

async function shot(page: Page, dir: string, name: string, size: Size, fullPage = true) {
  mkdirSync(path.join(OUT, dir), { recursive: true });
  await assertNoSecrets(page);
  await page.screenshot({ path: path.join(OUT, dir, `${name}_${size}.png`), fullPage, animations: "disabled", caret: "hide" });
}

// Abre una URL en cada tamaño, espera a `ready` y captura la página completa.
async function capture(page: Page, dir: string, name: string, url: string, ready: (p: Page) => Locator, sizes: Size[] = DESKTOP) {
  for (const size of sizes) {
    await page.setViewportSize(SIZES[size]);
    await page.goto(url);
    await expect(ready(page).first()).toBeVisible({ timeout: 90_000 });
    await settle(page);
    await shot(page, dir, name, size);
  }
  await page.setViewportSize(SIZES.desktop);
}

const heading = (text: string | RegExp) => (p: Page) => p.locator(".page-header h1, h1").filter({ hasText: text });
const ordersRows = (p: Page) => p.locator(".orders-list tbody tr");

test.describe.serial("Capturas de pantalla", () => {
  test("00 · Login", async ({ browser }) => {
    const page = await newSession(browser);
    for (const size of ALL) {
      await page.setViewportSize(SIZES[size]);
      await page.goto("/login");
      await expect(page.getByRole("button", { name: "Ingresar" })).toBeVisible({ timeout: 90_000 });
      await settle(page);
      await shot(page, "00-login", "login-inicio", size);
      // Formulario de Keycloak vacío: no se escribe la contraseña.
      await page.getByRole("button", { name: "Ingresar" }).click();
      await page.waitForURL(/\/auth\/realms\//);
      await expect(page.locator('input[name="username"]')).toBeVisible();
      await settle(page);
      await shot(page, "00-login", "login-keycloak", size);
    }
    await page.context().close();
  });

  test("01 · Home por perfil", async ({ browser }) => {
    for (const [key, email] of Object.entries(USERS)) {
      const page = await newSession(browser, email);
      const sizes = key === "admin" || key === "empaque" ? ALL : DESKTOP;
      await capture(page, "01-home", `home-${key}`, "/", (p) => p.locator(".home-view .home-card"), sizes);
      await page.context().close();
    }
  });

  test("02 · Órdenes (Administrador y Jefe de Operaciones)", async ({ browser }) => {
    const dir = "02-ordenes";
    const page = await newSession(browser, USERS.admin);
    await capture(page, dir, "ordenes-bandeja-admin", "/ordenes/todas/listado", ordersRows, ALL);

    // Filtros abiertos.
    await page.goto("/ordenes/todas/listado");
    await expect(ordersRows(page).first()).toBeVisible({ timeout: 90_000 });
    await settle(page);
    await page.getByRole("button", { name: /^Filtros/ }).click();
    await expect(page.getByRole("button", { name: "Aplicar" })).toBeVisible();
    await settle(page);
    await shot(page, dir, "ordenes-filtros", "desktop", false);
    await page.keyboard.press("Escape");

    // KPI «Incidencias» aplicado como filtro.
    await capture(page, dir, "ordenes-filtro-kpi-incidencias", "/ordenes/todas/listado?status=incident", ordersRows);

    // Detalle con historial: la orden de la bandeja con más eventos.
    const api = await ordersApi(USERS.admin);
    const list = (await api<{ data: { id: number }[] }>("GET", "/v1/orders?limit=20")).body.data;
    let detailId = list[0].id, most = -1;
    for (const o of list) {
      const n = (await api<{ events: unknown[] }>("GET", `/v1/orders/${o.id}`)).body.events.length;
      if (n > most) [detailId, most] = [o.id, n];
    }
    await capture(page, dir, "ordenes-detalle", `/ordenes/todas/listado?order=${detailId}`, (p) => p.locator(".orders-detail-panel"));
    await page.locator(".orders-detail-panel").getByRole("button", { name: "Ver todo" }).click();
    await expect(page.locator(".history-modal")).toBeVisible();
    await settle(page);
    await shot(page, dir, "ordenes-detalle-historial", "desktop", false);
    await page.keyboard.press("Escape");

    // Ingreso manual: formulario «Nueva orden» (no se envía).
    await page.goto("/ordenes/todas/listado");
    await expect(ordersRows(page).first()).toBeVisible({ timeout: 90_000 });
    await settle(page);
    await page.getByRole("button", { name: "Nueva orden" }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await settle(page);
    await shot(page, dir, "ordenes-ingreso-manual", "desktop", false);
    await page.keyboard.press("Escape");

    // Ingreso por Excel: menú y diálogo de importación (no se sube archivo).
    await page.goto("/ordenes/todas/listado");
    await expect(ordersRows(page).first()).toBeVisible({ timeout: 90_000 });
    await settle(page);
    await page.getByRole("button", { name: "Excel acciones" }).click();
    await expect(page.locator(".dropdown-menu")).toBeVisible();
    await shot(page, dir, "ordenes-excel-acciones", "desktop", false);
    await page.locator(".dropdown-menu button", { hasText: "Importar órdenes (Excel)" }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await settle(page);
    await shot(page, dir, "ordenes-ingreso-excel", "desktop", false);
    await page.keyboard.press("Escape");

    // Registro histórico y registro de acciones.
    await capture(page, dir, "registro-historico", "/ordenes/registro", (p) => p.locator(".registry-list tbody tr"));
    await page.locator(".registry-list tbody tr").first().click();
    await expect(page.locator(".registry-panel")).toBeVisible();
    await settle(page);
    await shot(page, dir, "registro-historico-detalle", "desktop");
    await capture(page, dir, "registro-acciones", "/ordenes/registro?tab=acciones", (p) => p.locator(".activity-list tbody tr"));
    await page.context().close();

    // Jefe de Operaciones: bandeja, asignación de órdenes y retiro y entrega.
    const jefe = await newSession(browser, USERS.jefe);
    await capture(jefe, dir, "ordenes-bandeja-jefe", "/ordenes/todas/listado", ordersRows);
    await capture(jefe, dir, "ordenes-asignacion", "/ordenes/todas/asignacion", ordersRows);
    // Con un courier elegido: sin él, la lista de retiro queda en «Elige un courier».
    await capture(jefe, dir, "retiro-entrega", "/ordenes/retiro?courier=starken", (p) => p.locator('.dispatch-list[aria-busy="false"] .dispatch-table tbody tr'), ALL);
    for (const size of ["desktop", "movil"] as Size[]) {
      await jefe.setViewportSize(SIZES[size]);
      await jefe.goto("/ordenes/retiro?courier=mercadoenvios");
      await expect(jefe.locator('.dispatch-list[aria-busy="false"]')).toBeVisible({ timeout: 90_000 });
      await settle(jefe);
      await jefe.getByRole("button", { name: /^Ver manifiesto abierto #/ }).click();
      await expect(jefe.locator(".manifest-modal")).toBeVisible();
      await settle(jefe);
      await shot(jefe, dir, "retiro-entrega-manifiesto", size, false);
      await jefe.keyboard.press("Escape");
    }
    await jefe.setViewportSize(SIZES.desktop);

    // Seguimiento del courier en el detalle de una orden entregada.
    await jefe.goto("/ordenes/todas/listado?status=handed_over");
    await jefe.locator(".orders-list tbody tr", { hasText: "48257769230" }).click();
    const tracking = jefe.locator(".order-detail section", { has: jefe.getByRole("heading", { name: "Seguimiento" }) });
    await expect(tracking.locator("tbody tr").first()).toBeVisible({ timeout: 60_000 });
    await settle(jefe);
    // Solo el panel de detalle: a página completa el sidebar fijo queda a media altura.
    await tracking.scrollIntoViewIfNeeded();
    await assertNoSecrets(jefe);
    await jefe.locator(".orders-detail-panel").screenshot({ path: path.join(OUT, dir, "ordenes-seguimiento_desktop.png"), animations: "disabled", caret: "hide" });
    await jefe.context().close();
  });

  test("02 · Empaque", async ({ browser }) => {
    const dir = "02-ordenes";
    const page = await newSession(browser, USERS.empaque);
    await capture(page, dir, "empaque", "/ordenes/empaque", (p) => p.locator(".packing-card"), ALL);
    // Orden en empaque tomada por el usuario de Empaque. La 20007 (demo del video) tiene productos
    // con descripción; las de TecnoPlus salen como «Producto sin descripción».
    const api = await ordersApi(USERS.empaque);
    const packing = (await api<{ data: { id: number; externalRef: string; claimedByName?: string }[] }>("GET", "/v1/orders?status=packing&limit=20")).body.data;
    const mine = packing.filter((o) => o.claimedByName === "Empaque Demo");
    const order = mine.find((o) => o.externalRef === "20007") ?? mine[0] ?? packing[0];
    await capture(page, dir, "empaque-orden", `/ordenes/empaque/${order.id}`, (p) => p.locator(".page-header h1, h1"), ALL);
    await page.context().close();
  });

  test("02 · Vista del Cliente", async ({ browser }) => {
    const dir = "02-ordenes";
    const page = await newSession(browser, USERS.cliente);
    await capture(page, dir, "ordenes-bandeja-cliente", "/ordenes/todas/listado", ordersRows);
    // Sin «Registro histórico» del Cliente: solo lista canceladas o cerradas hace más de un año, y
    // la empresa de cliente1 no tiene ninguna en el seed (saldría vacío).
    await page.context().close();
  });

  test("03 · Clientes", async ({ browser }) => {
    const dir = "03-clientes";
    const cliente = await newSession(browser, USERS.cliente);
    await capture(cliente, dir, "clientes-perfiles-empresa", "/clientes/perfiles-empresa", (p) => p.locator(".clients-list tbody tr, table tbody tr"));
    await cliente.context().close();

    const admin = await newSession(browser, USERS.admin);
    for (const tab of ["perfiles", "empresas", "solicitudes"]) {
      await capture(admin, dir, `clientes-administracion-${tab}`, `/clientes/administracion?tab=${tab}`, (p) => p.locator("table tbody tr, .empty-state, p.pad"));
    }
    await admin.context().close();
  });

  test("04 · Configuración", async ({ browser }) => {
    const dir = "04-configuracion";
    const page = await newSession(browser, USERS.admin);
    await capture(page, dir, "configuracion-usuarios", "/configuracion/usuarios", (p) => p.locator("table tbody tr"));
    await capture(page, dir, "configuracion-usuarios-solicitudes", "/configuracion/usuarios?tab=solicitudes", (p) => p.locator("table tbody tr, p.pad"));
    await page.context().close();
  });

  test("05 · Productos", async ({ browser }) => {
    const dir = "05-productos";
    const admin = await newSession(browser, USERS.admin);
    await capture(admin, dir, "productos-admin", "/productos", (p) => p.locator("table tbody tr"), ALL);
    await admin.context().close();
    const cliente = await newSession(browser, USERS.cliente);
    await capture(cliente, dir, "productos-cliente", "/productos", (p) => p.locator("table tbody tr"));
    await cliente.context().close();
  });

  test("06 · Integraciones", async ({ browser }) => {
    const dir = "06-integraciones";
    const page = await newSession(browser, USERS.admin);
    await capture(page, dir, "integraciones-catalogo", "/integraciones?tab=integraciones", heading("Integraciones"));
    await capture(page, dir, "integraciones-conexiones", "/integraciones?tab=clientes", (p) => p.locator(".connections-list tbody tr"));
    // Detalle de una conexión: el secreto se ve enmascarado (••••); nunca se rota ni se crea.
    await page.locator(".connections-list tbody tr").first().click();
    await expect(page.locator(".connection-detail")).toBeVisible();
    await expect(page.getByLabel("Secreto oculto")).toBeVisible();
    await settle(page);
    await shot(page, dir, "integraciones-conexion-detalle", "desktop", false);
    await page.context().close();
  });

  // Al final, porque crea una incidencia (el seed se recarga en la próxima corrida).
  test("02 · Campana de avisos", async ({ browser }) => {
    const page = await newSession(browser, USERS.jefe);
    await page.goto("/ordenes/todas/listado");
    await expect(ordersRows(page).first()).toBeVisible({ timeout: 90_000 });
    await expect(page.getByText("En vivo")).toBeVisible({ timeout: 60_000 });
    await settle(page);
    const api = await ordersApi(USERS.jefe);
    const picked = (await api<{ data: { id: number }[] }>("GET", "/v1/orders?status=picked&limit=1")).body.data[0];
    expect((await api("POST", `/v1/orders/${picked.id}/incident`, { kind: "otro", reason: "Producto dañado en bodega (demo)" })).status).toBe(200);
    const bell = page.locator(".notification-bell > button");
    await expect(bell).toHaveAttribute("aria-label", /sin leer/, { timeout: 30_000 });
    await settle(page);
    await bell.click();
    await expect(page.getByRole("region", { name: "Avisos" })).toBeVisible();
    await page.waitForTimeout(400);
    await shot(page, "02-ordenes", "ordenes-campana-avisos", "desktop", false);
    await page.context().close();
  });
});
