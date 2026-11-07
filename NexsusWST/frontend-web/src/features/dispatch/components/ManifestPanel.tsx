"use client";

import { useEffect, useState } from "react";
import { showToast } from "@/components/Toast";
import { closeManifest, type Manifest, type ManifestLine } from "@/services/dispatch";
import { getOrder } from "@/services/orders";
import { activeLines, closeNotes, lineProgress, readAuthorizations, saveAuthorizations, type DispatchAuthorization } from "../manifest";
import { ConfirmDeliveryModal, DeliverySummaryModal, DispatchAnywayModal, type DeliveryForm, type DeliverySummary } from "./DeliveryModals";
import { ManifestAccordions, type LineInfo } from "./ManifestAccordions";

// Manifiesto abierto del courier: se lee en pantalla (no se imprime) y se confirma la entrega.
export function ManifestPanel({ manifest, courierName, canOperate, userName, info, productName, onClosed }: {
  manifest: Manifest;
  courierName: string;
  canOperate: boolean;
  userName: string;
  info: (line: ManifestLine) => LineInfo;
  productName: (companyId: number, sku: string) => string;
  onClosed: () => void;
}) {
  const [authorizations, setAuthorizations] = useState(() => readAuthorizations(manifest.id));
  const [authorizing, setAuthorizing] = useState<ManifestLine | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [summary, setSummary] = useState<DeliverySummary | null>(null);
  useEffect(() => saveAuthorizations(manifest.id, authorizations), [manifest.id, authorizations]);

  const lines = activeLines(manifest.lines);
  // Una autorización deja de valer si la orden salió del manifiesto o ya tiene todo escaneado.
  const valid = authorizations.filter((a) => lines.some((l) => l.orderId === a.orderId && lineProgress(l).missing.length > 0));
  const totals = lines.reduce((t, l) => {
    const p = lineProgress(l);
    return { scanned: t.scanned + p.scanned, total: t.total + p.total };
  }, { scanned: 0, total: 0 });
  const lastPicker = valid.at(-1)?.pickerName ?? "";

  const authorize = (a: DispatchAuthorization) => {
    setAuthorizations((list) => [...list.filter((x) => x.orderId !== a.orderId), a]);
    setAuthorizing(null);
  };
  const confirm = async (form: DeliveryForm) => {
    const res = await closeManifest(manifest.id, {
      pickerName: form.pickerName.trim(), pickerId: form.pickerId.trim(), vehiclePlate: form.vehiclePlate.trim(),
      notes: closeNotes(form.company, form.notes, valid),
    });
    const ref = (id: number) => manifest.lines.find((l) => l.orderId === id)?.externalRef ?? String(id);
    const pendingReason = (id: number) => valid.some((a) => a.orderId === id)
      // TODO(api): con `dispatchAnyway` el servidor la entregaría; hoy solo queda registrada.
      ? "Despacho autorizado y registrado en el manifiesto; el servidor aún no entrega con bultos faltantes"
      : "Faltan bultos por escanear";
    const removed = manifest.lines.filter((l) => l.status === "removed");
    const incidents = await Promise.all(removed.map(async (l) => {
      const detail = await getOrder(l.orderId, null).catch(() => null);
      return { ref: l.externalRef, reason: [detail?.order.incidentLabel, detail?.order.note].filter(Boolean).join(": ") || "Incidencia" };
    }));
    setConfirming(false);
    saveAuthorizations(manifest.id, []);
    setSummary({ manifestId: manifest.id, delivered: res.delivered.map((id) => ({ ref: ref(id) })), pending: res.pending.map((id) => ({ ref: ref(id), reason: pendingReason(id) })), incidents });
    showToast(`Manifiesto #${manifest.id}: ${res.delivered.length} ${res.delivered.length === 1 ? "orden entregada" : "órdenes entregadas"}${res.pending.length ? `, ${res.pending.length} para el siguiente retiro` : ""}.`, res.pending.length ? "warning" : "success");
  };

  return (
    <section className="dispatch-panel manifest-panel" aria-labelledby="manifest-title">
      <div className="manifest-head">
        <h2 id="manifest-title">Manifiesto #{manifest.id} · {courierName}</h2>
        <dl className="manifest-facts">
          <div><dt>Courier</dt><dd>{courierName}</dd></div>
          <div><dt>Empresa del transporte</dt><dd>Se registra al confirmar</dd></div>
          <div><dt>Estado</dt><dd>Abierto</dd></div>
          <div><dt>Órdenes</dt><dd>{lines.length}</dd></div>
          <div><dt>Bultos</dt><dd>{totals.total}</dd></div>
          <div><dt>Escaneados</dt><dd>{totals.scanned}/{totals.total}</dd></div>
        </dl>
        <p className="muted">El manifiesto no se imprime: se revisa en pantalla. Solo se imprime la etiqueta.</p>
      </div>
      <ManifestAccordions lines={lines} info={info} productName={productName} authorizations={valid}
        renderOrderActions={(line) => {
          const { scanned, missing } = lineProgress(line);
          if (!missing.length || !canOperate) return null;
          const authorized = valid.some((a) => a.orderId === line.orderId);
          return authorized ? (
            <button type="button" className="btn ghost manifest-card-action" onClick={() => setAuthorizations((list) => list.filter((a) => a.orderId !== line.orderId))}>Quitar autorización</button>
          ) : (
            <span title={scanned === 0 ? "Escanea al menos un bulto: sin bultos no hay nada que despachar." : undefined}>
              <button type="button" className="btn manifest-card-action is-anyway" disabled={scanned === 0} onClick={() => setAuthorizing(line)}>Despachar igual</button>
            </span>
          );
        }} />
      <div className="manifest-foot">
        <p>Si una orden incompleta no se autoriza, queda para el siguiente retiro.</p>
        <button type="button" className="btn form-submit-success dispatch-action" disabled={!canOperate || lines.length === 0} onClick={() => setConfirming(true)}>Confirmar entrega</button>
      </div>
      {authorizing && <DispatchAnywayModal line={authorizing} authorizedBy={userName} defaultPicker={lastPicker} onClose={() => setAuthorizing(null)} onConfirm={authorize} />}
      {confirming && <ConfirmDeliveryModal defaultPicker={lastPicker} onClose={() => setConfirming(false)} onConfirm={confirm} />}
      {summary && <DeliverySummaryModal summary={summary} onClose={() => { setSummary(null); onClosed(); }} />}
    </section>
  );
}
