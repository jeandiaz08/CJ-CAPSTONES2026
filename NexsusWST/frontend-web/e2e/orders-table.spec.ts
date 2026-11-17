// Tabla de Todas las órdenes y Asignación en teléfono y tablet: encabezados y celdas en la misma
// columna, textos en una línea y scroll horizontal de toda la tabla. Solo lee.
import { expect, test, type Page } from "@playwright/test";
import { login } from "./helpers";

const WIDTHS = [360, 390, 768, 1024];
const VIEWS = ["/ordenes/todas/listado", "/ordenes/todas/asignacion"];

test.describe.serial("Tabla de órdenes", () => {
  let page: Page;
  test.beforeAll(async ({ browser }) => {
    page = await browser.newPage();
    await login(page, "jefe@nexus.local");
  });
  test.afterAll(async () => page.close());

  for (const view of VIEWS) {
    for (const width of WIDTHS) {
      test(`${view} a ${width} px`, async () => {
        await page.setViewportSize({ width, height: 900 });
        await page.goto(view);
        await expect(page.locator(".orders-list tbody tr").first()).toBeVisible({ timeout: 60_000 });

        const layout = await page.evaluate(() => {
          const table = document.querySelector(".orders-list table")!;
          const heads = [...table.querySelectorAll("thead th")].map((th) => th.getBoundingClientRect());
          const rows = [...table.querySelectorAll("tbody tr")].slice(0, 8);
          const misaligned: string[] = [];
          const heights: number[] = [];
          rows.forEach((tr, r) => {
            const cells = [...tr.querySelectorAll("td")];
            if (cells.length !== heads.length) misaligned.push(`fila ${r}: ${cells.length} celdas`);
            cells.forEach((td, c) => {
              const b = td.getBoundingClientRect();
              if (Math.abs(b.left - heads[c].left) > 1 || Math.abs(b.width - heads[c].width) > 1) misaligned.push(`fila ${r} col ${c}`);
            });
            heights.push(tr.getBoundingClientRect().height);
          });
          const wrapped = [...table.querySelectorAll("td")].filter((td) => getComputedStyle(td).whiteSpace !== "nowrap").length;
          const badges = [...table.querySelectorAll(".badge")].map((b) => b.getBoundingClientRect().height);
          const client = table.querySelector(".client-name") as HTMLElement | null;
          const scroller = document.querySelector(".orders-table-scroll")!;
          return {
            misaligned, heights, wrapped, badges,
            clientEllipsis: client ? getComputedStyle(client).textOverflow === "ellipsis" && !!client.title : true,
            tableScrolls: scroller.scrollWidth > scroller.clientWidth,
            pageOverflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
          };
        });

        expect(layout.misaligned).toEqual([]);
        expect(Math.max(...layout.heights) - Math.min(...layout.heights)).toBeLessThanOrEqual(2); // nada empuja la fila
        expect(layout.wrapped).toBe(0);
        expect(Math.max(...layout.badges)).toBeLessThan(30); // los chips no se parten
        expect(layout.clientEllipsis).toBe(true);
        expect(layout.tableScrolls).toBe(true); // scroll horizontal de toda la tabla
        expect(layout.pageOverflow).toBeLessThanOrEqual(1);
      });
    }
  }
});
