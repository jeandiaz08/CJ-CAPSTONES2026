"use client";

import { useId, useState } from "react";
import { FiltersModal } from "@/components/FiltersModal";
import { ROLE_LABELS } from "@/state/auth";

export type ActivityFilterValues = { from: string; to: string; actor: string; role: string; kind: string };

// El perfil Analista no se muestra en el front.
const ROLES = (["admin", "jefe_operaciones", "empaque", "cliente"] as const).map((value) => ({ value, label: ROLE_LABELS[value] }));

// Filtros del registro de acciones: se aplican al pulsar «Aplicar».
export function ActivityFiltersDrawer({ values, defaults, onApply, onClose }: {
  values: ActivityFilterValues;
  defaults: ActivityFilterValues;
  onApply: (values: ActivityFilterValues) => void;
  onClose: () => void;
}) {
  const id = useId();
  const [draft, setDraft] = useState(values);
  const [dateError, setDateError] = useState<string | null>(null);
  const set = (changes: Partial<ActivityFilterValues>) => setDraft((current) => ({ ...current, ...changes }));
  const apply = () => {
    if (draft.from && draft.to && draft.from > draft.to) return setDateError("«Desde» no puede ser posterior a «Hasta».");
    onApply(draft);
  };
  return (
    <FiltersModal title="Filtros" onClose={onClose} footer={<>
      <button type="button" className="btn ghost" onClick={() => onApply(defaults)}>Limpiar</button>
      <button type="button" className="btn form-submit-primary" onClick={apply}>Aplicar</button>
    </>}>
      <div className="filter-fields">
        <div className="filter-dates">
          <div className="modal-field">
            <label htmlFor={`${id}-from`}>Desde</label>
            <input id={`${id}-from`} type="date" value={draft.from} aria-invalid={!!dateError} onChange={(e) => { setDateError(null); set({ from: e.target.value }); }} />
          </div>
          <div className="modal-field">
            <label htmlFor={`${id}-to`}>Hasta</label>
            <input id={`${id}-to`} type="date" value={draft.to} aria-invalid={!!dateError} onChange={(e) => { setDateError(null); set({ to: e.target.value }); }} />
          </div>
        </div>
        {dateError && <p className="modal-field-error" role="alert">{dateError}</p>}
        <div className="modal-field">
          <label htmlFor={`${id}-actor`}>Actor</label>
          <input id={`${id}-actor`} value={draft.actor} placeholder="Nombre de quien hizo la acción" onChange={(e) => set({ actor: e.target.value })} />
        </div>
        <div className="modal-field">
          <label htmlFor={`${id}-role`}>Perfil</label>
          <select id={`${id}-role`} value={draft.role} onChange={(e) => set({ role: e.target.value })}>
            <option value="">Todos</option>
            {ROLES.map((r) => <option key={r.value} value={r.value}>{r.label}</option>)}
          </select>
        </div>
        <div className="modal-field">
          <label htmlFor={`${id}-kind`}>Humano/Sistema</label>
          <select id={`${id}-kind`} value={draft.kind} onChange={(e) => set({ kind: e.target.value })}>
            <option value="">Todos</option>
            <option value="human">Humano</option>
            <option value="system">Sistema</option>
          </select>
        </div>
      </div>
    </FiltersModal>
  );
}
