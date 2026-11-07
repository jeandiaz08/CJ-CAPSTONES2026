"use client";

import type { ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import { CompactTable } from "@/components/CompactTable";
import type { ManifestLine } from "@/services/dispatch";
import type { OrderItem } from "@/services/orders";
import { lineProgress } from "../manifest";

export type LineInfo = { companyName: string; labelCode: string; items: OrderItem[] };

function Accordion({ title, open, children }: { title: string; open: boolean; children: ReactNode }) {
  return (
    <details className="manifest-accordion" open={open}>
      <summary><span>{title}</span><ChevronDown size={20} aria-hidden="true" className="manifest-chevron" /></summary>
      {children}
    </details>
  );
}

// «Órdenes (N)» abierto y «Bultos (N)» cerrado, cada uno con una tabla compacta.
export function ManifestAccordions({ lines, info, productName, orderAction }: {
  lines: ManifestLine[];
  info: (line: ManifestLine) => LineInfo;
  productName: (companyId: number, sku: string) => string;
  orderAction?: (line: ManifestLine) => ReactNode;
}) {
  const bundles = lines.flatMap((line) => line.packages.map((bundle) => ({ line, bundle })));
  return (
    <div className="manifest-accordions">
      <Accordion title={`Órdenes (${lines.length})`} open>
        <CompactTable label="Órdenes del manifiesto"
          columns={[{ label: "N° orden" }, { label: "Cliente", className: "is-text" }, { label: "Comuna" }, { label: "Bultos" }, { label: "Código de etiqueta" }, { label: "Acción", className: "is-action" }]}
          rows={lines.map((line) => {
            const { scanned, total, missing } = lineProgress(line);
            const data = info(line);
            return {
              key: line.orderId,
              className: missing.length ? "is-incomplete" : "",
              cells: [
                `#${line.externalRef}`,
                <span key="c" className="cell-truncate" title={data.companyName}>{data.companyName}</span>,
                line.commune || "—",
                <span key="b" className={`manifest-count ${missing.length ? "is-partial" : "is-complete"}`} title={missing.length ? `Faltan: ${missing.join(", ")}` : undefined}>{scanned}/{total}</span>,
                data.labelCode || "—",
                orderAction?.(line) ?? "—",
              ],
            };
          })} />
      </Accordion>
      <Accordion title={`Bultos (${bundles.length})`} open={false}>
        <CompactTable label="Bultos del manifiesto"
          columns={[{ label: "Bulto" }, { label: "Estado" }, { label: "Código de etiqueta" }, { label: "Orden" }, { label: "Productos", className: "is-text" }]}
          rows={bundles.map(({ line, bundle }) => {
            const data = info(line);
            // TODO(api): los bultos no registran qué productos llevan; se resumen los de la orden.
            const products = data.items.map((i) => `${productName(line.companyId, i.sku)} ×${i.quantity}`).join(", ");
            return {
              key: bundle.code,
              cells: [
                bundle.code,
                <span key="s" className={bundle.scanned ? "bundle-scanned" : "bundle-missing"}>{bundle.scanned ? "Escaneado" : "Sin escanear"}</span>,
                data.labelCode || "—",
                `#${line.externalRef}`,
                <span key="p" className="cell-truncate" title={products}>{products || "—"}</span>,
              ],
            };
          })} />
      </Accordion>
    </div>
  );
}
