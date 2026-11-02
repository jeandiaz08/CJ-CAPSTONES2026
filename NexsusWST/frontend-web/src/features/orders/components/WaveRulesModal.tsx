"use client";

import { useId, useState, type DragEvent } from "react";
import { ArrowDown, ArrowUp, GripVertical, Pencil, Plus, Trash2 } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { FormModal } from "@/components/FormModal";
import { showToast } from "@/components/Toast";
import { apiErrorMessage } from "@/services/api";
import type { CatalogItem, Company } from "@/services/core";
import { deleteWaveRule, getWarehouses, listWaveRules, listWaves, reorderWaveRules, saveWaveRule, type WaveRule } from "@/services/orders";
import { channelLabel, DISPATCH_LABELS } from "../format";
import { useOrderResource } from "../hooks/useOrderResources";

const CHANNELS = ["manual", "mercadolibre", "shopify"];
const EMPTY_RULE: WaveRule = { id: 0, name: "", position: 0, active: true, courier: "", companyId: null, channel: "", dispatchType: "", warehouse: "", waveCode: "" };

function RuleForm({ rule, companies, couriers, onClose, onSaved }: {
  rule: WaveRule | null;
  companies: Company[];
  couriers: CatalogItem[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const id = useId();
  const [form, setForm] = useState<WaveRule>(rule ?? EMPTY_RULE);
  const [errors, setErrors] = useState<{ name?: string; waveCode?: string }>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const waves = useOrderResource(listWaves, "waves");
  const warehouses = useOrderResource(getWarehouses, "warehouses");
  const set = (changes: Partial<WaveRule>) => setForm((current) => ({ ...current, ...changes }));
  const knownWarehouses = warehouses.data?.data ?? [];
  const warehouse = form.warehouse.trim().toLowerCase();
  const unknownWarehouse = warehouses.data && warehouse !== "" && !knownWarehouses.some((value) => value.toLowerCase() === warehouse);
  const waveOptions = [
    ...(waves.data?.data ?? []),
    ...(form.waveCode && !waves.data?.data.some((wave) => wave.code === form.waveCode) ? [{ id: form.waveCode, code: form.waveCode, name: form.waveCode, type: "" }] : []),
  ];
  const submit = async () => {
    setFormError(null);
    const found = { ...(form.name.trim() ? {} : { name: "Indica el nombre de la regla." }), ...(form.waveCode ? {} : { waveCode: "Elige la ola." }) };
    setErrors(found);
    if (Object.keys(found).length) return;
    setBusy(true);
    try {
      await saveWaveRule(form);
      showToast(rule ? "Regla actualizada." : "Regla creada.");
      onSaved();
    } catch (e) {
      setFormError(apiErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <FormModal title={rule ? "Editar regla" : "Nueva regla"} onClose={onClose} onSubmit={submit} submitLabel={rule ? "Guardar regla" : "Crear regla"} busy={busy} error={formError}>
      <div className="filter-fields">
        <div className="modal-field">
          <label htmlFor={`${id}-name`}>Nombre *</label>
          <input id={`${id}-name`} maxLength={80} value={form.name} aria-invalid={!!errors.name} onChange={(e) => set({ name: e.target.value })} />
          {errors.name && <span className="modal-field-error" role="alert">{errors.name}</span>}
        </div>
        <p className="form-hint">Las condiciones vacías aceptan cualquier valor.</p>
        <div className="filter-dates">
          <div className="modal-field">
            <label htmlFor={`${id}-courier`}>Courier</label>
            <select id={`${id}-courier`} value={form.courier} onChange={(e) => set({ courier: e.target.value })}>
              <option value="">Cualquiera</option>
              {couriers.map((courier) => <option key={courier.code} value={courier.code}>{courier.name}</option>)}
            </select>
          </div>
          <div className="modal-field">
            <label htmlFor={`${id}-company`}>Cliente</label>
            <select id={`${id}-company`} value={form.companyId ?? ""} onChange={(e) => set({ companyId: e.target.value ? Number(e.target.value) : null })}>
              <option value="">Cualquiera</option>
              {companies.map((company) => <option key={company.id} value={company.id}>{company.name}</option>)}
            </select>
          </div>
          <div className="modal-field">
            <label htmlFor={`${id}-channel`}>Canal</label>
            <select id={`${id}-channel`} value={form.channel} onChange={(e) => set({ channel: e.target.value })}>
              <option value="">Cualquiera</option>
              {[...new Set([...CHANNELS, ...(form.channel ? [form.channel] : [])])].map((channel) => <option key={channel} value={channel}>{channelLabel(channel)}</option>)}
            </select>
          </div>
          <div className="modal-field">
            <label htmlFor={`${id}-dispatch`}>Tipo de despacho</label>
            <select id={`${id}-dispatch`} value={form.dispatchType} onChange={(e) => set({ dispatchType: e.target.value })}>
              <option value="">Cualquiera</option>
              {Object.entries(DISPATCH_LABELS).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
          </div>
        </div>
        <div className="modal-field">
          <label htmlFor={`${id}-warehouse`}>Bodega</label>
          <input id={`${id}-warehouse`} list={`${id}-warehouses`} value={form.warehouse} placeholder="Cualquiera" aria-describedby={unknownWarehouse ? `${id}-warehouse-notice` : undefined} onChange={(e) => set({ warehouse: e.target.value })} />
          <datalist id={`${id}-warehouses`}>{knownWarehouses.map((value) => <option key={value} value={value} />)}</datalist>
          {unknownWarehouse && <span id={`${id}-warehouse-notice`} className="form-warning" role="status">Esta bodega no aparece en ninguna orden</span>}
        </div>
        <div className="modal-field">
          <label htmlFor={`${id}-wave`}>Ola de Invas *</label>
          <select id={`${id}-wave`} value={form.waveCode} aria-invalid={!!errors.waveCode} onChange={(e) => set({ waveCode: e.target.value })}>
            <option value="">Elige una ola</option>
            {waveOptions.map((wave) => <option key={wave.code} value={wave.code}>{wave.name === wave.code ? wave.code : `${wave.name} (${wave.code})`}</option>)}
          </select>
          {errors.waveCode && <span className="modal-field-error" role="alert">{errors.waveCode}</span>}
          {waves.error && <span className="modal-field-error" role="alert">No se pudieron cargar las olas de Invas: {waves.error}</span>}
        </div>
      </div>
    </FormModal>
  );
}

// «Configurar asignación»: gana la primera regla que coincide, por eso importa el orden.
export function WaveRulesModal({ companies, couriers, onClose, onChanged }: {
  companies: Company[];
  couriers: CatalogItem[];
  onClose: () => void;
  onChanged: () => void;
}) {
  const [version, setVersion] = useState(0);
  const resource = useOrderResource(listWaveRules, `wave-rules:${version}`, "wave-rules");
  const [draft, setDraft] = useState<WaveRule[] | null>(null); // orden provisional mientras se arrastra
  const [dragging, setDragging] = useState<number | null>(null);
  const [editing, setEditing] = useState<WaveRule | "new" | null>(null);
  const [deleting, setDeleting] = useState<WaveRule | null>(null);
  const [error, setError] = useState<string | null>(null);
  const rules = draft ?? resource.data?.data ?? [];
  const refresh = () => { setVersion((v) => v + 1); onChanged(); };
  const courierName = (code: string) => couriers.find((c) => c.code === code)?.name ?? code;
  const companyName = (id: number) => companies.find((c) => c.id === id)?.name ?? `Empresa ${id}`;
  const summary = (rule: WaveRule) => [
    rule.courier && courierName(rule.courier),
    rule.companyId != null && companyName(rule.companyId),
    rule.channel && channelLabel(rule.channel),
    rule.dispatchType && DISPATCH_LABELS[rule.dispatchType],
    rule.warehouse,
  ].filter(Boolean).join(" · ") || "Cualquier orden";

  const commit = async (next: WaveRule[]) => {
    setError(null);
    setDraft(next);
    try {
      await reorderWaveRules(next.map((rule) => rule.id));
      showToast("Orden de las reglas actualizado.");
      refresh();
    } catch (e) {
      setError(apiErrorMessage(e));
      setVersion((v) => v + 1);
    } finally {
      setDraft(null);
    }
  };
  const move = (from: number, to: number) => {
    const next = [...rules];
    const [rule] = next.splice(from, 1);
    next.splice(to, 0, rule);
    return next;
  };
  const dragOver = (e: DragEvent, index: number) => {
    e.preventDefault();
    if (dragging === null || dragging === index) return;
    setDraft(move(dragging, index));
    setDragging(index);
  };
  const drop = () => {
    setDragging(null);
    if (draft) commit(draft);
  };
  const toggle = async (rule: WaveRule) => {
    setError(null);
    try {
      await saveWaveRule({ ...rule, active: !rule.active });
      showToast(rule.active ? `Regla «${rule.name}» pausada.` : `Regla «${rule.name}» activada.`);
      refresh();
    } catch (e) {
      setError(apiErrorMessage(e));
    }
  };

  return (
    <FormModal wide title="Configurar asignación" onClose={onClose} error={error}>
      <div className="config-toolbar">
        <p className="form-notice">Gana la primera regla que coincide. Arrastra o usa las flechas para cambiar el orden.</p>
        <button type="button" className="btn form-submit-primary" onClick={() => setEditing("new")}><Plus size={18} aria-hidden="true" />Nueva regla</button>
      </div>
      {resource.error && <p className="modal-field-error" role="alert">No se pudieron cargar las reglas: {resource.error}</p>}
      {resource.loading && <div className="detail-skeleton config-skeleton" aria-label="Cargando reglas" />}
      {!resource.loading && !resource.error && rules.length === 0 && <p className="pad">Aún no hay reglas. Sin reglas, las órdenes quedan para asignarlas a mano.</p>}
      <ol className="config-list">
        {rules.map((rule, index) => (
          <li key={rule.id} className={`config-row ${rule.active ? "" : "is-paused"} ${dragging === index ? "is-dragging" : ""}`} draggable onDragStart={() => setDragging(index)} onDragOver={(e) => dragOver(e, index)} onDrop={drop} onDragEnd={() => { setDragging(null); setDraft(null); }}>
            <GripVertical size={18} className="config-grip" aria-hidden="true" />
            <span className="config-position">{index + 1}</span>
            <div className="config-text">
              <strong>{rule.name}</strong>
              <small>{summary(rule)} → Ola {rule.waveCode}</small>
            </div>
            <label className="filter-switch config-switch">
              <input type="checkbox" role="switch" aria-label={`Regla ${rule.name} activa`} checked={rule.active} onChange={() => toggle(rule)} />
              <span>{rule.active ? "Activa" : "Pausada"}</span>
            </label>
            <div className="config-actions">
              <button type="button" className="toast-close" aria-label={`Subir ${rule.name}`} disabled={index === 0} onClick={() => commit(move(index, index - 1))}><ArrowUp size={18} /></button>
              <button type="button" className="toast-close" aria-label={`Bajar ${rule.name}`} disabled={index === rules.length - 1} onClick={() => commit(move(index, index + 1))}><ArrowDown size={18} /></button>
              <button type="button" className="toast-close" aria-label={`Editar ${rule.name}`} onClick={() => setEditing(rule)}><Pencil size={18} /></button>
              <button type="button" className="toast-close" aria-label={`Eliminar ${rule.name}`} onClick={() => setDeleting(rule)}><Trash2 size={18} /></button>
            </div>
          </li>
        ))}
      </ol>
      {editing && <RuleForm rule={editing === "new" ? null : editing} companies={companies} couriers={couriers} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); refresh(); }} />}
      {deleting && (
        <ConfirmDialog title={`Eliminar regla «${deleting.name}»`} confirmLabel="Eliminar regla" destructive onClose={() => setDeleting(null)} onConfirm={async () => {
          await deleteWaveRule(deleting.id);
          showToast("Regla eliminada.");
          setDeleting(null);
          refresh();
        }}>
          <p>Las órdenes que dependían de esta regla volverán a evaluarse con las demás.</p>
        </ConfirmDialog>
      )}
    </FormModal>
  );
}
