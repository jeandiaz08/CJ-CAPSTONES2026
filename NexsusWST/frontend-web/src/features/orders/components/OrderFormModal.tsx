"use client";

import { useId, useRef, useState, type ReactNode } from "react";
import { Plus, X } from "lucide-react";
import { FileDrop } from "@/components/FileDrop";
import { FormModal } from "@/components/FormModal";
import { showToast } from "@/components/Toast";
import { useDebounce } from "@/hooks/useDebounce";
import { ApiError, apiErrorMessage } from "@/services/api";
import { getCatalog, listProducts, type CatalogItem, type Company } from "@/services/core";
import { createOrder, updateOrder, uploadDocument, type Order, type OrderChanges } from "@/services/orders";
import { useOrderResource } from "../hooks/useOrderResources";
import { DISPATCH_LABELS, DOCUMENT_KINDS } from "../format";

const MAX_REASON = 140;
const MAX_DOCUMENT_BYTES = 10 << 20;
const DOCUMENT_EXTENSIONS = [".pdf", ".png", ".jpg", ".jpeg", ".xml"];
const RE_REF = /^[A-Za-z0-9._:/#-]{1,80}$/;
const RE_EMAIL = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;
const RE_PHONE = /^\+?[0-9 ()-]{8,20}$/;

type Row = { key: number; sku: string; quantity: string };
type PendingDocument = { key: number; file: File; kind: string };
type Commune = { name: string; regionCode: string };

function Field({ label, error, children, required = false }: { label: string; error?: string; children: (id: string, invalid: boolean) => ReactNode; required?: boolean }) {
  const id = useId();
  return (
    <div className="modal-field">
      <label htmlFor={id}>{label}{required ? " *" : ""}</label>
      {children(id, !!error)}
      {error && <span className="modal-field-error" role="alert">{error}</span>}
    </div>
  );
}

// Fila de producto: busca entre los productos activos de la empresa a medida que se escribe.
function ItemRow({ row, companyId, error, disabled, canRemove, onChange, onRemove }: {
  row: Row;
  companyId: number;
  error?: string;
  disabled: boolean;
  canRemove: boolean;
  onChange: (changes: Partial<Row>) => void;
  onRemove: () => void;
}) {
  const listId = useId();
  const term = useDebounce(row.sku.trim());
  const products = useOrderResource(
    () => (companyId ? listProducts(term, companyId).then((r) => r.data.filter((p) => p.active)) : Promise.resolve([])),
    `products:${companyId}:${term}`,
    `products:${companyId}`,
  );
  const name = products.data?.find((p) => p.sku.toLowerCase() === row.sku.trim().toLowerCase())?.description;
  return (
    <div className="order-form-item">
      <div className="order-form-item-fields">
        <div className="modal-field">
          <input aria-label="SKU" placeholder="Buscar SKU o producto" list={listId} value={row.sku} disabled={disabled} aria-invalid={!!error} onChange={(e) => onChange({ sku: e.target.value })} />
          <datalist id={listId}>{products.data?.map((p) => <option key={p.id} value={p.sku}>{p.description}</option>)}</datalist>
        </div>
        <div className="modal-field">
          <input aria-label="Cantidad" type="number" min={1} step={1} value={row.quantity} disabled={disabled} aria-invalid={!!error} onChange={(e) => onChange({ quantity: e.target.value })} />
        </div>
        {canRemove && !disabled && <button type="button" className="toast-close" aria-label="Quitar producto" onClick={onRemove}><X size={18} /></button>}
      </div>
      {name && <small className="order-form-item-name">{name}</small>}
      {error && <span className="modal-field-error" role="alert">{error}</span>}
    </div>
  );
}

// Nueva orden y Editar usan el mismo formulario: con `order` se edita y solo se envía lo que cambió.
export function OrderFormModal({ order, companies, couriers, defaultCompanyId, onClose, onSaved, onConflict }: {
  order?: Order;
  companies: Company[];
  couriers: CatalogItem[];
  defaultCompanyId: number | null;
  onClose: () => void;
  onSaved: (order: Order) => void;
  onConflict?: () => void;
}) {
  const editing = !!order;
  const nextKey = useRef(1);
  const newKey = () => nextKey.current++;
  const activeCompanies = companies.filter((company) => company.active !== false);
  const onlyCompany = activeCompanies.length === 1 ? activeCompanies[0].id : 0;
  const [companyId, setCompanyId] = useState<number>(order?.companyId ?? defaultCompanyId ?? onlyCompany);
  const [form, setForm] = useState({
    externalRef: order?.externalRef ?? "",
    courier: order?.courier ?? "",
    dispatchType: order?.dispatchType || "domicilio",
    recipientName: order?.recipientName ?? "",
    recipientPhone: order?.recipientPhone ?? "",
    recipientEmail: order?.recipientEmail ?? "",
    addressLine: order?.addressLine ?? "",
    commune: order?.commune ?? "",
    customerNotes: order?.customerNotes ?? "",
  });
  const [rows, setRows] = useState<Row[]>(() => order?.items?.length ? order.items.map((item) => ({ key: newKey(), sku: item.sku, quantity: String(item.quantity) })) : [{ key: newKey(), sku: "", quantity: "1" }]);
  const [documents, setDocuments] = useState<PendingDocument[]>([]);
  const [reason, setReason] = useState("");
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const communes = useOrderResource(() => getCatalog("communes").then((r) => r.data as unknown as Commune[]), "communes");
  const pickup = form.dispatchType === "retiro_oficina";
  const itemsLocked = editing && order.itemsEditable === false;
  const commune = communes.data?.find((c) => c.name.toLowerCase() === form.commune.trim().toLowerCase());
  const courierOptions = order && !couriers.some((c) => c.code === order.courier) ? [...couriers, { code: order.courier, name: order.courier }] : couriers;
  const set = (changes: Partial<typeof form>) => setForm((current) => ({ ...current, ...changes }));

  const validate = () => {
    const found: Record<string, string> = {};
    if (!companyId) found.companyId = "Elige el cliente.";
    if (!editing && form.externalRef.trim() && !RE_REF.test(form.externalRef.trim())) found.externalRef = "Sin espacios y máximo 80 caracteres.";
    if (!form.courier) found.courier = "Elige el courier.";
    if (!form.recipientName.trim()) found.recipientName = "Indica quién recibe.";
    if (form.recipientPhone.trim() && !RE_PHONE.test(form.recipientPhone.trim())) found.recipientPhone = "Teléfono inválido. Ejemplo: +56 9 1234 5678.";
    if (form.recipientEmail.trim() && !RE_EMAIL.test(form.recipientEmail.trim())) found.recipientEmail = "Correo inválido.";
    if (!pickup && (form.addressLine.trim().length < 5 || !/\d/.test(form.addressLine))) found.addressLine = "Indica calle y número.";
    if (!form.commune.trim()) found.commune = "Elige la comuna.";
    else if (communes.data && !commune) found.commune = "Elige una comuna de la lista.";
    if (!itemsLocked) {
      rows.forEach((row, index) => {
        const quantity = Number(row.quantity);
        if (!row.sku.trim()) found[`items[${index}]`] = "Indica el SKU.";
        else if (!Number.isInteger(quantity) || quantity <= 0) found[`items[${index}]`] = "La cantidad debe ser un entero mayor que 0.";
      });
    }
    return found;
  };

  // SKU escritos a mano: se confirma contra los productos activos de la empresa antes de enviar.
  const validateSkus = async () => {
    const found: Record<string, string> = {};
    const original = new Set(order?.items?.map((item) => item.sku.toLowerCase()));
    await Promise.all(rows.map(async (row, index) => {
      const sku = row.sku.trim();
      if (!sku || original.has(sku.toLowerCase())) return;
      const { data } = await listProducts(sku, companyId);
      if (!data.some((p) => p.active && p.sku.toLowerCase() === sku.toLowerCase())) found[`items[${index}]`] = "No está entre los productos activos de este cliente.";
    }));
    return found;
  };

  const items = () => rows.map((row) => ({ sku: row.sku.trim(), quantity: Number(row.quantity) }));
  const destination = () => ({
    recipientName: form.recipientName.trim(),
    recipientPhone: form.recipientPhone.trim(),
    recipientEmail: form.recipientEmail.trim(),
    addressLine: form.addressLine.trim(),
    commune: commune?.name ?? form.commune.trim(),
    regionCode: commune?.regionCode ?? order?.regionCode ?? "",
  });

  const changes = (): OrderChanges => {
    if (!order) return {};
    const out: OrderChanges = {};
    if (form.courier !== order.courier) out.courier = form.courier;
    if (form.dispatchType !== (order.dispatchType || "domicilio")) out.dispatchType = form.dispatchType;
    if (form.customerNotes.trim() !== (order.customerNotes ?? "")) out.customerNotes = form.customerNotes.trim();
    const next = destination();
    const current = { recipientName: order.recipientName ?? "", recipientPhone: order.recipientPhone ?? "", recipientEmail: order.recipientEmail ?? "", addressLine: order.addressLine ?? "", commune: order.commune ?? "", regionCode: order.regionCode ?? "" };
    if ((Object.keys(next) as (keyof typeof next)[]).some((key) => next[key] !== current[key])) out.destination = next;
    const canonical = (list: { sku: string; quantity: number }[]) => JSON.stringify(list.map((i) => [i.sku.toUpperCase(), i.quantity]).sort());
    if (!itemsLocked && canonical(items()) !== canonical(order.items ?? [])) out.items = items();
    return out;
  };

  const uploadAll = async (saved: Order) => {
    const results = await Promise.allSettled(documents.map((doc) => uploadDocument(saved.id, doc.file, doc.kind)));
    return documents.filter((_, index) => results[index].status === "rejected").map((doc) => doc.file.name);
  };

  const submit = async () => {
    setFormError(null);
    const found = validate();
    const edits = changes();
    const hasChanges = Object.keys(edits).length > 0;
    if (editing && hasChanges && !reason.trim()) found.reason = "Indica el motivo del cambio.";
    if (Object.keys(found).length) {
      setErrors(found);
      setFormError("Revisa los datos marcados.");
      return;
    }
    if (editing && !hasChanges && !documents.length) {
      showToast("No hay cambios que guardar.", "info");
      onClose();
      return;
    }
    setBusy(true);
    try {
      const skuErrors = editing && !edits.items ? {} : await validateSkus();
      if (Object.keys(skuErrors).length) {
        setErrors(skuErrors);
        setFormError("Revisa los datos marcados.");
        return;
      }
      setErrors({});
      let saved: Order;
      if (order) saved = hasChanges ? await updateOrder(order.id, { ...edits, reason: reason.trim() }) : order;
      else saved = await createOrder({ companyId, externalRef: form.externalRef.trim(), courier: form.courier, ...destination(), dispatchType: form.dispatchType, customerNotes: form.customerNotes.trim(), items: items() });
      const failedDocuments = await uploadAll(saved);
      if (failedDocuments.length) showToast(`${order ? "Orden guardada" : "Orden creada"}, pero no se pudo subir: ${failedDocuments.join(", ")}.`, "warning");
      else if (order) showToast(edits.items || edits.destination || edits.courier ? `Orden #${order.externalRef} actualizada. Se volverá a validar.` : `Orden #${order.externalRef} actualizada.`);
      else showToast("Orden creada (Pendiente).");
      onSaved(saved);
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) onConflict?.();
      if (e instanceof ApiError && e.fields.length) {
        setErrors(Object.fromEntries(e.fields.map((f) => [f.field, f.message])));
        setFormError("Revisa los datos marcados.");
      } else {
        setFormError(apiErrorMessage(e));
      }
    } finally {
      setBusy(false);
    }
  };

  const changeCompany = (value: number) => {
    setCompanyId(value);
    setRows([{ key: newKey(), sku: "", quantity: "1" }]); // cada cliente tiene sus productos
  };
  const clientName = companies.find((company) => company.id === companyId)?.name;

  return (
    <FormModal wide title={order ? `Editar orden • #${order.externalRef}` : "Nueva orden"} onClose={onClose} onSubmit={submit} submitLabel={order ? "Guardar cambios" : "Crear orden"} busy={busy} error={formError}>
      <div className="order-form">
        {order && <p className="form-notice">Si cambias productos, destino o courier, la orden se vuelve a validar.</p>}
        <section className="form-grid">
          <h3>Orden</h3>
          <Field label="Cliente" required error={errors.companyId}>
            {(id, invalid) => (editing || activeCompanies.length === 1 ? (
              <input id={id} value={clientName ?? ""} disabled />
            ) : (
              <select id={id} value={companyId || ""} aria-invalid={invalid} onChange={(e) => changeCompany(Number(e.target.value))}>
                <option value="">Elige un cliente</option>
                {activeCompanies.map((company) => <option key={company.id} value={company.id}>{company.name}</option>)}
              </select>
            ))}
          </Field>
          <Field label="N° de referencia externa" error={errors.externalRef}>
            {(id, invalid) => <input id={id} value={form.externalRef} disabled={editing} aria-invalid={invalid} placeholder={editing ? "" : "Si se omite, se genera NX-…"} onChange={(e) => set({ externalRef: e.target.value })} />}
          </Field>
          <Field label="Courier" required error={errors.courier}>
            {(id, invalid) => (
              <select id={id} value={form.courier} aria-invalid={invalid} onChange={(e) => set({ courier: e.target.value })}>
                <option value="">Elige un courier</option>
                {courierOptions.map((courier) => <option key={courier.code} value={courier.code}>{courier.name}</option>)}
              </select>
            )}
          </Field>
          <Field label="Tipo de despacho" error={errors.dispatchType}>
            {(id, invalid) => (
              <select id={id} value={form.dispatchType} aria-invalid={invalid} onChange={(e) => set({ dispatchType: e.target.value })}>
                {Object.entries(DISPATCH_LABELS).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
              </select>
            )}
          </Field>
        </section>

        <section className="form-grid">
          <h3>Destinatario</h3>
          <Field label="Nombre" required error={errors.recipientName}>
            {(id, invalid) => <input id={id} maxLength={120} value={form.recipientName} aria-invalid={invalid} onChange={(e) => set({ recipientName: e.target.value })} />}
          </Field>
          <Field label="Teléfono" error={errors.recipientPhone}>
            {(id, invalid) => <input id={id} value={form.recipientPhone} aria-invalid={invalid} placeholder="+56 9 1234 5678" onChange={(e) => set({ recipientPhone: e.target.value })} />}
          </Field>
          <Field label="Correo" error={errors.recipientEmail}>
            {(id, invalid) => <input id={id} type="email" value={form.recipientEmail} aria-invalid={invalid} onChange={(e) => set({ recipientEmail: e.target.value })} />}
          </Field>
        </section>

        <section className="form-grid">
          <h3>Dirección</h3>
          <div className="form-grid-full">
            <Field label="Calle y número" required={!pickup} error={errors.addressLine}>
              {(id, invalid) => <input id={id} value={form.addressLine} aria-invalid={invalid} onChange={(e) => set({ addressLine: e.target.value })} />}
            </Field>
          </div>
          <Field label="Comuna" required error={errors.commune}>
            {(id, invalid) => (
              <>
                <input id={id} list={`${id}-list`} value={form.commune} aria-invalid={invalid} onChange={(e) => set({ commune: e.target.value })} />
                <datalist id={`${id}-list`}>{communes.data?.map((c) => <option key={c.name} value={c.name} />)}</datalist>
              </>
            )}
          </Field>
          <Field label="Región">
            {(id) => <input id={id} value={commune?.regionCode ?? ""} disabled placeholder="Se completa según la comuna" />}
          </Field>
        </section>

        <section>
          <h3>Productos *</h3>
          {itemsLocked && <p className="form-notice">Esta orden ya estaba lista para retiro: los productos no se pueden cambiar.</p>}
          {errors.items && <span className="modal-field-error" role="alert">{errors.items}</span>}
          <div className="order-form-items">
            {rows.map((row, index) => (
              <ItemRow
                key={row.key}
                row={row}
                companyId={companyId}
                error={errors[`items[${index}]`]}
                disabled={itemsLocked}
                canRemove={rows.length > 1}
                onChange={(next) => setRows((current) => current.map((r) => (r.key === row.key ? { ...r, ...next } : r)))}
                onRemove={() => setRows((current) => current.filter((r) => r.key !== row.key))}
              />
            ))}
          </div>
          {!itemsLocked && (
            <button type="button" className="text-button order-form-add" onClick={() => setRows((current) => [...current, { key: newKey(), sku: "", quantity: "1" }])}>
              <Plus size={16} aria-hidden="true" /> Agregar producto
            </button>
          )}
        </section>

        <section className="order-form-stack">
          <Field label="Notas del cliente" error={errors.customerNotes}>
            {(id, invalid) => <textarea id={id} rows={2} maxLength={500} value={form.customerNotes} aria-invalid={invalid} onChange={(e) => set({ customerNotes: e.target.value })} />}
          </Field>
          <FileDrop label="Documentos adjuntos (opcional)" accept={DOCUMENT_EXTENSIONS} maxBytes={MAX_DOCUMENT_BYTES} value={null} onChange={(file) => { if (file) setDocuments((current) => [...current, { key: newKey(), file, kind: "factura" }]); }} />
          {documents.map((doc) => (
            <div key={doc.key} className="order-form-document">
              <span title={doc.file.name}>{doc.file.name}</span>
              <select aria-label={`Tipo de documento de ${doc.file.name}`} value={doc.kind} onChange={(e) => setDocuments((current) => current.map((d) => (d.key === doc.key ? { ...d, kind: e.target.value } : d)))}>
                {Object.entries(DOCUMENT_KINDS).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
              </select>
              <button type="button" className="toast-close" aria-label={`Quitar ${doc.file.name}`} onClick={() => setDocuments((current) => current.filter((d) => d.key !== doc.key))}><X size={18} /></button>
            </div>
          ))}
        </section>

        {order && (
          <section>
            <Field label={`Motivo del cambio (${MAX_REASON} máx.)`} required error={errors.reason}>
              {(id, invalid) => <textarea id={id} rows={2} maxLength={MAX_REASON} value={reason} aria-invalid={invalid} onChange={(e) => setReason(e.target.value)} />}
            </Field>
          </section>
        )}
      </div>
    </FormModal>
  );
}
