"use client";

import { CircleAlert } from "lucide-react";
import type { ManifestLine } from "@/services/dispatch";
import type { OrderItem } from "@/services/orders";
import { lineProgress, type DispatchAuthorization } from "../manifest";

export type LineInfo = { companyName: string; labelCode: string; items: OrderItem[] };

// «Órdenes (N)» y «Bultos (N)»: tarjetas en una fila que se desplaza a la derecha.
export function ManifestAccordions({ lines, info, productName, authorizations = [], open = true, renderOrderActions }: {
  lines: ManifestLine[];
  info: (line: ManifestLine) => LineInfo;
  productName: (companyId: number, sku: string) => string;
  authorizations?: DispatchAuthorization[];
  open?: boolean;
  renderOrderActions?: (line: ManifestLine) => React.ReactNode;
}) {
  const bundles = lines.flatMap((line) => line.packages.map((p) => ({ line, bundle: p })));
  return (
    <div className="manifest-accordions">
      <details open={open}>
        <summary>Órdenes ({lines.length})</summary>
        <div className="manifest-strip">
          {lines.map((line) => {
            const { scanned, total, missing } = lineProgress(line);
            const data = info(line);
            const authorized = authorizations.find((a) => a.orderId === line.orderId);
            return (
              <article key={line.orderId} className={`manifest-card ${missing.length ? "is-incomplete" : "is-complete"}`} aria-label={`Orden ${line.externalRef}`}>
                <dl>
                  <div><dt>N° orden</dt><dd>#{line.externalRef}</dd></div>
                  <div><dt>Cliente</dt><dd>{data.companyName}</dd></div>
                  <div><dt>Comuna</dt><dd>{line.commune || "—"}</dd></div>
                  <div><dt>Bultos</dt><dd className="manifest-count">{scanned}/{total}</dd></div>
                  <div><dt>Código de etiqueta</dt><dd>{data.labelCode || "—"}</dd></div>
                </dl>
                {missing.length > 0 && (
                  <p className="manifest-warning"><CircleAlert size={16} aria-hidden="true" />Faltan {missing.length} {missing.length === 1 ? "bulto" : "bultos"}: {missing.join(", ")}</p>
                )}
                {authorized && <p className="manifest-authorized">Autorizada sin {authorized.missing.join(", ")} · retira {authorized.pickerName}</p>}
                {renderOrderActions?.(line)}
              </article>
            );
          })}
        </div>
      </details>
      <details open={open}>
        <summary>Bultos ({bundles.length})</summary>
        <div className="manifest-strip">
          {bundles.map(({ line, bundle }) => {
            const data = info(line);
            return (
              <article key={bundle.code} className={`manifest-card ${bundle.scanned ? "is-complete" : "is-incomplete"}`} aria-label={`Bulto ${bundle.code}`}>
                <dl>
                  <div><dt>Bulto</dt><dd>{bundle.code} {bundle.scanned ? "· escaneado" : "· sin escanear"}</dd></div>
                  <div><dt>Código de etiqueta</dt><dd>{data.labelCode || "—"}</dd></div>
                  <div><dt>Orden</dt><dd>#{line.externalRef}</dd></div>
                </dl>
                {/* TODO(api): los bultos no registran qué productos llevan; se listan los de la orden. */}
                <ul className="manifest-products" aria-label="Productos de la orden">
                  {data.items.map((item) => <li key={item.sku}>{productName(line.companyId, item.sku)} · SKU {item.sku} · {item.quantity}</li>)}
                </ul>
              </article>
            );
          })}
        </div>
      </details>
    </div>
  );
}
