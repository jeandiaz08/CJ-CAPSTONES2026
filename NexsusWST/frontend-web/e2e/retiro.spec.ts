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
    await expect(row(READY).locator(".dispatch-count")).toHaveText("1/2");
    expect(codes).toHaveLength(2);
  });

  test("Fase 3: «Despachar igual» queda en las notas y la orden sigue para el siguiente retiro", async () => {
    const panel = page.locator(".manifest-panel");
    await expect(panel).toBeVisible();
    await expect(page.getByRole("button", { name: "Generar manifiesto" })).toBeDisabled();
    await expect(panel.locator("summary")).toHaveText(["Órdenes (1)", "Bultos (2)"]);
    const card = panel.getByRole("article", { name: `Orden ${READY}` });
    await expect(card.locator(".manifest-warning")).toContainText(`Faltan 1 bulto: ${codes[1]}`);

    await card.getByRole("button", { name: "Despachar igual" }).click();
    const anyway = page.getByRole("dialog");
    await expect(anyway).toContainText(codes[1]);
    await anyway.getByLabel("Nombre de quien retira").fill("Juan Pérez");
    await anyway.getByLabel(/Observación/).fill("Bulto 2 dañado (prueba)");
    await anyway.getByRole("button", { name: "Autorizar despacho" }).click();
    await expect(card.locator(".manifest-authorized")).toContainText("retira Juan Pérez");

    await panel.getByRole("button", { name: "Confirmar entrega" }).click();
    const confirm = page.getByRole("dialog");
    await expect(confirm.getByLabel("Nombre de quien retira *")).toHaveValue("Juan Pérez");
    await confirm.getByLabel(/Empresa del transporte/).fill("Transportes Sur");
    await confirm.getByLabel(/Patente/).fill("AB12CD");
    const [res] = await Promise.all([
      page.waitForResponse((r) => /\/bff\/orders\/v1\/dispatch\/manifests\/\d+\/close$/.test(r.url())),
      confirm.getByRole("button", { name: "Confirmar entrega" }).click(),
    ]);
    expect(res.status()).toBe(200);
    const summary = page.getByRole("dialog");
    await expect(summary).toContainText("Para el siguiente retiro (1)");
    await expect(summary).toContainText(`#${READY} · Despacho autorizado`);
    const manifestId = Number(res.url().match(/manifests\/(\d+)\/close/)![1]);
    const closed = (await api<Manifest & { notes: string; status: string; vehiclePlate: string }>("GET", `/v1/dispatch/manifests/${manifestId}`)).body;
    expect(closed.status).toBe("closed");
    expect(closed.vehiclePlate).toBe("AB12CD");
    expect(closed.notes).toContain("Empresa del transporte: Transportes Sur");
    expect(closed.notes).toContain(`Autorizado sin bulto · #${READY} · ${codes[1]} · Retira: Juan Pérez`);
    // TODO(api): con dispatchAnyway saldría entregada; hoy el servidor la deja para el siguiente retiro.
    expect((await api<{ order: Order }>("GET", `/v1/orders/${ready.id}`)).body.order.status).toBe("ready_pickup");
    await summary.locator(".form-modal-foot").getByRole("button", { name: "Cerrar" }).click();
    await expect(page.locator(".manifest-panel")).toHaveCount(0);

    await page.getByRole("link", { name: "Manifiestos cerrados" }).click();
    const item = page.locator(".closed-manifest", { hasText: `Manifiesto #${manifestId}` });
    await expect(item).toBeVisible();
    await item.locator("summary").first().click();
    await expect(item).toContainText("Transportes Sur");
    await expect(item).toContainText("Juan Pérez");
  });

  // El seed deja abierto un manifiesto de Mercado Envíos con 48257769225 y 48257769226, sin escanear.
  test("Fase 3: manifiesto abierto del courier; sin escanear no se despacha igual; completa, se entrega", async () => {
    const open = (await api<{ data: (Manifest & { courier: string })[] }>("GET", "/v1/dispatch/manifests?status=open")).body.data.find((m) => m.courier === "mercadoenvios")!;
    expect(open, "el seed deja un manifiesto abierto de Mercado Envíos").toBeTruthy();
    const delivered = (await api<{ data: Order[] }>("GET", "/v1/orders?status=ready_pickup&courier=mercadoenvios&limit=50")).body.data.find((o) => o.externalRef === "48257769225")!;
    const code = open.lines.find((l) => l.orderId === delivered.id)!.packages[0].code;

    await page.goto("/ordenes/retiro?courier=mercadoenvios");
    const panel = page.locator(".manifest-panel");
    await expect(panel.locator("h2")).toHaveText(`Manifiesto #${open.id} · Mercado Envíos`);
    await expect(page.getByRole("button", { name: "Generar manifiesto" })).toBeDisabled();
    const card = panel.getByRole("article", { name: `Orden ${delivered.externalRef}` });
    await expect(card.getByRole("button", { name: "Despachar igual" })).toBeDisabled();
    await expect(card.locator("span[title]")).toHaveAttribute("title", /Escanea al menos un bulto/);

    await expect(page.locator(".dispatch-list")).toHaveAttribute("aria-busy", "false");
    await scan(code); // la orden ya está en el manifiesto: va directo al servidor
    await expect(card.locator(".manifest-count")).toHaveText("1/1");
    await panel.getByRole("button", { name: "Confirmar entrega" }).click();
    await page.getByRole("dialog").getByLabel("Nombre de quien retira *").fill("Ana Retira");
    await page.getByRole("dialog").getByRole("button", { name: "Confirmar entrega" }).click();
    const summary = page.getByRole("dialog");
    await expect(summary).toContainText("Entregadas al courier (1)");
    await expect(summary).toContainText("#48257769226 · Faltan bultos por escanear");
    expect((await api<{ order: Order }>("GET", `/v1/orders/${delivered.id}`)).body.order.status).toBe("handed_over");
  });

  test("Administrador: solo lectura", async ({ browser }) => {
    const admin = await browser.newPage();
    await login(admin, "admin@nexus.local");
    await admin.goto("/ordenes/retiro");
    await expect(admin.getByText("Solo lectura")).toBeVisible();
    await admin.close();
  });
});
