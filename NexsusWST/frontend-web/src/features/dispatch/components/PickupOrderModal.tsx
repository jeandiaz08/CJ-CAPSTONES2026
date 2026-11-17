"use client";

import { useState } from "react";
import { FormModal } from "@/components/FormModal";
import { ReportProblemDialog, type ProblemOption } from "@/components/ReportProblemDialog";
import { showToast } from "@/components/Toast";
import { originLabel, useOrderResource } from "@/features/orders";
import { productsForCompany } from "@/services/orderReference";
import { reportIncident } from "@/services/orders";
import type { PickupRow } from "../pickupList";

const PICKUP_PROBLEMS: ProblemOption[] = [
  { value: "courier_cobertura", label: "Courier no llega a la dirección" },
  { value: "retiro", label: "Otro problema en el retiro" },
];

// Detalle de una orden de la Lista de retiro, con «Informar problema».
export function PickupOrderModal({ row, companyName, courierName, canOperate, onClose, onReported }: {
  row: PickupRow;
  companyName: string;
  courierName: string;
  canOperate: boolean;
  onClose: () => void;
  onReported: () => void;
}) {
  const { order } = row;
  const [reporting, setReporting] = useState(false);
  const [picked, setPicked] = useState("");
  const products = useOrderResource(() => productsForCompany(order.companyId), `pickup-products:${order.companyId}`);
  const name = (sku: string) => products.data?.find((p) => p.sku === sku)?.description ?? "Producto sin descripción";
  const items = order.items ?? [];
  const labelCode = order.label.status === "ready" ? order.label.trackingCode : "";
  const rows: [string, string | undefined][] = [
    ["Cliente", companyName],
    ["Destinatario", order.recipientName],
    ["Teléfono", order.recipientPhone],
    ["Dirección", order.addressLine],
    ["Comuna", order.commune],
    ["Origen", originLabel(order)],
    ["Courier", courierName],
    ["Observaciones", order.customerNotes],
  ];
  // El problema puede ser de un bulto o de un producto. TODO(api): campo propio en la incidencia;
  // hoy va dentro del motivo como texto.
  const targets: ProblemOption[] = [
    ...(row.packages ?? []).map((p) => ({ value: `bulto:${p.code}`, label: `Bulto ${p.code}` })),
    ...items.map((i) => ({ value: `sku:${i.sku}`, label: `${name(i.sku)} (SKU ${i.sku})` })),
  ];
  const report = async (kind: string, reason: string, target?: ProblemOption) => {
    await reportIncident(order.id, kind, target ? `${target.label}: ${reason}` : reason);
    showToast(`La orden #${order.externalRef} pasó a Incidencia y salió de la lista de retiro.`, "warning");
    onReported();
  };

  return (
    <>
      <FormModal wide title={`${labelCode || "Sin código"} • #${order.externalRef}`} onClose={onClose} closeLabel="Cerrar"
        onSubmit={canOperate ? () => setReporting(true) : undefined} submitLabel="Informar problema" submitTone="highlight">
        {/* TODO(api): chip «Cambiado de {courier}» cuando la orden traiga el courier anterior. */}
        <dl className="modal-data pickup-data">{rows.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value || "—"}</dd></div>)}</dl>
        <div className="pickup-items">
          <table>
            <thead><tr><th scope="col">Producto</th><th scope="col">SKU</th><th scope="col">Cantidad</th><th scope="col">Bulto</th></tr></thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.sku} className={picked === `sku:${item.sku}` ? "is-picked" : ""} onClick={() => setPicked(picked === `sku:${item.sku}` ? "" : `sku:${item.sku}`)}
                  aria-selected={picked === `sku:${item.sku}`} title="Elige el producto si tiene un problema">
                  <td>{name(item.sku)}</td>
                  <td>{item.sku}</td>
                  <td>{item.quantity}</td>
                  {/* TODO(api): los bultos no registran qué productos llevan. */}
                  <td>—</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </FormModal>
      {reporting && (
        <ReportProblemDialog title="Informar problema" kinds={PICKUP_PROBLEMS} targets={targets} defaultTarget={picked}
          onClose={() => setReporting(false)} onConfirm={report} />
      )}
    </>
  );
}
