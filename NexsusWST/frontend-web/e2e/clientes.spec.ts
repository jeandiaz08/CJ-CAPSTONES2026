// Clientes (Tarea 07). Fase 1: menú, rutas y permiso «Gestionar perfiles».
// Usa el seed de desarrollo: admin@nexus.local con el permiso y admin.lectura@nexus.local sin él.
import { expect, test, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { coreApi, login, mailSubjects, ordersApi } from "./helpers";

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

// ---------------------------------------------------------------------------------------------
// Fase 2: Administración clientes › Empresas. Las empresas de prueba se llaman «E2E · …»: el seed
// de demostración las borra al recargar (global-setup).
// ---------------------------------------------------------------------------------------------
type Company = { id: number; name: string; taxId: string; waveCode: string | null; active: boolean; wmsCode?: string };
type Activity = { items: { action: string; actionLabel: string; entity: { type: string; label: string }; detailLines: string[] }[] };

// RUT válido al azar (módulo 11), con puntos y guion.
function randomRut() {
  const body = 70_000_000 + Math.floor(Math.random() * 9_000_000);
  let sum = 0, mul = 2;
  for (const d of String(body).split("").reverse()) { sum += Number(d) * mul; mul = mul === 7 ? 2 : mul + 1; }
  const r = 11 - (sum % 11);
  const dv = r === 11 ? "0" : r === 10 ? "K" : String(r);
  return { formatted: `${String(body).replace(/\B(?=(\d{3})+(?!\d))/g, ".")}-${dv}`, plain: `${body}${dv}` };
}

test.describe.serial("Clientes · Fase 2: Empresas", () => {
  const name = `E2E · Empresa ${Date.now()}`;
  const rut = randomRut();
  let created: Company;

  test("API: crear, RUT repetido, permisos, desactivar con motivo y registro de acciones", async () => {
    const admin = await coreApi("admin@nexus.local");
    const reader = await coreApi("admin.lectura@nexus.local");

    expect((await reader("POST", "/v1/companies", { name, taxId: rut.formatted, waveCode: null })).status).toBe(403);
    const invalid = await admin<{ fields: { field: string }[] }>("POST", "/v1/companies", { name: "", taxId: "76.123.456-7", email: "x", waveCode: null });
    expect(invalid.status).toBe(422);
    expect(invalid.body.fields.map((f) => f.field).sort()).toEqual(["email", "name", "taxId"]);

    const res = await admin<Company>("POST", "/v1/companies", { name, taxId: rut.plain, email: "e2e@empresa.cl", phone: "+56 9 1234 5678", waveCode: null, wmsCode: "E2E" });
    expect(res.status).toBe(201);
    created = res.body;
    expect(created).toMatchObject({ taxId: rut.formatted, waveCode: null, active: true, wmsCode: "E2E" });

    // El mismo RUT con otro formato también se rechaza (índice único sobre el RUT normalizado).
    const dup = await admin<{ fields: { field: string }[] }>("POST", "/v1/companies", { name: "Otra", taxId: rut.plain.toLowerCase(), waveCode: null });
    expect(dup.status).toBe(409);
    expect(dup.body.fields[0].field).toBe("taxId");

    expect((await admin("POST", `/v1/companies/${created.id}/disable`, { reason: "no" })).status).toBe(422);
    const off = await admin<Company>("POST", `/v1/companies/${created.id}/disable`, { reason: "Término de contrato E2E" });
    expect(off.status).toBe(200);
    expect(off.body.active).toBe(false);
    expect((await admin("POST", `/v1/companies/${created.id}/disable`, { reason: "Otra vez" })).status).toBe(409);
    expect((await reader<{ data: Company[] }>("GET", `/v1/companies?status=inactive&q=${encodeURIComponent(name)}`)).body.data.map((c) => c.id)).toEqual([created.id]);
    expect((await admin("POST", `/v1/companies/${created.id}/enable`)).status).toBe(200);

    // Las acciones aparecen solas en Registro de acciones › Usuarios.
    const orders = await ordersApi("admin@nexus.local");
    const log = (await orders<Activity>("GET", `/v1/activity?module=users&q=${encodeURIComponent(name)}&limit=50`)).body.items;
    const mine = log.filter((i) => i.entity.label === name);
    expect(mine.map((i) => i.action).sort()).toEqual(["company_created", "company_disabled", "company_enabled"]);
    const disabled = mine.find((i) => i.action === "company_disabled")!;
    expect(disabled).toMatchObject({ actionLabel: "Empresa desactivada", entity: { type: "company" } });
    expect(disabled.detailLines).toEqual(["Estado: Activa → Inactiva", "Motivo: Término de contrato E2E"]);
  });

  test("H-03 criterio 3: con la empresa desactivada, su Cliente deja de operar; el historial se conserva", async () => {
    const admin = await coreApi("admin@nexus.local");
    const demo = (await admin<{ data: Company[] }>("GET", "/v1/companies?q=11.111.111-1")).body.data.find((c) => c.taxId === "11.111.111-1")!;
    try {
      expect((await admin("POST", `/v1/companies/${demo.id}/disable`, { reason: "Prueba E2E de bloqueo" })).status).toBe(200);
      const me = await (await coreApi("cliente1@nexus.local"))<{ error: string }>("GET", "/v1/auth/me");
      expect(me.status).toBe(403);
      expect(me.body.error).toBe("company_disabled");
    } finally {
      await admin("POST", `/v1/companies/${demo.id}/enable`, { reason: "Fin de la prueba E2E" });
    }
    expect((await (await coreApi("cliente1@nexus.local"))("GET", "/v1/auth/me")).status).toBe(200);
  });

  test("UI: el Administrador con el permiso crea y edita; el de lectura no ve los botones", async ({ browser }) => {
    const page = await browser.newPage();
    await login(page, "admin@nexus.local");
    await page.goto("/clientes/administracion?tab=empresas");
    await page.getByRole("button", { name: "Nueva empresa" }).click();
    const form = page.locator(".form-modal");
    const uiName = `${name} UI`;
    await form.getByLabel("Nombre empresa *").fill(uiName);
    await form.getByLabel("RUT *").fill(randomRut().plain);
    await form.getByLabel("Ola *").selectOption({ label: "General (según reglas)" });
    await form.getByRole("button", { name: "Crear empresa" }).click();
    await expect(page.getByText(`Empresa ${uiName} creada.`)).toBeVisible();
    const panel = page.locator(".clients-panel");
    await expect(panel.getByRole("heading", { name: `Detalle de empresa ${uiName}` })).toBeVisible();
    await expect(panel.getByRole("link", { name: "Ver integraciones" })).toHaveAttribute("href", /\/integraciones\?empresa=\d+$/);
    await expect(panel.getByRole("button", { name: "Desactivar" })).toBeVisible();
    await expect(panel.getByRole("button", { name: "Activar", exact: true })).toHaveCount(0);
    await panel.getByRole("button", { name: "Editar" }).click();
    await form.getByLabel("Teléfono").fill("+56 2 2222 3333");
    await form.getByRole("button", { name: "Guardar cambios" }).click();
    await expect(panel.getByText("+56 2 2222 3333")).toBeVisible();
    await page.close();

    const reader = await browser.newPage();
    await login(reader, "admin.lectura@nexus.local");
    await reader.goto(`/clientes/administracion?tab=empresas&q=${encodeURIComponent(uiName)}`);
    await reader.getByRole("button", { name: `Ver detalle de la empresa ${uiName}`, exact: true }).click();
    const rp = reader.locator(".clients-panel");
    await expect(rp.getByRole("link", { name: "Ver integraciones" })).toBeVisible();
    await expect(reader.getByRole("button", { name: "Nueva empresa" })).toHaveCount(0);
    for (const t of ["Editar", "Desactivar", "Activar"]) await expect(rp.getByRole("button", { name: t, exact: true })).toHaveCount(0);
    await reader.close();
  });

  test("390 px: el detalle se abre como modal, sin scroll horizontal de la página", async ({ browser }) => {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 } });
    await login(page, "admin@nexus.local");
    await page.goto(`/clientes/administracion?tab=empresas&q=${encodeURIComponent(name)}`);
    await page.getByRole("button", { name: `Ver detalle de la empresa ${name}`, exact: true }).click();
    const modal = page.getByRole("dialog");
    await expect(modal).toBeVisible();
    await expect(modal.getByRole("link", { name: "Ver integraciones" })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.close();
  });
});

// ---------------------------------------------------------------------------------------------
// Fase 3: Administración clientes › Perfiles. Los perfiles de prueba van en las empresas demo
// («SEED-…»): el seed los borra al recargar. Sus cuentas de Keycloak quedan (solo desarrollo).
// ---------------------------------------------------------------------------------------------
type Profile = { id: number; name: string; email: string; username?: string; role: string; permissions: string[]; companyName: string; active: boolean; temporaryPassword?: string };

test.describe.serial("Clientes · Fase 3: Perfiles", () => {
  const stamp = Date.now();
  const email = `e2e.perfil.${stamp}@andesoutdoor.cl`;
  let andes = 0;

  test("API: crear con o sin correo, rol Cliente fijo, correo de Keycloak, editar, desactivar y registro", async () => {
    const admin = await coreApi("admin@nexus.local");
    const reader = await coreApi("admin.lectura@nexus.local");
    const cliente = await coreApi("cliente1@nexus.local");
    andes = (await admin<{ data: Company[] }>("GET", `/v1/companies?q=${encodeURIComponent("Andes Outdoor")}`)).body.data[0].id;

    expect((await reader("POST", "/v1/profiles", { name: "X", companyId: andes, email })).status).toBe(403);
    expect((await cliente("GET", "/v1/profiles")).status).toBe(403);

    const created = await admin<Profile>("POST", "/v1/profiles", { name: "E2E Perfil Correo", companyId: andes, email, phone: "+56 9 1234 5678", jobTitle: "Bodega" });
    expect(created.status).toBe(201);
    expect(created.body).toMatchObject({ role: "cliente", permissions: [], companyName: "Andes Outdoor SpA (demo)", active: true });
    expect(created.body.temporaryPassword).toBeUndefined();
    // Keycloak envía el enlace para definir la contraseña.
    await expect.poll(() => mailSubjects(email), { timeout: 20_000 }).not.toHaveLength(0);

    const dup = await admin<{ fields: { field: string }[] }>("POST", "/v1/profiles", { name: "Otro", companyId: andes, email });
    expect(dup.status).toBe(409);
    expect(dup.body.fields[0].field).toBe("email");

    const id = created.body.id;
    expect((await admin<Profile>("PATCH", `/v1/profiles/${id}`, { name: "E2E Perfil Correo", companyId: andes, phone: "+56 2 2222 3333", jobTitle: "Ventas" })).status).toBe(200);
    expect((await admin("POST", `/v1/profiles/${id}/disable`, { reason: "no" })).status).toBe(422);
    expect((await admin<Profile>("POST", `/v1/profiles/${id}/disable`, { reason: "Ya no trabaja en la empresa" })).body.active).toBe(false);

    const log = (await (await ordersApi("admin@nexus.local"))<Activity>("GET", `/v1/activity?module=users&q=${encodeURIComponent("E2E Perfil Correo")}&limit=50`)).body.items
      .filter((i) => i.entity.label === "E2E Perfil Correo");
    expect(log.map((i) => i.action).sort()).toEqual(["created", "disabled", "updated"]);
    expect(log.find((i) => i.action === "updated")!.detailLines).toEqual(["Cargo: Bodega → Ventas", "Teléfono: +56 9 1234 5678 → +56 2 2222 3333"]);
  });

  test("API: importación masiva crea las filas válidas e informa las inválidas con fila y motivo", async () => {
    const admin = await coreApi("admin@nexus.local");
    const form = new FormData();
    form.append("file", new Blob([readFileSync("e2e/fixtures/perfiles-import.xlsx")]), "perfiles-import.xlsx");
    const res = await admin<{ rows: number; created: { row: number; name: string }[]; errors: { row: number; message: string }[] }>("POST", "/v1/profiles/import", form);
    expect(res.status).toBe(200);
    expect(res.body.created.map((c) => [c.row, c.name])).toEqual([[3, "E2E Import Uno"]]);
    expect(res.body.errors.map((e) => e.row)).toEqual([4, 5, 6, 7]);
    expect(res.body.errors.map((e) => e.message).join(" | ")).toMatch(/nombre.*RUT 99\.999\.999-9.*desactivada.*Correo inválido/);
    expect((await (await coreApi("admin.lectura@nexus.local"))("POST", "/v1/profiles/import", form)).status).toBe(403);
  });

  test("UI: panel del perfil del seed y lectura sin el permiso", async ({ browser }) => {
    const page = await browser.newPage();
    await login(page, "admin@nexus.local");
    await page.goto(`/clientes/administracion?tab=perfiles&q=${encodeURIComponent("Esperanza")}`);
    await expect(page.getByRole("heading", { name: "Todos los perfiles" })).toBeVisible();
    await page.getByRole("button", { name: "Ver detalle del perfil Esperanza Gómez", exact: true }).click();
    const panel = page.locator(".clients-panel");
    await expect(panel.getByRole("heading", { name: "Detalle de perfil Esperanza Gómez" })).toBeVisible();
    for (const text of ["Recursos humanos", "Cliente", "Ninguno", "Andes Outdoor SpA (demo)"]) await expect(panel.getByText(text, { exact: true })).toBeVisible();
    await expect(panel.getByRole("button", { name: "Desactivar" })).toBeVisible();
    await expect(panel.getByRole("button", { name: "Activar", exact: true })).toHaveCount(0);
    await page.close();

    const reader = await browser.newPage();
    await login(reader, "admin.lectura@nexus.local");
    await reader.goto(`/clientes/administracion?tab=perfiles&q=${encodeURIComponent("Esperanza")}`);
    await reader.getByRole("button", { name: "Ver detalle del perfil Esperanza Gómez", exact: true }).click();
    await expect(reader.getByRole("button", { name: "Nuevo perfil" })).toHaveCount(0);
    for (const t of ["Editar", "Desactivar", "Activar", "Restablecer contraseña"]) await expect(reader.locator(".clients-panel").getByRole("button", { name: t, exact: true })).toHaveCount(0);
    await reader.close();
  });

  test("390 px: el detalle del perfil se abre como modal", async ({ browser }) => {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 } });
    await login(page, "admin@nexus.local");
    await page.goto(`/clientes/administracion?tab=perfiles&q=${encodeURIComponent("Esperanza")}`);
    await page.getByRole("button", { name: "Ver detalle del perfil Esperanza Gómez", exact: true }).click();
    await expect(page.getByRole("dialog")).toBeVisible();
    await expect(page.getByRole("dialog").getByText("Recursos humanos")).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.close();
  });
});

// Perfil Cliente sin correo (Fase 3), aparte: falla si el realm local exige correo.
test.describe("Clientes · Fase 3: Perfil sin correo", () => {
  // Requiere que el realm local no exija correo (realm del repo desde 3b292d4, SCRUM-231).
  test("API: perfil sin correo con contraseña temporal; restablecer genera otra", async () => {
    const admin = await coreApi("admin@nexus.local");
    const andes = (await admin<{ data: Company[] }>("GET", `/v1/companies?q=${encodeURIComponent("Andes Outdoor")}`)).body.data[0].id;
    const stamp = Date.now();
    const local = await admin<Profile>("POST", "/v1/profiles", { name: "E2E Perfil Sin Correo", companyId: andes, username: `e2e.${stamp}` });
    expect(local.status, JSON.stringify(local.body)).toBe(201);
    expect(local.body.temporaryPassword).toBeTruthy();
    const reset = await admin<{ emailSent: boolean; temporaryPassword?: string }>("POST", `/v1/profiles/${local.body.id}/password`);
    expect(reset.status).toBe(200);
    expect(reset.body.temporaryPassword).toBeTruthy();
    expect(reset.body.temporaryPassword).not.toBe(local.body.temporaryPassword);
  });
});
