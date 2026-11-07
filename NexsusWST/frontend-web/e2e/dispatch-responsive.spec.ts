// Responsive de Retiro y entrega en 360, 390, 768 y 1024 px. Solo lee: necesita el seed con el
// manifiesto abierto de Mercado Envíos.
import { expect, test, type Page } from "@playwright/test";
import { login } from "./helpers";

const WIDTHS = [360, 390, 768, 1024];

test.describe.serial("Retiro y entrega: responsive", () => {
  let page: Page;
  test.beforeAll(async ({ browser }) => {
    page = await browser.newPage();
    await login(page, "jefe@nexus.local");
  });
  test.afterAll(async () => page.close());

  const box = async (selector: string) => (await page.locator(selector).first().boundingBox())!;

  for (const width of WIDTHS) {
    test(`${width} px`, async () => {
      await page.setViewportSize({ width, height: 800 });
      await page.goto("/ordenes/retiro?courier=mercadoenvios");
      await expect(page.locator(".dispatch-list")).toHaveAttribute("aria-busy", "false");

      // Sin scroll horizontal de la página.
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(1);

      // Cada tarjeta de courier entra completa: su contenido no se corta.
      const cards = page.locator(".dispatch-courier");
      for (let i = 0; i < (await cards.count()); i++) {
        const clipped = await cards.nth(i).evaluate((el) => el.scrollWidth - el.clientWidth);
        expect(clipped, `tarjeta ${i}`).toBeLessThanOrEqual(1);
      }

      const kpi = await box(".dispatch-top .kpi");
      const strip = await box(".dispatch-courier-strip");
      if (width < 768) {
        // KPI en una línea, a lo ancho, y los couriers debajo.
        const top = await box(".dispatch-top");
        expect(kpi.width).toBeGreaterThan(top.width - 4);
        expect(kpi.height).toBeLessThan(90);
        expect(strip.y).toBeGreaterThanOrEqual(kpi.y + kpi.height - 1);
        expect(await page.locator(".dispatch-courier-strip").evaluate((el) => getComputedStyle(el).overflowX)).toBe("auto");
        // «Couriers:» pegado a las tarjetas, tarjetas a la medida y el número grande, a la derecha.
        const title = await box(".dispatch-couriers h2");
        const card = await box(".dispatch-courier");
        expect(card.y - (title.y + title.height)).toBeLessThan(24);
        expect(card.height).toBeLessThan(110);
        const number = await box(".dispatch-courier strong");
        expect(card.x + card.width - (number.x + number.width)).toBeLessThan(24);
        expect(await page.locator(".dispatch-courier strong").first().evaluate((el) => parseFloat(getComputedStyle(el).fontSize))).toBeGreaterThanOrEqual(32);
      }
      // Contenido de la tabla centrado en cada columna.
      expect(await page.locator(".dispatch-table td").nth(2).evaluate((el) => getComputedStyle(el).textAlign)).toBe("center");

      // «Conteo al retirar» después de la lista en pantallas angostas, con el botón visible.
      const list = await box(".dispatch-list");
      const count = await box(".dispatch-count-panel");
      if (width <= 1100) expect(count.y).toBeGreaterThanOrEqual(list.y + list.height - 1);
      const generate = page.getByRole("button", { name: "Generar manifiesto" });
      await generate.scrollIntoViewIfNeeded();
      await expect(generate).toBeInViewport();

      // El manifiesto es un modal; en teléfono ocupa la pantalla completa. Necesita el manifiesto
      // abierto que deja el seed: si alguien ya lo confirmó, esta parte se omite.
      const open = page.getByRole("button", { name: /^Ver manifiesto abierto #/ });
      if (!(await open.count())) {
        test.info().annotations.push({ type: "omitido", description: "sin manifiesto abierto de Mercado Envíos: recarga el seed" });
        return;
      }
      await open.click();
      const modal = await box(".manifest-modal");
      if (width < 768) {
        expect(modal.width).toBeGreaterThan(width - 2);
        expect(modal.height).toBeGreaterThan(798);
      } else {
        expect(modal.width).toBeLessThan(width);
      }
      const orders = page.locator(".manifest-modal").getByRole("region", { name: "Órdenes del manifiesto" });
      expect(await orders.evaluate((el) => getComputedStyle(el).overflowX)).toBe("auto");
      await page.keyboard.press("Escape");
    });
  }
});
