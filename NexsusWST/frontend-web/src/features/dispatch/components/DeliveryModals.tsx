"use client";

import { useId, useState } from "react";
import { FormModal } from "@/components/FormModal";
import { apiErrorMessage } from "@/services/api";
import type { ManifestLine } from "@/services/dispatch";
import { lineProgress, MIN_DISPATCH_REASON, type DispatchAuthorization } from "../manifest";

const MAX_TEXT = 140;

function Field({ id, label, required, value, onChange, error, multiline, plain }: {
  id: string; label: string; required?: boolean; value: string; onChange: (v: string) => void; error?: string | null; multiline?: boolean; plain?: boolean;
}) {
  const props = { id, value, maxLength: MAX_TEXT, "aria-invalid": !!error, "aria-describedby": error ? `${id}-error` : undefined, onChange: (e: { target: { value: string } }) => onChange(e.target.value) };
  return (
    <div className="modal-field">
      <label htmlFor={id}>{label}{required ? " *" : plain ? "" : " · opcional"}</label>
      {multiline ? <textarea rows={3} {...props} /> : <input {...props} />}
      {error && <span id={`${id}-error`} className="modal-field-error" role="alert">{error}</span>}
    </div>
  );
}

// «Despachar igual»: la orden sale con bultos faltantes, con quién retira y por qué.
export function DispatchAnywayModal({ line, authorizedBy, defaultPicker, onClose, onConfirm }: {
  line: ManifestLine;
  authorizedBy: string;
  defaultPicker: string;
  onClose: () => void;
  onConfirm: (authorization: DispatchAuthorization) => void;
}) {
  const id = useId();
  const [now] = useState(() => new Date());
  const [picker, setPicker] = useState(defaultPicker);
  const [reason, setReason] = useState("");
  const [errors, setErrors] = useState<{ picker?: string; reason?: string }>({});
  const { missing } = lineProgress(line);
  const submit = () => {
    const why = reason.trim();
    const next = {
      picker: picker.trim() ? undefined : "Indica quién retira.",
      reason: !why ? "Indica por qué sale sin ese bulto o producto." : [...why].length < MIN_DISPATCH_REASON ? `Escribe al menos ${MIN_DISPATCH_REASON} caracteres.` : undefined,
    };
    setErrors(next);
    if (next.picker || next.reason) return;
    onConfirm({ orderId: line.orderId, externalRef: line.externalRef, missing, pickerName: picker.trim(), authorizedBy, at: now.toISOString(), reason: why });
  };
  return (
    <FormModal title="Despachar igual" onClose={onClose} onSubmit={submit} submitLabel="Autorizar despacho" submitTone="highlight">
      <dl className="modal-data">
        <div><dt>N° orden</dt><dd>#{line.externalRef}</dd></div>
        <div><dt>Bultos faltantes</dt><dd>{missing.join(", ")}</dd></div>
        <div><dt>Fecha y hora</dt><dd>{now.toLocaleString("es-CL", { dateStyle: "short", timeStyle: "short" })}</dd></div>
        <div><dt>Autoriza</dt><dd>{authorizedBy}</dd></div>
      </dl>
      <Field id={`${id}-picker`} label="Nombre de quien retira" required value={picker} onChange={setPicker} error={errors.picker} />
      <Field id={`${id}-reason`} label="Observación: por qué sale sin ese bulto o producto" required value={reason} onChange={setReason} error={errors.reason} multiline />
    </FormModal>
  );
}

// Observación opcional de una orden; vacía, se quita.
export function ObservationModal({ line, initial, onClose, onSave }: {
  line: ManifestLine;
  initial: string;
  onClose: () => void;
  onSave: (text: string) => void;
}) {
  const id = useId();
  const [text, setText] = useState(initial);
  return (
    <FormModal title={`Observación · #${line.externalRef}`} onClose={onClose} onSubmit={() => onSave(text.trim())} submitLabel="Guardar observación">
      <p className="muted">Por ejemplo, un producto que faltó dentro de un bulto. Queda en el manifiesto al confirmar la entrega.</p>
      <Field id={`${id}-text`} label="Observación" value={text} onChange={setText} multiline />
    </FormModal>
  );
}

export type DeliveryForm = { pickerName: string; pickerId: string; vehiclePlate: string; company: string; notes: string };

// «Confirmar entrega»: datos de quien retira. La entrega la decide el servidor.
export function ConfirmDeliveryModal({ defaultPicker, onClose, onConfirm }: {
  defaultPicker: string;
  onClose: () => void;
  onConfirm: (form: DeliveryForm) => Promise<void>;
}) {
  const id = useId();
  const [form, setForm] = useState<DeliveryForm>({ pickerName: defaultPicker, pickerId: "", vehiclePlate: "", company: "", notes: "" });
  const [pickerError, setPickerError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const set = (key: keyof DeliveryForm) => (value: string) => setForm((f) => ({ ...f, [key]: value }));
  const submit = async () => {
    setError(null);
    setPickerError(form.pickerName.trim() ? null : "Indica quién retira.");
    if (!form.pickerName.trim()) return;
    setBusy(true);
    try {
      await onConfirm(form);
    } catch (e) {
      setError(apiErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <FormModal title="Confirmar entrega" onClose={onClose} onSubmit={submit} submitLabel="Confirmar entrega" submitTone="success" closeLabel="Cancelar" submitFirst busy={busy} error={error}>
      <div className="delivery-form">
        <section>
          <h3>Quien retira</h3>
          <Field id={`${id}-picker`} label="Nombre conductor" required value={form.pickerName} onChange={set("pickerName")} error={pickerError} />
          <div className="delivery-pair">
            <Field id={`${id}-rut`} label="RUT" plain value={form.pickerId} onChange={set("pickerId")} />
            <Field id={`${id}-plate`} label="Patente" plain value={form.vehiclePlate} onChange={set("vehiclePlate")} />
          </div>
        </section>
        <section>
          <h3>Transporte</h3>
          <Field id={`${id}-company`} label="Empresa del transporte" plain value={form.company} onChange={set("company")} />
          <Field id={`${id}-notes`} label="Notas" plain value={form.notes} onChange={set("notes")} multiline />
        </section>
        <p className="delivery-hint">Fecha y hora se registran solas, después de cerrar cualquier cambio genera incidencia</p>
      </div>
    </FormModal>
  );
}

export type DeliverySummary = {
  manifestId: number;
  delivered: { ref: string; reason?: string }[];
  pending: { ref: string; reason: string }[];
  incidents: { ref: string; reason: string }[];
};

export function DeliverySummaryModal({ summary, onClose }: { summary: DeliverySummary; onClose: () => void }) {
  const block = (title: string, items: { ref: string; reason?: string }[], tone: string) => (
    <section className={`delivery-block ${tone}`}>
      <h3>{title} ({items.length})</h3>
      {items.length ? <ul>{items.map((i) => <li key={i.ref}><strong>#{i.ref}</strong>{i.reason ? ` · ${i.reason}` : ""}</li>)}</ul> : <p className="muted">Ninguna.</p>}
    </section>
  );
  return (
    <FormModal title={`Manifiesto #${summary.manifestId} cerrado`} onClose={onClose} closeLabel="Cerrar">
      {block("Entregadas al courier", summary.delivered, "is-ok")}
      {block("Para el siguiente retiro", summary.pending, "is-warn")}
      {block("Incidencias", summary.incidents, "is-error")}
    </FormModal>
  );
}
