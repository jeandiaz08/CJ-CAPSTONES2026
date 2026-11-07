// Retiro y entrega (Tarea 05, fases 1 y 2). Requiere el seed de demostración recargado:
// BlueExpress tiene 48257769227 (2 bultos, etiqueta lista) y 48257769228 (etiqueta pendiente).
import { expect, test, type Page } from "@playwright/test";
import { login, ordersApi } from "./helpers";

type Order = { id: number; externalRef: string; courier: string; status: string };
type Manifest = { id: number; lines: { orderId: number; packages: { code: string; scanned: boolean }[] }[] };

const READY = "48257769227";
const PENDING = "48257769228";

test.describe.serial("Retiro y entrega", () => {
  let page: Page;
  let api: Awaited<ReturnType<typeof ordersApi>>;
  let ready: Order;
  let pending: Order;
  let codes: string[];

  test.beforeAll(async ({ browser }) => {
    api = await ordersApi("jefe@nexus.local");
    const list = (await api<{ data: Order[] }>("GET", "/v1/orders?status=ready_pickup&limit=200")).body.data;
    ready = list.find((o) => o.externalRef === READY)!;
    pending = list.find((o) => o.externalRef === PENDING)!;
    expect(ready && pending, "recarga el seed de demostración").toBeTruthy();
    codes = (await api<{ packages: { code: string }[] }>("GET", `/v1/orders/${ready.id}`)).body.packages.map((p) => p.code);
    page = await browser.newPage();
    await login(page, "jefe@nexus.local");
  });
  test.afterAll(async () => page.close());

  const reader = () => page.locator("#dispatch-reader-input");
  const scan = async (code: string) => {
    await reader().fill(code);
    await reader().press("Enter");
  };
  const row = (ref: string) => page.locator(".dispatch-table tbody tr", { hasText: `#${ref}` });
  const marked = () => page.locator(".dispatch-tally strong").nth(1);

  test("Fase 1: lista por courier, lector y bloqueo por etiqueta", async () => {
    await page.locator(".sidebar-nav .nav-link", { hasText: "Órdenes" }).click();
    await expect(page.locator('.nav-submenu a[href="/ordenes/retiro"]')).toHaveCount(1);
    await page.goto("/ordenes/retiro");
    const total = (await api<{ data: { orders: number }[] }>("GET", "/v1/dispatch/couriers")).body.data.reduce((t, q) => t + q.orders, 0);
    await expect(page.locator(".kpi strong").first()).toHaveText(String(total));
    await page.locator(".dispatch-courier", { hasText: "BlueExpress" }).click();
    await expect(page).toHaveURL(/courier=blueexpress/);
    await expect(row(PENDING)).toHaveClass(/is-blocked/);
    await expect(row(PENDING)).toContainText("Etiqueta pendiente");
    await expect(row(PENDING).locator("input")).toBeDisabled();
    await expect(reader()).toBeFocused();
    await expect(page.locator(".dispatch-list")).toHaveAttribute("aria-busy", "false"); // bultos cargados

    await scan("NADA-1");
    await expect(page.locator(".dispatch-reader-notice")).toHaveClass(/is-error/);
    await scan(codes[0]);
    await expect(row(READY).locator(".dispatch-count")).toHaveText("1/2");
    await expect(row(READY).locator(".dispatch-count")).toHaveClass(/is-partial/);
    await expect(row(READY).locator("input")).toBeChecked();
    await expect(marked()).toHaveText("1");
    await scan(codes[0]);
    await expect(page.locator(".dispatch-reader-notice")).toHaveClass(/is-warn/);

    await page.reload();
    await expect(page.locator(".dispatch-list")).toHaveAttribute("aria-busy", "false");
    await expect(marked()).toHaveText("1");
    expect(await page.evaluate(() => Object.keys(sessionStorage))).toContain("nexus:dispatch:blueexpress");
  });

  test("Fase 2: detalle de la orden e «Informar problema»", async () => {
    await page.getByRole("button", { name: `Ver orden ${READY}` }).click();
    const detail = page.getByRole("dialog");
    await expect(detail).toContainText(`#${READY}`);
    for (const label of ["Cliente", "Destinatario", "Teléfono", "Dirección", "Comuna", "Origen", "Courier", "Observaciones"]) await expect(detail).toContainText(label);
    await expect(detail.locator("th")).toHaveText(["Producto", "SKU", "Cantidad", "Bulto"]);
    await expect(detail).not.toContainText("Cambiado de");
    await detail.locator(".form-modal-foot").getByRole("button", { name: "Cerrar" }).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);

    await page.getByRole("button", { name: `Ver orden ${PENDING}` }).click();
    await page.getByRole("dialog").getByRole("button", { name: "Informar problema" }).click();
    const report = page.getByRole("dialog").last();
    await report.locator("select").first().selectOption("courier_cobertura");
    await report.locator("textarea").fill("La comuna no tiene cobertura (prueba)");
    await report.getByRole("button", { name: "Informar" }).click();
    await expect(page.locator(".toast-warning")).toContainText("pasó a Incidencia");
    await expect(row(PENDING)).toHaveCount(0);
    const after = (await api<{ order: Order & { incidentKind: string } }>("GET", `/v1/orders/${pending.id}`)).body.order;
    expect([after.status, after.incidentKind]).toEqual(["incident", "courier_cobertura"]);
  });

  test("Fase 1: generar manifiesto reenvía lo leído y limpia lo guardado", async () => {
    const [res] = await Promise.all([
      page.waitForResponse((r) => r.url().endsWith("/bff/orders/v1/dispatch/manifests") && r.request().method() === "POST"),
      page.getByRole("button", { name: "Generar manifiesto" }).click(),
    ]);
    expect(res.status()).toBe(201);
    const { manifest } = (await res.json()) as { manifest: Manifest };
    await expect(row(READY)).toContainText(`En manifiesto #${manifest.id}`);
    expect(await page.evaluate(() => Object.keys(sessionStorage).some((k) => k.startsWith("nexus:dispatch:")))).toBe(false);
    const saved = (await api<Manifest>("GET", `/v1/dispatch/manifests/${manifest.id}`)).body;
    expect(saved.lines.find((l) => l.orderId === ready.id)!.packages.find((p) => p.code === codes[0])!.scanned).toBe(true);
    await scan(codes[1]); // ya en el manifiesto: va directo al servidor
    await expect(row(READY).locator(".dispatch-count")).toHaveText("2/2");
  });

  test("Administrador: solo lectura", async ({ browser }) => {
    const admin = await browser.newPage();
    await login(admin, "admin@nexus.local");
    await admin.goto("/ordenes/retiro");
    await expect(admin.getByText("Solo lectura")).toBeVisible();
    await admin.close();
  });
});
