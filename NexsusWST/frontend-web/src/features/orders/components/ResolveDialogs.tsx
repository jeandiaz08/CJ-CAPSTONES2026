"use client";

import { useId, useState } from "react";
import { FormModal } from "@/components/FormModal";
import { OptionCards, type CardOption } from "@/components/OptionCards";
import { StatusBadge } from "@/components/StatusBadge";
import { showToast } from "@/components/Toast";
import { ApiError, apiErrorMessage } from "@/services/api";
import { getCatalog, type CatalogItem, type Product } from "@/services/core";
import { cancelOrder, recoverVoided, resolveIncident, type Order, type OrderEvent, type ResolveCorrections } from "@/services/orders";
import { originLabel } from "../format";
import { useOrderResource } from "../hooks/useOrderResources";
import { resolveOptions, type ResolveOption } from "../rules";
import { dateLabel } from "./OrderDetail";

const MAX_NOTE = 140;
type Commune = { name: string; regionCode: string };
type Row = [string, string | undefined];

// Último evento que dejó la orden en ese estado: de ahí salen la hora y quién lo notificó.
const detectionOf = (events: OrderEvent[], status: string) => events.filter((event) => event.type === "status_change" && event.toStatus === status).at(-1);

function DataRows({ rows }: { rows: Row[] }) {
  return <dl className="modal-data">{rows.map(([label, value]) => <div key={label}><dt>{label}</dt><dd>{value || "—"}</dd></div>)}</dl>;
}

function ResolveForm({ title, order, rows, options, option, onOption, optionError, note, onNote, noteError, formError, busy, onSubmit, onClose }: {
  title: string;
  order: Order;
  rows: Row[];
  options: CardOption[];
  option: string;
  onOption: (value: string) => void;
  optionError?: string;
  note: string;
  onNote: (value: string) => void;
  noteError?: string;
  formError: string | null;
  busy: boolean;
  onSubmit: () => void;
  onClose: () => void;
}) {
  const id = useId();
  return (
    <FormModal wide title={title} status={<StatusBadge status={order.status} label={order.statusLabel} />} onClose={onClose} onSubmit={onSubmit} submitLabel="Resolver" submitTone="success" busy={busy} error={formError}>
      <DataRows rows={rows} />
      <OptionCards legend="¿Cómo se resuelve?" options={options} value={option} onChange={onOption} error={optionError} />
      <div className="modal-field resolve-note">
        <label htmlFor={id}>Observaciones de la resolución ({MAX_NOTE} máx.) *</label>
        <textarea id={id} rows={3} maxLength={MAX_NOTE} value={note} aria-invalid={!!noteError} onChange={(e) => onNote(e.target.value)} />
        <div className="modal-field-meta">
          {noteError ? <span className="modal-field-error" role="alert">{noteError}</span> : <span />}
          <span>{note.length}/{MAX_NOTE}</span>
        </div>
      </div>
    </FormModal>
  );
}

type CommonProps = {
  order: Order;
  events: OrderEvent[];
  courierName?: string;
  onDone: (order: Order) => void;
  onClose: () => void;
  onConflict: () => void;
};

export function ResolveVoidedDialog({ order, events, courierName, companyName, products, onDone, onClose, onConflict }: CommonProps & { companyName?: string; products: Product[] }) {
  const [option, setOption] = useState("");
  const [note, setNote] = useState("");
  const [errors, setErrors] = useState<{ option?: string; note?: string }>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const detection = detectionOf(events, "voided");
  const items = order.items ?? [];
  const firstName = products.find((product) => product.sku === items[0]?.sku)?.description;
  const more = items.length > 1 ? ` (+${items.length - 1} más)` : "";
  const rows: Row[] = [
    ["Cliente", companyName],
    ["Producto", firstName ? `${firstName}${more}` : undefined],
    ["SKU", items[0] ? `${items[0].sku}${more}` : undefined],
    ["Detectado", detection?.actor],
    ["Hora detectado", detection ? dateLabel(detection.occurredAt) : undefined],
    ["Origen", originLabel(order)],
    ["Courier", courierName ?? order.courier],
    ["Motivo", order.note],
  ];
  const options: CardOption[] = [
    { value: "retry", title: "Reintentar validación", description: "Se revalidará la orden: vuelve a validarse automáticamente." },
    { value: "cancel", title: "Cancelar orden", description: "Definitivo. La orden queda en el Registro histórico.", danger: true },
  ];
  const submit = async () => {
    setFormError(null);
    const found = { ...(option ? {} : { option: "Elige cómo se resuelve." }), ...(note.trim() ? {} : { note: "Deja escrito qué se hizo." }) };
    setErrors(found);
    if (Object.keys(found).length) return;
    setBusy(true);
    try {
      const updated = option === "retry" ? await recoverVoided(order.id, note.trim()) : await cancelOrder(order.id, note.trim());
      showToast(option === "retry" ? `Orden #${order.externalRef}: vuelve a validarse.` : `Orden #${order.externalRef} cancelada.`);
      onDone(updated);
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) onConflict();
      setFormError(apiErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  return <ResolveForm title={`Resolver anulación • #${order.externalRef}`} order={order} rows={rows} options={options} option={option} onOption={setOption} optionError={errors.option} note={note} onNote={setNote} noteError={errors.note} formError={formError} busy={busy} onSubmit={submit} onClose={onClose} />;
}

export function ResolveIncidentDialog({ order, events, courierName, couriers, labels, onDone, onClose, onConflict }: CommonProps & { couriers: CatalogItem[]; labels: Record<string, string> }) {
  const id = useId();
  const [option, setOption] = useState<ResolveOption | "">("");
  const [note, setNote] = useState("");
  const [address, setAddress] = useState(order.addressLine ?? "");
  const [communeName, setCommuneName] = useState(order.commune ?? "");
  const [courier, setCourier] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const communes = useOrderResource(() => getCatalog("communes").then((r) => r.data as unknown as Commune[]), "communes");
  const commune = communes.data?.find((c) => c.name.toLowerCase() === communeName.trim().toLowerCase());
  const detection = detectionOf(events, "incident");
  const rows: Row[] = [
    ["Etapa", order.prevStatus ? labels[order.prevStatus] ?? order.prevStatus : undefined],
    ["Dirección", order.addressLine],
    ["Comuna", order.commune],
    ["Motivo", order.incidentLabel],
    ["Hora detectado", detection ? dateLabel(detection.occurredAt) : undefined],
    ["Courier", courierName ?? order.courier],
    ["Notificó", detection?.actor],
    ["Comentario", order.note],
  ];
  const addressFields = (
    <div className="resolve-fields">
      <div className="modal-field">
        <label htmlFor={`${id}-address`}>Dirección</label>
        <input id={`${id}-address`} value={address} aria-invalid={!!errors.addressLine} onChange={(e) => setAddress(e.target.value)} />
        {errors.addressLine && <span className="modal-field-error" role="alert">{errors.addressLine}</span>}
      </div>
      <div className="modal-field">
        <label htmlFor={`${id}-commune`}>Comuna</label>
        <input id={`${id}-commune`} list={`${id}-communes`} value={communeName} aria-invalid={!!errors.commune} onChange={(e) => setCommuneName(e.target.value)} />
        <datalist id={`${id}-communes`}>{communes.data?.map((c) => <option key={c.name} value={c.name} />)}</datalist>
        {errors.commune && <span className="modal-field-error" role="alert">{errors.commune}</span>}
      </div>
    </div>
  );
  const courierField = (
    <div className="modal-field">
      <label htmlFor={`${id}-courier`}>Nuevo courier</label>
      <select id={`${id}-courier`} value={courier} aria-invalid={!!errors.courier} onChange={(e) => setCourier(e.target.value)}>
        <option value="">Elige un courier</option>
        {couriers.filter((c) => c.code !== order.courier).map((c) => <option key={c.code} value={c.code}>{c.name}</option>)}
      </select>
      {errors.courier && <span className="modal-field-error" role="alert">{errors.courier}</span>}
    </div>
  );
  const cards: Record<ResolveOption, CardOption> = {
    address: { value: "address", title: "Corregir dirección", children: addressFields },
    courier: { value: "courier", title: "Cambiar courier", children: courierField },
    split: { value: "split", title: "Dividir envío", badge: "Próximamente", disabled: true },
    retry: { value: "retry", title: "Reintentar", description: "Vuelve a poner en cola el trabajo que falló." },
    resume: { value: "resume", title: order.incidentKind === "cancelacion" ? "Mantener la orden" : "Reanudar sin cambios", description: "Vuelve a la etapa donde estaba." },
    cancel: { value: "cancel", title: "Cancelar orden", description: "Definitivo. La orden queda en el Registro histórico.", danger: true },
  };
  const options = resolveOptions(order.incidentKind).map((value) => cards[value]);

  const validate = () => {
    const found: Record<string, string> = {};
    if (!option) found.option = "Elige cómo se resuelve.";
    if (!note.trim()) found.note = "Deja escrito qué se hizo.";
    if (option === "address") {
      if (address.trim().length < 5 || !/\d/.test(address)) found.addressLine = "Indica calle y número.";
      if (!communeName.trim()) found.commune = "Elige la comuna.";
      else if (communes.data && !commune) found.commune = "Elige una comuna de la lista.";
    }
    if (option === "courier" && !courier) found.courier = "Elige el courier.";
    return found;
  };

  const corrections = (): ResolveCorrections | undefined => {
    if (option === "address") return { addressLine: address.trim(), commune: commune?.name ?? communeName.trim(), regionCode: commune?.regionCode ?? order.regionCode ?? "" };
    if (option === "courier") return { courier };
    return undefined;
  };

  const submit = async () => {
    setFormError(null);
    const found = validate();
    setErrors(found);
    if (Object.keys(found).length) return;
    setBusy(true);
    try {
      const action = option === "retry" ? "retry" : option === "cancel" ? "cancel" : "resume";
      const updated = await resolveIncident(order.id, action, note.trim(), corrections());
      if (updated.status === "cancelled") showToast(`Orden #${order.externalRef} cancelada.`);
      else if (option === "courier" && order.prevStatus === "ready_pickup") showToast("Courier cambiado. Reimprime la etiqueta y vuelve a agregarla al retiro");
      else showToast(`Incidencia resuelta. La orden vuelve a ${labels[updated.status] ?? updated.statusLabel}.`);
      onDone(updated);
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) onConflict();
      if (e instanceof ApiError && e.fields.length) {
        const visible = ["addressLine", "commune", "courier"];
        const shown = option === "address" ? visible.slice(0, 2) : option === "courier" ? ["courier"] : [];
        setErrors(Object.fromEntries(e.fields.filter((f) => shown.includes(f.field)).map((f) => [f.field, f.message])));
        const rest = e.fields.filter((f) => !shown.includes(f.field)).map((f) => f.message);
        setFormError(rest.length ? rest.join(" · ") : "Revisa los datos marcados.");
      } else {
        setFormError(apiErrorMessage(e));
      }
    } finally {
      setBusy(false);
    }
  };
  return <ResolveForm title={`Resolver incidencia • #${order.externalRef}`} order={order} rows={rows} options={options} option={option} onOption={(value) => setOption(value as ResolveOption)} optionError={errors.option} note={note} onNote={setNote} noteError={errors.note} formError={formError} busy={busy} onSubmit={submit} onClose={onClose} />;
}
