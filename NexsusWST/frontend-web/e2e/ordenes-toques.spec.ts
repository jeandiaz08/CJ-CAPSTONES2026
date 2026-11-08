// Últimos toques de Órdenes: Filtros centrado, «Ver todo» del historial en horizontal y el
// scroll de la página después de cerrar modales. Solo lee.
import { expect, test, type Page } from "@playwright/test";
import { login, ordersApi } from "./helpers";

type Order = { id: number; externalRef: string };

test.describe.serial("Órdenes: últimos toques", () => {
  let page: Page;
  let order: Order;
  let eventCount = 0;

  test.beforeAll(async ({ browser }) => {
    // La orden de la bandeja con más eventos, para que el historial tenga varios pasos.
    const api = await ordersApi("jefe@nexus.local");
    const list = (await api<{ data: Order[] }>("GET", "/v1/orders?limit=20")).body.data;
    for (const o of list) {
      const n = (await api<{ events: unknown[] }>("GET", `/v1/orders/${o.id}`)).body.events.length;
      if (n > eventCount) [order, eventCount] = [o, n];
    }
    expect(eventCount, "recarga el seed de demostración").toBeGreaterThan(2);
    page = await browser.newPage();
    await login(page, "jefe@nexus.local");
  });
  test.afterAll(async () => page.close());

  const scrollFree = async () => {
    expect(await page.evaluate(() => getComputedStyle(document.body).overflow)).not.toBe("hidden");
    // Y la página de verdad se mueve con la rueda.
    const moved = await page.evaluate(async () => {
      const root = document.scrollingElement!;
      if (root.scrollHeight <= root.clientHeight) return true;
      root.scrollTop = 0;
      window.scrollBy(0, 200);
      await new Promise((r) => requestAnimationFrame(r));
      const ok = root.scrollTop > 0;
      root.scrollTop = 0;
      return ok;
    });
    expect(moved, "la página vuelve a hacer scroll").toBe(true);
  };

  for (const [name, url] of [
    ["Todas las órdenes", "/ordenes/todas/listado"],
    ["Asignación", "/ordenes/todas/asignacion"],
    ["Registro órdenes", "/ordenes/registro"],
    ["Registro de acciones", "/ordenes/registro?tab=acciones"],
  ]) {
    test(`Filtros se abre centrado en ${name}; Escape y clic afuera devuelven el scroll`, async () => {
      for (const size of [{ width: 1440, height: 700 }, { width: 390, height: 700 }]) {
        await page.setViewportSize(size);
        await page.goto(url);
        const open = page.getByRole("button", { name: /^Filtros/ });
        await expect(open).toBeVisible({ timeout: 60_000 });
        await open.click();
        const dialog = page.getByRole("dialog", { name: "Filtros" });
        await expect(dialog).toBeVisible();
        await dialog.evaluate((d) => Promise.all(d.getAnimations({ subtree: true }).map((a) => a.finished))); // fin de la animación de entrada
        const box = (await dialog.boundingBox())!;
        expect(Math.abs(box.x + box.width / 2 - size.width / 2), "centrado en horizontal").toBeLessThan(2);
        expect(Math.abs(box.y + box.height / 2 - size.height / 2), "centrado en vertical").toBeLessThan(2);
        await page.keyboard.press("Escape");
        await expect(dialog).toHaveCount(0);
        await scrollFree();
        await open.click();
        await page.mouse.click(5, 5); // clic afuera
        await expect(dialog).toHaveCount(0);
        await scrollFree();
      }
    });
  }

  test("«Ver todo»: todos los eventos en horizontal, del primero (izquierda) al último (derecha)", async () => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto(`/ordenes/todas/listado?order=${order.id}`);
    const panel = page.locator(".orders-detail-panel");
    await expect(panel.locator("h2")).toHaveText(`Detalle de orden #${order.externalRef}`, { timeout: 60_000 });
    await panel.getByRole("button", { name: "Ver todo" }).click();
    const modal = page.locator(".history-modal");
    const items = modal.locator("ol.event-timeline.is-horizontal > li");
    await expect(items).toHaveCount(eventCount);
    const boxes = await items.evaluateAll((lis) => lis.map((li) => li.getBoundingClientRect()).map((b) => ({ x: b.x, y: b.y })));
    for (let i = 1; i < boxes.length; i++) {
      expect(boxes[i].x).toBeGreaterThan(boxes[i - 1].x);
      expect(Math.abs(boxes[i].y - boxes[0].y)).toBeLessThan(1);
    }
    await page.keyboard.press("Escape");
    await expect(modal).toHaveCount(0);
    await scrollFree();
  });

  test("«Ver todo» en teléfono: horizontal con scroll; cerrar los dos modales devuelve el scroll", async () => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/ordenes/todas/listado?order=${order.id}`);
    const detail = page.getByRole("dialog", { name: `Detalle de orden #${order.externalRef}` });
    await expect(detail).toBeVisible({ timeout: 60_000 });
    await detail.getByRole("button", { name: "Ver todo" }).click();
    const list = page.locator(".history-modal ol.event-timeline.is-horizontal");
    await expect(list.locator("> li")).toHaveCount(eventCount);
    const scroll = await list.evaluate((ol) => ({ scrolls: ol.scrollWidth > ol.clientWidth, page: document.documentElement.scrollWidth - document.documentElement.clientWidth }));
    expect(scroll.scrolls, "más eventos que ancho: scroll horizontal").toBe(eventCount > 2);
    expect(scroll.page).toBeLessThanOrEqual(1);
    await list.evaluate((ol) => { ol.scrollLeft = ol.scrollWidth; });
    await expect(list.locator("> li").last()).toBeInViewport();
    await page.keyboard.press("Escape"); // cierra el historial
    await expect(detail).toBeVisible();
    expect(await page.evaluate(() => getComputedStyle(document.body).overflow)).toBe("hidden");
    await detail.getByRole("button", { name: "Cerrar" }).click(); // y el detalle
    await expect(detail).toHaveCount(0);
    await scrollFree();
  });
});
