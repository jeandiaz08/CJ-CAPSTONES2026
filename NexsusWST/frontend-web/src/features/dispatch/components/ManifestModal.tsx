"use client";

import { useEffect, useState } from "react";
import { MessageSquarePlus } from "lucide-react";
import { Modal } from "@/components/Modal";
import { showToast } from "@/components/Toast";
import { closeManifest, type Manifest, type ManifestLine } from "@/services/dispatch";
import { getOrder } from "@/services/orders";
import {
  activeLines, closeNotes, lineProgress, readAuthorizations, readObservations, saveAuthorizations, saveObservations,
  type DispatchAuthorization, type OrderObservation,
} from "../manifest";
import { ConfirmDeliveryModal, DeliverySummaryModal, DispatchAnywayModal, ObservationModal, type DeliveryForm, type DeliverySummary } from "./DeliveryModals";
import { ManifestAccordions, type LineInfo } from "./ManifestAccordions";

// Manifiesto del courier en un modal: se revisa en pantalla (no se imprime) y se confirma la entrega.
export function ManifestModal({ manifest, courierName, canOperate, userName, info, productName, onClose, onClosed }: {
  manifest: Manifest;
  courierName: string;
  canOperate: boolean;
  userName: string;
  info: (line: ManifestLine) => LineInfo;
  productName: (companyId: number, sku: string) => string;
  onClose: () => void;
  onClosed: () => void;
}) {
  const [authorizations, setAuthorizations] = useState(() => readAuthorizations(manifest.id));
  const [observations, setObservations] = useState(() => readObservations(manifest.id));
  const [authorizing, setAuthorizing] = useState<ManifestLine | null>(null);
  const [observing, setObserving] = useState<ManifestLine | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [summary, setSummary] = useState<DeliverySummary | null>(null);
  useEffect(() => saveAuthorizations(manifest.id, authorizations), [manifest.id, authorizations]);
  useEffect(() => saveObservations(manifest.id, observations), [manifest.id, observations]);

  const lines = activeLines(manifest.lines);
  // Una autorización deja de valer si la orden salió del manifiesto o ya tiene todo escaneado.
  const valid = authorizations.filter((a) => lines.some((l) => l.orderId === a.orderId && lineProgress(l).missing.length > 0));
  const notes = observations.filter((o) => lines.some((l) => l.orderId === o.orderId));
  const totals = lines.reduce((t, l) => {
    const p = lineProgress(l);
    return { scanned: t.scanned + p.scanned, total: t.total + p.total };
  }, { scanned: 0, total: 0 });
  const lastPicker = valid.at(-1)?.pickerName ?? "";

  const confirm = async (form: DeliveryForm) => {
    const res = await closeManifest(manifest.id, {
      pickerName: form.pickerName.trim(), pickerId: form.pickerId.trim(), vehiclePlate: form.vehiclePlate.trim(),
      notes: closeNotes(form.company, form.notes, valid, notes),
    });
    const ref = (id: number) => manifest.lines.find((l) => l.orderId === id)?.externalRef ?? String(id);
    const pendingReason = (id: number) => valid.some((a) => a.orderId === id)
      // TODO(api): con `dispatchAnyway` el servidor la entregaría; hoy solo queda registrada.
      ? "Despacho autorizado y registrado en el manifiesto; el servidor aún no entrega con bultos faltantes"
      : "Faltan bultos por escanear";
    const incidents = await Promise.all(manifest.lines.filter((l) => l.status === "removed").map(async (l) => {
      const detail = await getOrder(l.orderId, null).catch(() => null);
      return { ref: l.externalRef, reason: [detail?.order.incidentLabel, detail?.order.note].filter(Boolean).join(": ") || "Incidencia" };
    }));
    setConfirming(false);
    saveAuthorizations(manifest.id, []);
    saveObservations(manifest.id, []);
    setSummary({ manifestId: manifest.id, delivered: res.delivered.map((id) => ({ ref: ref(id) })), pending: res.pending.map((id) => ({ ref: ref(id), reason: pendingReason(id) })), incidents });
    showToast(`Manifiesto #${manifest.id}: ${res.delivered.length} ${res.delivered.length === 1 ? "orden entregada" : "órdenes entregadas"}${res.pending.length ? `, ${res.pending.length} para el siguiente retiro` : ""}.`, res.pending.length ? "warning" : "success");
  };

  const orderAction = (line: ManifestLine) => {
    if (!canOperate) return "—";
    const { scanned, missing } = lineProgress(line);
    const authorized = valid.find((a) => a.orderId === line.orderId);
    const observation = notes.find((o) => o.orderId === line.orderId);
    return (
      <span className="manifest-actions">
        {missing.length > 0 && (authorized ? (
          <button type="button" className="btn ghost manifest-action" title={`Autorizada sin ${authorized.missing.join(", ")} · retira ${authorized.pickerName}`}
            onClick={() => setAuthorizations((list) => list.filter((a) => a.orderId !== line.orderId))}>Quitar autorización</button>
        ) : (
          <span title={scanned === 0 ? "Escanea al menos un bulto: sin bultos no hay nada que despachar." : undefined}>
            <button type="button" className="btn manifest-action is-anyway" disabled={scanned === 0} onClick={() => setAuthorizing(line)}>Despachar igual</button>
          </span>
        ))}
        <button type="button" className="btn ghost manifest-action" title={observation?.text} onClick={() => setObserving(line)}>
          <MessageSquarePlus size={16} aria-hidden="true" />{observation ? "Editar observación" : "Agregar observación"}
        </button>
      </span>
    );
  };

  return (
    <>
      <Modal title={`Manifiesto #${manifest.id} · ${courierName}`} onClose={onClose} panelClassName="manifest-modal">
        <dl className="manifest-facts">
          <div><dt>Courier</dt><dd>{courierName}</dd></div>
          <div><dt>Empresa del transporte</dt><dd>Se registra al confirmar</dd></div>
          <div><dt>Estado</dt><dd>{manifest.status === "open" ? "Abierto" : "Cerrado"}</dd></div>
          <div><dt>Órdenes</dt><dd>{lines.length}</dd></div>
          <div><dt>Bultos</dt><dd>{totals.total}</dd></div>
          <div><dt>Escaneados</dt><dd>{totals.scanned}/{totals.total}</dd></div>
        </dl>
        {lines.some((l) => lineProgress(l).missing.length > 0) && (
          <p className="manifest-warning">Hay órdenes con bultos faltantes: si no se autorizan con «Despachar igual», quedan para el siguiente retiro.</p>
        )}
        <ManifestAccordions lines={lines} info={info} productName={productName} orderAction={orderAction} />
        <div className="manifest-foot">
          <p className="muted">El manifiesto no se imprime: se revisa en pantalla. Solo se imprime la etiqueta.</p>
          <button type="button" className="btn form-submit-success dispatch-action" disabled={!canOperate || lines.length === 0} onClick={() => setConfirming(true)}>Confirmar entrega</button>
        </div>
      </Modal>
      {authorizing && (
        <DispatchAnywayModal line={authorizing} authorizedBy={userName} defaultPicker={lastPicker} onClose={() => setAuthorizing(null)}
          onConfirm={(a: DispatchAuthorization) => { setAuthorizations((list) => [...list.filter((x) => x.orderId !== a.orderId), a]); setAuthorizing(null); }} />
      )}
      {observing && (
        <ObservationModal line={observing} initial={notes.find((o) => o.orderId === observing.orderId)?.text ?? ""} onClose={() => setObserving(null)}
          onSave={(text) => {
            setObservations((list) => [...list.filter((o) => o.orderId !== observing.orderId), ...(text ? [{ orderId: observing.orderId, externalRef: observing.externalRef, text } satisfies OrderObservation] : [])]);
            setObserving(null);
          }} />
      )}
      {confirming && <ConfirmDeliveryModal defaultPicker={lastPicker} onClose={() => setConfirming(false)} onConfirm={confirm} />}
      {summary && <DeliverySummaryModal summary={summary} onClose={() => { setSummary(null); onClosed(); }} />}
    </>
  );
}
