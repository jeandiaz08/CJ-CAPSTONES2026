"use client";

import { ConfirmDialog } from "@/components/ConfirmDialog";
import { StatusBadge } from "@/components/StatusBadge";
import { showToast } from "@/components/Toast";
import { ApiError } from "@/services/api";
import { cancelOrder, setPriority, type Order } from "@/services/orders";
import { originLabel } from "../format";

type DialogProps = { order: Order; companyName?: string; courierName?: string; onDone: (order: Order) => void; onClose: () => void; onConflict: () => void };

function OrderData({ order, companyName, courierName }: Pick<DialogProps, "order" | "companyName" | "courierName">) {
  const rows: [string, string | undefined][] = [["Cliente", companyName], ["Destinatario", order.recipientName], ["Origen", originLabel(order)], ["Courier", courierName ?? order.courier]];
  return <dl className="modal-data">{rows.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value || "—"}</dd></div>)}</dl>;
}

// 409: otra persona cambió la orden, así que se refresca la vista detrás del aviso.
async function guarded<T>(action: () => Promise<T>, onConflict: () => void): Promise<T> {
  try {
    return await action();
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) onConflict();
    throw e;
  }
}

export function CancelOrderDialog({ order, companyName, courierName, onDone, onClose, onConflict }: DialogProps) {
  return (
    <ConfirmDialog
      title={`Cancelar orden • #${order.externalRef}`}
      status={<StatusBadge status={order.status} label={order.statusLabel} />}
      reason="required"
      reasonLabel="Motivo de la cancelación (140 máx.)"
      reasonRequiredMessage="Indica el motivo de la cancelación."
      confirmLabel="Continuar"
      destructive
      secondStep={{ message: "¿Seguro? La orden quedará Cancelada y pasará al Registro histórico. No se puede deshacer.", confirmLabel: "Sí, cancelar orden" }}
      onClose={onClose}
      onConfirm={async (reason) => {
        const updated = await guarded(() => cancelOrder(order.id, reason), onConflict);
        if (updated.status === "cancelled") showToast(`Orden #${order.externalRef} cancelada.`);
        else showToast(`Orden #${order.externalRef}: ${updated.incidentLabel || "cancelación por confirmar"}. Ya está en la ola de Invas, quedó en Incidencia.`, "warn");
        onDone(updated);
      }}
    >
      <OrderData order={order} companyName={companyName} courierName={courierName} />
    </ConfirmDialog>
  );
}

export function PriorityDialog({ order, companyName, courierName, onDone, onClose, onConflict }: DialogProps) {
  const next = !order.priority;
  return (
    <ConfirmDialog
      title={`${next ? "Enviar primero a la cola" : "Quitar prioridad"} • #${order.externalRef}`}
      status={<StatusBadge status={order.status} label={order.statusLabel} />}
      reason="optional"
      reasonLabel="Motivo (140 máx.)"
      confirmLabel={next ? "Enviar primero" : "Quitar prioridad"}
      onClose={onClose}
      onConfirm={async (reason) => {
        const updated = await guarded(() => setPriority(order.id, next, reason), onConflict);
        showToast(next ? `Orden #${order.externalRef} enviada primero a la cola.` : `Orden #${order.externalRef} sin prioridad.`);
        onDone(updated);
      }}
    >
      <OrderData order={order} companyName={companyName} courierName={courierName} />
    </ConfirmDialog>
  );
}
