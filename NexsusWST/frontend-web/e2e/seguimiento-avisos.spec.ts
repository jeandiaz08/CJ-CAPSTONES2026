// Fase 4: seguimiento en el detalle y campana. Requiere el seed: 48257769230 (Entregada) tiene
// seguimiento y 48257769231 (Entregada) no. Modifica datos: una incidencia y una entrega de Recíbelo.
import { expect, test, type Page } from "@playwright/test";
import { login, ordersApi } from "./helpers";

type Order = { id: number; externalRef: string; status: string; courier: string; label: { status: string } };
type Manifest = { id: number; lines: { orderId: number; packages: { code: string }[] }[] };

test.describe.serial("Seguimiento y campana", () => {
  let page: Page;
  let api: Awaited<ReturnType<typeof ordersApi>>;
  test.beforeAll(async ({ browser }) => {
    api = await ordersApi("jefe@nexus.local");
    page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    await login(page, "jefe@nexus.local");
  });
  test.afterAll(async () => page.close());

  const openDetail = async (ref: string) => {
    await page.goto("/ordenes/todas/listado?status=handed_over");
    await page.locator(".orders-list tbody tr", { hasText: ref }).click();
    return page.locator(".order-detail");
  };

  test("detalle: seguimiento del courier, o «Sin seguimiento disponible»", async () => {
    const withTracking = await openDetail("48257769230");
    const section = withTracking.locator("section", { has: page.getByRole("heading", { name: "Seguimiento" }) });
    await expect(section.getByRole("region", { name: "Seguimiento del courier" }).locator("th")).toHaveText(["Estado", "Origen", "Fecha"]);
    await expect(section.locator("tbody tr")).toHaveCount(3);
    await expect(section).toContainText("En tránsito a Santiago");
    await expect(section).toContainText("mercadoenvios");

    const without = await openDetail("48257769231");
    await expect(without.locator("section", { has: page.getByRole("heading", { name: "Seguimiento" }) })).toContainText("Sin seguimiento disponible");
  });

  test("campana: incidencia y entrega llegan en vivo, con contador", async () => {
    await page.goto("/ordenes/empaque");
    const bell = page.locator(".notification-bell > button");
    await expect(page.getByText("En vivo")).toBeVisible();
    await expect(bell).toHaveAttribute("aria-label", "Notificaciones");

    // Incidencia sobre una Preparada.
    const picked = (await api<{ data: Order[] }>("GET", "/v1/orders?status=picked&limit=1")).body.data[0];
    expect((await api("POST", `/v1/orders/${picked.id}/incident`, { kind: "otro", reason: "Prueba de campana" })).status).toBe(200);
    await expect(bell).toHaveAttribute("aria-label", "Notificaciones, 1 sin leer");

    // Entrega de una orden de Recíbelo («Buscar en oficina»): manifiesto, escaneo y cierre por API.
    const recibelo = (await api<{ data: Order[] }>("GET", "/v1/orders?status=ready_pickup&courier=recibelo&limit=5")).body.data[0];
    const created = (await api<{ manifest: Manifest }>("POST", "/v1/dispatch/manifests", { courier: "recibelo", orderIds: [recibelo.id] })).body.manifest;
    for (const p of created.lines[0].packages) await api("POST", `/v1/dispatch/manifests/${created.id}/scan`, { code: p.code });
    expect((await api("POST", `/v1/dispatch/manifests/${created.id}/close`, { pickerName: "Prueba campana", pickerId: "", vehiclePlate: "", notes: "" })).status).toBe(200);
    await expect(bell).toHaveAttribute("aria-label", "Notificaciones, 2 sin leer");

    await bell.click();
    const list = page.getByRole("region", { name: "Avisos" }).locator("li strong");
    await expect(list.first()).toHaveText(`Orden #${recibelo.externalRef} entregada al courier`);
    await expect(list.nth(1)).toContainText("Incidencia");
    await expect(bell).toHaveAttribute("aria-label", "Notificaciones");
    await page.keyboard.press("Escape");
    await expect(page.getByRole("region", { name: "Avisos" })).toHaveCount(0);
  });
});
