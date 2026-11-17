"use client";

import { useState } from "react";
import { FormModal } from "@/components/FormModal";
import { StatusBadge } from "@/components/StatusBadge";
import { showToast } from "@/components/Toast";
import { originLabel, useOrderResource } from "@/features/orders";
import { ApiError, apiErrorMessage } from "@/services/api";
import { productsForCompany } from "@/services/orderReference";
import { takeForPacking, type Order } from "@/services/orders";
import { ageLabel, unitsOf } from "../format";

// Detalle de una orden de «Por empacar», con el botón para tomarla.
export function TakeOrderModal({ order, companyName, courierName, now, onClose, onTaken, onLost }: {
  order: Order;
  companyName?: string;
  courierName?: string;
  now: number;
  onClose: () => void;
  onTaken: (order: Order) => void;
  onLost: (order: Order) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const products = useOrderResource(() => productsForCompany(order.companyId), `packing-products:${order.companyId}`);
  const items = order.items ?? [];
  const rows: [string, string | undefined][] = [
    ["Cliente", companyName],
    ["Courier", courierName ?? order.courier],
    ["Origen", originLabel(order)],
    ["Destinatario", order.recipientName],
    ["Comuna", order.commune],
    ["Productos", String(items.length)],
    ["Unidades", String(unitsOf(items))],
    ["Antigüedad", ageLabel(order.createdAt, now)],
  ];
  const take = async () => {
    setError(null);
    setBusy(true);
    try {
      onTaken(await takeForPacking(order.id));
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        showToast("Otra persona ya tomó esta orden", "warning");
        onLost(order);
        return;
      }
      setError(apiErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <FormModal title={`Orden #${order.externalRef}`} status={<StatusBadge status={order.status} label={order.statusLabel} />} onClose={onClose} onSubmit={take} submitLabel="Tomar orden" submitTone="highlight" busy={busy} error={error}>
      <dl className="modal-data">{rows.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value || "—"}</dd></div>)}</dl>
      <div className="packing-items">
        <table>
          <thead><tr><th>SKU</th><th>Producto</th><th>Cantidad</th></tr></thead>
          <tbody>{items.map((item) => <tr key={item.sku}><td>{item.sku}</td><td>{products.data?.find((p) => p.sku === item.sku)?.description ?? "—"}</td><td>{item.quantity}</td></tr>)}</tbody>
        </table>
      </div>
    </FormModal>
  );
}
