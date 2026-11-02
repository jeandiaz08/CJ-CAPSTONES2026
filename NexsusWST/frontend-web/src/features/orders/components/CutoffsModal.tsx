"use client";

import { useId, useState } from "react";
import { Pencil, Plus, Trash2, X } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { FormModal } from "@/components/FormModal";
import { showToast } from "@/components/Toast";
import { apiErrorMessage } from "@/services/api";
import type { CatalogItem } from "@/services/core";
import { deleteCutoff, getCutoffs, getOperationSchedule, saveAssignmentDeadline, saveCutoff, saveOperationSchedule, type Cutoff } from "@/services/orders";
import { useOrderResource } from "../hooks/useOrderResources";

type CourierRow = { key: number; code: string; arrivalTime: string };

const DAYS = [["mon", "L", "Lunes"], ["tue", "M", "Martes"], ["wed", "X", "Miércoles"], ["thu", "J", "Jueves"], ["fri", "V", "Viernes"], ["sat", "S", "Sábado"], ["sun", "D", "Domingo"]];

function CutoffForm({ cutoff, others, deadline, start, couriers, onClose, onSaved }: {
  cutoff: Cutoff | null;
  others: Cutoff[];
  deadline?: string;
  start?: string;
  couriers: CatalogItem[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const id = useId();
  const [name, setName] = useState(cutoff?.name ?? "");
  const [time, setTime] = useState(cutoff?.cutoffTime ?? "");
  const [nextKey, setNextKey] = useState((cutoff?.couriers.length ?? 0) + 1);
  const [rows, setRows] = useState<CourierRow[]>(cutoff?.couriers.map((c, index) => ({ key: index, code: c.code, arrivalTime: c.arrivalTime })) ?? [{ key: 0, code: "", arrivalTime: "" }]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const options = (row: CourierRow) => [
    ...couriers,
    ...(row.code && !couriers.some((c) => c.code === row.code) ? [{ code: row.code, name: row.code }] : []),
  ].filter((c) => c.code === row.code || !rows.some((other) => other.code === c.code));

  const validate = () => {
    const found: Record<string, string> = {};
    if (!time) found.time = "Indica la hora de corte.";
    else if (others.some((other) => other.cutoffTime === time)) found.time = "Ya existe un corte a esa hora.";
    else if (deadline && time > deadline) found.time = `No puede ser posterior a la hora límite (${deadline}).`;
    else if (start && time <= start) found.time = `Debe ser posterior al inicio de operación (${start}).`;
    if (!rows.length) found.couriers = "Agrega al menos un courier.";
    rows.forEach((row) => {
      if (!row.code) found[`row-${row.key}`] = "Elige el courier.";
      else if (!row.arrivalTime) found[`row-${row.key}`] = "Indica la hora de llegada.";
      else if (time && row.arrivalTime < time) found[`row-${row.key}`] = "La llegada no puede ser anterior al corte.";
    });
    return found;
  };
  const submit = async () => {
    setFormError(null);
    const found = validate();
    setErrors(found);
    if (Object.keys(found).length) return;
    setBusy(true);
    try {
      await saveCutoff(cutoff?.id ?? null, { name: name.trim() || null, cutoffTime: time, couriers: rows.map((row) => ({ code: row.code, arrivalTime: row.arrivalTime })) });
      showToast(cutoff ? "Corte actualizado." : "Corte creado.");
      onSaved();
    } catch (e) {
      setFormError(apiErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <FormModal title={cutoff ? "Editar corte" : "Nuevo corte"} onClose={onClose} onSubmit={submit} submitLabel={cutoff ? "Guardar corte" : "Crear corte"} busy={busy} error={formError}>
      <div className="filter-fields">
        <div className="filter-dates">
          <div className="modal-field">
            <label htmlFor={`${id}-name`}>Nombre</label>
            <input id={`${id}-name`} value={name} placeholder={cutoff ? `Corte ${cutoff.number}` : "Corte N"} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="modal-field">
            <label htmlFor={`${id}-time`}>Hora de corte *</label>
            <input id={`${id}-time`} type="time" value={time} aria-invalid={!!errors.time} onChange={(e) => setTime(e.target.value)} />
            {errors.time && <span className="modal-field-error" role="alert">{errors.time}</span>}
          </div>
        </div>
        <fieldset className="config-couriers">
          <legend>Couriers y hora de llegada *</legend>
          {errors.couriers && <span className="modal-field-error" role="alert">{errors.couriers}</span>}
          {rows.map((row) => (
            <div key={row.key} className="config-courier">
              <div className="config-courier-fields">
                <select aria-label="Courier" className="config-input" value={row.code} aria-invalid={!!errors[`row-${row.key}`]} onChange={(e) => setRows((current) => current.map((r) => (r.key === row.key ? { ...r, code: e.target.value } : r)))}>
                  <option value="">Elige un courier</option>
                  {options(row).map((courier) => <option key={courier.code} value={courier.code}>{courier.name}</option>)}
                </select>
                <input aria-label="Hora de llegada" className="config-input" type="time" value={row.arrivalTime} onChange={(e) => setRows((current) => current.map((r) => (r.key === row.key ? { ...r, arrivalTime: e.target.value } : r)))} />
                <button type="button" className="toast-close" aria-label="Quitar courier" onClick={() => setRows((current) => current.filter((r) => r.key !== row.key))}><X size={18} /></button>
              </div>
              {errors[`row-${row.key}`] && <span className="modal-field-error" role="alert">{errors[`row-${row.key}`]}</span>}
            </div>
          ))}
          <button type="button" className="text-button order-form-add" onClick={() => { setRows((current) => [...current, { key: nextKey, code: "", arrivalTime: "" }]); setNextKey((k) => k + 1); }}>
            <Plus size={16} aria-hidden="true" /> Agregar courier
          </button>
        </fieldset>
      </div>
    </FormModal>
  );
}

// «Configurar cortes»: cortes activos y hora límite. El listado se ordena por hora en la API.
export function CutoffsModal({ couriers, onClose, onChanged }: { couriers: CatalogItem[]; onClose: () => void; onChanged: () => void }) {
  const id = useId();
  const [version, setVersion] = useState(0);
  const cuts = useOrderResource(getCutoffs, `config-cutoffs:${version}`, "config-cutoffs");
  const schedule = useOrderResource(getOperationSchedule, `config-operation:${version}`, "config-operation");
  const [editing, setEditing] = useState<Cutoff | "new" | null>(null);
  const [removing, setRemoving] = useState<Cutoff | null>(null);
  const [deadlineDraft, setDeadlineDraft] = useState<string | null>(null);
  const [deadlineError, setDeadlineError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [startDraft, setStartDraft] = useState<string | null>(null);
  const [daysDraft, setDaysDraft] = useState<string[] | null>(null);
  const [operationError, setOperationError] = useState<string | null>(null);
  const list = cuts.data?.data ?? [];
  const currentDeadline = schedule.data?.data.assignmentDeadline;
  const startValue = startDraft ?? schedule.data?.data.operationStart ?? "";
  const daysValue = daysDraft ?? schedule.data?.data.operationDays ?? [];
  const deadlineValue = deadlineDraft ?? currentDeadline ?? "";
  const refresh = () => { setVersion((v) => v + 1); onChanged(); };
  const saveDeadline = async () => {
    setDeadlineError(null);
    const latest = list.at(-1)?.cutoffTime;
    if (!deadlineValue) return setDeadlineError("Indica la hora límite.");
    if (latest && deadlineValue < latest) return setDeadlineError(`No puede ser anterior al último corte (${latest}).`);
    setSaving(true);
    try {
      await saveAssignmentDeadline(deadlineValue);
      showToast("Hora límite actualizada.");
      setDeadlineDraft(null);
      refresh();
    } catch (e) {
      setDeadlineError(apiErrorMessage(e));
    } finally {
      setSaving(false);
    }
  };
  const saveOperation = async () => {
    setOperationError(null);
    const first = list[0]?.cutoffTime;
    if (!startValue) return setOperationError("Indica el inicio de operación.");
    if (first && startValue >= first) return setOperationError(`Debe ser anterior al primer corte (${first}).`);
    if (daysValue.length === 0) return setOperationError("Elige al menos un día de operación.");
    setSaving(true);
    try {
      await saveOperationSchedule({ operationStart: startValue, operationDays: daysValue });
      showToast("Horario de operación actualizado.");
      setStartDraft(null);
      setDaysDraft(null);
      refresh();
    } catch (e) {
      setOperationError(apiErrorMessage(e));
    } finally {
      setSaving(false);
    }
  };
  return (
    <FormModal wide title="Configurar cortes" onClose={onClose}>
      <div className="config-toolbar">
        <p className="form-notice">Cada corte indica qué couriers pasan y a qué hora llegan.</p>
        <button type="button" className="btn form-submit-primary" onClick={() => setEditing("new")}><Plus size={18} aria-hidden="true" />Agregar corte</button>
      </div>
      {cuts.error && <p className="modal-field-error" role="alert">No se pudieron cargar los cortes: {cuts.error}</p>}
      {cuts.loading && <div className="detail-skeleton config-skeleton" aria-label="Cargando cortes" />}
      {!cuts.loading && !cuts.error && list.length === 0 && <p className="pad">Aún no hay cortes configurados.</p>}
      <ul className="config-list">
        {list.map((cutoff) => (
          <li key={cutoff.id} className="config-row">
            <span className="config-time">{cutoff.cutoffTime}</span>
            <div className="config-text">
              <strong>{cutoff.name || `Corte ${cutoff.number}`}</strong>
              <small>{cutoff.couriers.map((c) => `${c.name} · llega ${c.arrivalTime}`).join(" | ")}</small>
            </div>
            <div className="config-actions">
              <button type="button" className="toast-close" aria-label={`Editar ${cutoff.name || `corte ${cutoff.number}`}`} onClick={() => setEditing(cutoff)}><Pencil size={18} /></button>
              <button type="button" className="toast-close" aria-label={`Desactivar ${cutoff.name || `corte ${cutoff.number}`}`} onClick={() => setRemoving(cutoff)}><Trash2 size={18} /></button>
            </div>
          </li>
        ))}
      </ul>
      <div className="config-deadline">
        <div className="modal-field">
          <label htmlFor={`${id}-deadline`}>Hora límite del último corte</label>
          <input id={`${id}-deadline`} type="time" value={deadlineValue} aria-invalid={!!deadlineError} onChange={(e) => { setDeadlineError(null); setDeadlineDraft(e.target.value); }} />
        </div>
        <button type="button" className="btn ghost" disabled={saving || deadlineDraft === null || deadlineDraft === currentDeadline} onClick={saveDeadline}>Guardar hora límite</button>
        <p className="form-hint">No se asignan órdenes después de esta hora.</p>
        {deadlineError && <p className="modal-field-error" role="alert">{deadlineError}</p>}
      </div>
      <div className="config-deadline">
        <div className="modal-field">
          <label htmlFor={`${id}-start`}>Inicio de operación</label>
          <input id={`${id}-start`} type="time" value={startValue} aria-invalid={!!operationError} onChange={(e) => { setOperationError(null); setStartDraft(e.target.value); }} />
        </div>
        <fieldset className="config-days">
          <legend>Días de operación</legend>
          {DAYS.map(([code, letter, name]) => (
            <label key={code} title={name}>
              <input type="checkbox" aria-label={name} checked={daysValue.includes(code)} onChange={() => { setOperationError(null); setDaysDraft(daysValue.includes(code) ? daysValue.filter((day) => day !== code) : [...daysValue, code]); }} />
              <span aria-hidden="true">{letter}</span>
            </label>
          ))}
        </fieldset>
        <button type="button" className="btn ghost" disabled={saving || (startDraft === null && daysDraft === null)} onClick={saveOperation}>Guardar horario</button>
        <p className="form-hint">Lo retenido sale el próximo día de operación a esta hora, por corte, prioridad y antigüedad.</p>
        {operationError && <p className="modal-field-error" role="alert">{operationError}</p>}
      </div>
      {editing && <CutoffForm cutoff={editing === "new" ? null : editing} others={list.filter((c) => editing === "new" || c.id !== editing.id)} deadline={currentDeadline} start={schedule.data?.data.operationStart} couriers={couriers} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); refresh(); }} />}
      {removing && (
        <ConfirmDialog title={`Desactivar ${removing.name || `corte ${removing.number}`}`} confirmLabel="Desactivar corte" destructive onClose={() => setRemoving(null)} onConfirm={async () => {
          await deleteCutoff(removing.id);
          showToast("Corte desactivado.");
          setRemoving(null);
          refresh();
        }}>
          <p>El corte de las {removing.cutoffTime} dejará de aplicarse. Su historial se conserva.</p>
        </ConfirmDialog>
      )}
    </FormModal>
  );
}
