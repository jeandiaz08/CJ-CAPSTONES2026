"use client";

import { CompactTable } from "@/components/CompactTable";
import { Badge } from "@/components/StatusBadge";
import type { CatalogProduct } from "@/services/products";
import { dateLabel } from "@/features/orders/components/OrderDetail";
import { waveLabel } from "@/features/clients/waves";
import { TYPE_LABELS } from "../labels";

// Panel «Detalle de producto» (Tarea 09). Datos del producto, quién lo modificó y cuándo; si es kit,
// su composición (un kit no tiene stock propio). Editar y Activar o Desactivar solo para quien puede
// escribir (el Administrador).
export function ProductDetail({ product, canWrite, inModal = false, onEdit, onToggle }: {
  product: CatalogProduct;
  canWrite: boolean;
  inModal?: boolean;
  onEdit: () => void;
  onToggle: () => void;
}) {
  const facts: [string, string][] = [
    ["SKU", product.sku],
    ["Descripción", product.description],
    ["Tipo", TYPE_LABELS[product.type]],
    ["Cliente", product.companyName || "—"],
    ["Ola", waveLabel(product.waveCode)],
    ["Fecha de creación", dateLabel(product.createdAt)],
    ["Fecha de actualización", dateLabel(product.updatedAt)],
    ["Modificado por", product.updatedBy || "—"],
  ];
  return (
    <div className="order-detail company-detail">
      {!inModal && <h2>Detalle de producto {product.sku}</h2>}
      <div className="order-detail-content">
        <div className="detail-status-row"><Badge tone={product.active ? "ok" : "neutral"} label={product.active ? "Activo" : "Inactivo"} /></div>
        <section>
          <dl className="registry-facts">
            {facts.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value}</dd></div>)}
          </dl>
        </section>
        {product.type === "kit" && (
          <section aria-label="Composición">
            <h3>Composición</h3>
            <p className="form-hint">Kit: no tiene stock propio.</p>
            <CompactTable label="Composición del kit" columns={[{ label: "SKU" }, { label: "Descripción" }, { label: "Cantidad" }]}
              rows={(product.components ?? []).map((c) => ({ key: c.productId, cells: [c.sku, c.description, c.quantity] }))} />
          </section>
        )}
      </div>
      {canWrite && (
        <div className="detail-actions">
          {product.active
            ? <button type="button" className="btn danger" onClick={onToggle}>Desactivar</button>
            : <button type="button" className="btn success" onClick={onToggle}>Activar</button>}
          <div><button type="button" className="btn ghost" onClick={onEdit}>Editar</button></div>
        </div>
      )}
    </div>
  );
}
