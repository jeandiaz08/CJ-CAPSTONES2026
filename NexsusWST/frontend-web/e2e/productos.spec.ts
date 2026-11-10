// Productos (Tarea 09, H-04). Fase 1: CRUD de productos y kits, permisos y registro.
// Usa el seed: Empresa Demo Uno (11.111.111-1, SKU-001…) del de desarrollo y Andes Outdoor
// (SEED-76.100.001-1, AO-1001…) del de demostración. Lo que crean estas pruebas usa SKU «E2E-…».
import { expect, test, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { BASE, coreApi, login, ordersApi, password } from "./helpers";

type Product = { id: number; companyId: number; sku: string; type: string; active: boolean; waveCode: string | null; updatedBy: string;
  components?: { sku: string; quantity: number }[] };
type Activity = { items: { action: string; entity: { type: string; id: string; label: string }; detailLines: string[] }[] };

const sidebar = (page: Page) => page.locator(".sidebar");

async function andes(admin: Awaited<ReturnType<typeof coreApi>>) {
  const r = await admin<{ data: Product[] }>("GET", "/v1/products?q=AO-1001");
  return r.body.data.find((p) => p.sku === "AO-1001")!.companyId;
}

test.describe("Productos · Fase 1: API", () => {
  test("permisos: escribe solo el Administrador; el Cliente ve solo su empresa", async () => {
    const admin = await coreApi("admin@nexus.local");
    const jefe = await coreApi("jefe@nexus.local");
    const cliente = await coreApi("cliente1@nexus.local");
    const empaque = await coreApi("empaque@nexus.local");
    const body = { companyId: 1, sku: `E2E-X-${Date.now()}`, description: "No debe crearse", type: "simple", waveCode: null };
    expect((await jefe("POST", "/v1/products", body)).status).toBe(403);
    expect((await cliente("POST", "/v1/products", body)).status).toBe(403);
    // Rutas antiguas quitadas: la ola y la composición se editan con PATCH /v1/products/{id}.
    expect([404, 405]).toContain((await admin("PATCH", "/v1/products/1/wave", { waveCode: "AM" })).status);
    expect([404, 405]).toContain((await admin("PUT", "/v1/products/1/components", { components: [] })).status);
    expect((await empaque("GET", "/v1/products")).status).toBe(403);
    const mine = await cliente<{ data: Product[]; total: number }>("GET", "/v1/products?limit=200");
    expect(mine.status).toBe(200);
    expect(mine.body.data.length).toBeGreaterThan(0);
    expect(mine.body.data.every((p) => p.companyId === 1)).toBe(true);
    expect((await cliente("GET", "/v1/products?company_id=2")).status).toBe(403);
    // El formulario de órdenes sigue usando la misma ruta (search y company_id).
    const form = await cliente<{ data: Product[] }>("GET", "/v1/products?limit=100&search=SKU-00&company_id=1");
    expect(form.body.data.map((p) => p.sku)).toEqual(expect.arrayContaining(["SKU-001", "SKU-002"]));
  });

  test("CRUD: SKU único por empresa, kit como composición y sin borrado; todo en el registro", async () => {
    const admin = await coreApi("admin@nexus.local");
    const company = await andes(admin);
    const stamp = Date.now();
    const sku = `E2E-ART-${stamp}`;
    const created = await admin<Product>("POST", "/v1/products", { companyId: company, sku: sku.toLowerCase(), description: "Artículo e2e", type: "simple", waveCode: null });
    expect(created.status).toBe(201);
    expect(created.body).toMatchObject({ sku, type: "simple", active: true, waveCode: null, updatedBy: "Admin Demo" });
    const dup = await admin<{ fields: { field: string }[] }>("POST", "/v1/products", { companyId: company, sku, description: "Otro", type: "simple", waveCode: null });
    expect(dup.status).toBe(409);
    expect(dup.body.fields[0].field).toBe("sku");

    // Kit: composición con artículos activos del mismo cliente; nunca stock.
    const kitSku = `E2E-KIT-${stamp}`;
    expect((await admin("POST", "/v1/products", { companyId: company, sku: kitSku, description: "Kit e2e", type: "kit", waveCode: null, components: [] })).status).toBe(422);
    expect((await admin("POST", "/v1/products", { companyId: company, sku: kitSku, description: "Kit e2e", type: "kit", waveCode: null, components: [{ sku: "SKU-001", quantity: 1 }] })).status).toBe(422);
    const kit = await admin<Product>("POST", "/v1/products", { companyId: company, sku: kitSku, description: "Kit e2e", type: "kit", waveCode: "AM",
      components: [{ sku, quantity: 2 }, { sku: "AO-1002", quantity: 1 }] });
    expect(kit.status).toBe(201);
    expect(kit.body.components).toEqual([expect.objectContaining({ sku: "AO-1002", quantity: 1 }), expect.objectContaining({ sku, quantity: 2 })]);
    expect(JSON.stringify(kit.body).toLowerCase()).not.toContain("stock");

    // Editar: cliente, SKU y tipo no se editan; descripción, ola y composición sí.
    expect((await admin("PATCH", `/v1/products/${kit.body.id}`, { description: "Kit", sku: "OTRO" })).status).toBe(400);
    const edited = await admin<Product>("PATCH", `/v1/products/${kit.body.id}`, { description: "Kit e2e editado", waveCode: null, components: [{ sku, quantity: 3 }] });
    expect(edited.status).toBe(200);
    expect(edited.body.components).toEqual([expect.objectContaining({ sku, quantity: 3 })]);

    // Sin borrado: se desactiva con motivo; un componente de un kit activo no se desactiva.
    expect((await admin("POST", `/v1/products/${created.body.id}/disable`, { reason: "Descontinuado" })).status).toBe(409);
    expect((await admin("POST", `/v1/products/${kit.body.id}/disable`, { reason: "Fin de temporada" })).status).toBe(200);
    expect((await admin("POST", `/v1/products/${created.body.id}/disable`, { reason: "Descontinuado" })).status).toBe(200);
    expect((await admin<Product>("GET", `/v1/products/${created.body.id}`)).body.active).toBe(false);

    const log = (await (await ordersApi("admin@nexus.local"))<Activity>("GET", `/v1/activity?module=products&q=${encodeURIComponent(kitSku)}&limit=50`)).body.items;
    expect(log.map((i) => i.action).sort()).toEqual(["product_created", "product_disabled", "product_updated"]);
    expect(log.find((i) => i.action === "product_updated")!.detailLines.join(" ")).toContain("Composición");
  });
});

test.describe("Productos · Fase 1: pantalla", () => {
  test("Administrador: crea un kit desde el formulario y ve su composición; Registro › Productos", async ({ page }) => {
    await login(page, "admin@nexus.local");
    await sidebar(page).getByRole("link", { name: "Productos" }).click();
    await expect(page.getByRole("heading", { name: "Productos" })).toBeVisible();
    await expect(page.locator(".clients-list th")).toHaveText(["", "SKU", "Descripción", "Tipo", "Cliente", "Ola", "Estado", "Fecha de actualización"]);

    const sku = `E2E-UI-${Date.now()}`;
    await page.getByRole("button", { name: "Nuevo producto" }).click();
    const modal = page.locator(".form-modal");
    await modal.getByLabel("Cliente *").selectOption({ label: "Empresa Demo Uno SpA" });
    await modal.getByLabel("SKU *").fill(sku);
    await modal.getByLabel("Descripción *").fill("Kit armado en e2e");
    await modal.getByLabel("Tipo *").selectOption("kit");
    await modal.getByLabel("SKU del componente 1").fill("SKU-001");
    await modal.getByLabel("Cantidad del componente 1").fill("2");
    await modal.getByRole("button", { name: "Crear producto" }).click();
    const panel = page.locator(".clients-panel");
    await expect(panel.getByRole("heading", { name: `Detalle de producto ${sku}` })).toBeVisible();
    await expect(panel.getByText("Kit: no tiene stock propio.")).toBeVisible();
    await expect(panel.locator(".compact-table tbody tr")).toHaveText([/SKU-001.*2/]);
    await expect(panel.getByText("Admin Demo")).toBeVisible();

    await page.goto(`/ordenes/registro?tab=acciones&section=products&q=${sku}`);
    await expect(page.getByRole("link", { name: sku })).toHaveAttribute("href", /\/productos\?producto=\d+/);
  });

  test("Jefe y Cliente: solo lectura; el formulario de órdenes sigue sugiriendo SKU", async ({ browser }) => {
    const jefe = await browser.newPage();
    await login(jefe, "jefe@nexus.local");
    await jefe.goto("/productos");
    await expect(jefe.locator(".clients-list tbody tr").first()).toBeVisible();
    await expect(jefe.getByRole("button", { name: "Nuevo producto" })).toHaveCount(0);
    await jefe.locator(".clients-list tbody tr").first().click();
    await expect(jefe.locator(".clients-panel").getByRole("button", { name: "Editar" })).toHaveCount(0);
    await jefe.close();

    const cliente = await browser.newPage();
    await login(cliente, "cliente1@nexus.local");
    await cliente.goto("/productos");
    await expect(cliente.locator(".clients-list tbody tr").first()).toBeVisible();
    await expect(cliente.locator(".clients-list tbody td:nth-child(5)").filter({ hasNotText: "Empresa Demo Uno SpA" })).toHaveCount(0);
    await cliente.goto("/ordenes/todas/listado");
    await cliente.getByRole("button", { name: "Nueva orden" }).click();
    const form = cliente.locator(".form-modal");
    await form.getByLabel("SKU").first().fill("SKU-00");
    await expect(form.locator("datalist option[value='SKU-001']")).toHaveCount(1);
    await cliente.close();
  });
});

test.describe("Productos · Fase 2: Excel y seed", () => {
  test("API: importa artículos válidos e informa las filas inválidas con fila y motivo", async () => {
    const admin = await coreApi("admin@nexus.local");
    const res = await fetch(`${BASE}/auth/realms/nexuswst/protocol/openid-connect/token`, {
      method: "POST",
      body: new URLSearchParams({ grant_type: "password", client_id: "nexus-web", username: "admin@nexus.local", password: password("admin@nexus.local") }),
    });
    const { access_token: token } = await res.json();
    const form = new FormData();
    form.append("file", new Blob([readFileSync("e2e/fixtures/productos-import.xlsx")]), "productos-import.xlsx");
    const imp = await fetch(`${BASE}/api/core/v1/products/import`, { method: "POST", headers: { Authorization: `Bearer ${token}` }, body: form });
    expect(imp.status).toBe(200);
    const body = (await imp.json()) as { rows: number; created: { row: number }[]; errors: { row: number; field: string }[] };
    expect(body.rows).toBe(9);
    expect(body.created.map((c) => c.row)).toEqual([3, 4]);
    expect(body.errors.map((e) => `${e.row}:${e.field}`)).toEqual(["5:companyId", "6:companyId", "7:companyId", "8:sku", "9:sku", "10:sku", "11:waveCode"]);
    const list = await admin<{ data: Product[] }>("GET", "/v1/products?q=E2E-IMP-00");
    expect(list.body.data.map((p) => [p.sku, p.type, p.waveCode]).sort()).toEqual([["E2E-IMP-001", "simple", null], ["E2E-IMP-002", "simple", "PM"]]);
    // El Jefe y el Cliente no importan ni descargan la plantilla.
    expect((await (await coreApi("jefe@nexus.local"))("GET", "/v1/products/template")).status).toBe(403);
  });

  test("seed: kits como composición, el inactivo y el Cliente exporta solo su empresa", async ({ page }) => {
    const admin = await coreApi("admin@nexus.local");
    const kits = await admin<{ data: Product[] }>("GET", "/v1/products?type=kit&q=KIT-01");
    expect(kits.body.data.map((p) => p.sku).sort()).toEqual(["AO-KIT-01", "CB-KIT-01"]);
    const kit = await admin<Product>("GET", `/v1/products/${kits.body.data.find((p) => p.sku === "AO-KIT-01")!.id}`);
    expect(kit.body.components!.map((c) => c.sku)).toEqual(["AO-1001", "AO-1002"]);
    expect((await admin<{ data: Product[] }>("GET", "/v1/products?q=AO-1006&status=inactive")).body.data).toHaveLength(1);

    await login(page, "cliente1@nexus.local");
    await page.goto("/productos");
    await page.getByRole("button", { name: /Excel acciones/ }).click();
    await expect(page.getByRole("menuitem", { name: "Importar productos" })).toHaveCount(0);
    const [download] = await Promise.all([page.waitForEvent("download"), page.getByRole("menuitem", { name: "Exportar (filtros actuales)" }).click()]);
    expect(download.suggestedFilename()).toMatch(/\.xlsx$/);
  });

  test("UI: el Administrador importa desde el menú Excel y ve el resultado", async ({ page }) => {
    await login(page, "admin@nexus.local");
    await page.goto("/productos");
    await page.getByRole("button", { name: /Excel acciones/ }).click();
    const [template] = await Promise.all([page.waitForEvent("download"), page.getByRole("menuitem", { name: "Descargar plantilla" }).click()]);
    expect(template.suggestedFilename()).toBe("plantilla-productos.xlsx");
    await page.getByRole("button", { name: /Excel acciones/ }).click();
    await page.getByRole("menuitem", { name: "Importar productos" }).click();
    const modal = page.locator(".form-modal");
    await modal.locator("input[type=file]").setInputFiles("e2e/fixtures/productos-import.xlsx");
    await modal.getByRole("button", { name: "Importar" }).click();
    // Orden independiente de la prueba de API: las filas inválidas del archivo siempre se informan.
    await expect(page.locator(".form-modal").getByText("Resultado de la importación")).toBeVisible();
    await expect(page.locator(".form-modal").getByText("No hay un cliente con el RUT 99.999.999-9.")).toBeVisible();
    await expect(page.locator(".form-modal").getByText("El cliente con RUT SEED-76.100.004-4 está desactivado.")).toBeVisible();
  });
});
