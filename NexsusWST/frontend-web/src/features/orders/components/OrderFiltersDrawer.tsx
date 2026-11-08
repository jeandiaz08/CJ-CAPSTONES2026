"use client";

import { useId, useState } from "react";
import { FiltersModal } from "@/components/FiltersModal";
import { EMPTY_FILTERS, type FilterValues } from "../filters";
import { ORIGIN_OPTIONS, SITUATION_LABELS } from "../format";

type Option = { value: string; label: string };

// Filtros de la pestaña activa. Los cambios se aplican recién al pulsar «Aplicar».
// registry: Registro histórico, sin Origen ni «Solo en curso»; las fechas son las del cierre.
export function OrderFiltersDrawer({ assignment, registry = false, values, companies, couriers, statuses, onApply, onClose }: {
  assignment: boolean;
  registry?: boolean;
  values: FilterValues;
  companies: Option[] | null; // null: el usuario tiene una sola empresa y no se muestra
  couriers: Option[];
  statuses: Option[];
  onApply: (values: FilterValues) => void;
  onClose: () => void;
}) {
  const id = useId();
  const [draft, setDraft] = useState(values);
  const [dateError, setDateError] = useState<string | null>(null);
  const set = (changes: Partial<FilterValues>) => setDraft((current) => ({ ...current, ...changes }));
  const visibleStatuses = draft.onlyActive ? statuses.filter((status) => status.value !== "handed_over") : statuses;
  const toggleStatus = (status: string) => set({ statuses: draft.statuses.includes(status) ? draft.statuses.filter((value) => value !== status) : [...draft.statuses, status] });
  const toggleOnlyActive = (onlyActive: boolean) => set({ onlyActive, statuses: onlyActive ? draft.statuses.filter((status) => status !== "handed_over") : draft.statuses });
  const apply = () => {
    if (draft.from && draft.to && draft.from > draft.to) {
      setDateError("«Desde» no puede ser posterior a «Hasta».");
      return;
    }
    onApply(draft);
  };
  return (
    <FiltersModal title="Filtros" onClose={onClose} footer={<>
      <button type="button" className="btn ghost" onClick={() => onApply(EMPTY_FILTERS)}>Limpiar</button>
      <button type="button" className="btn form-submit-primary" onClick={apply}>Aplicar</button>
    </>}>
      <div className="filter-fields">
        {companies && (
          <div className="modal-field">
            <label htmlFor={`${id}-company`}>Cliente</label>
            <select id={`${id}-company`} value={draft.companyId} onChange={(e) => set({ companyId: e.target.value })}>
              <option value="">Todos</option>
              {companies.map((company) => <option key={company.value} value={company.value}>{company.label}</option>)}
            </select>
          </div>
        )}
        {registry ? null : assignment ? (
          <div className="modal-field">
            <label htmlFor={`${id}-situation`}>Situación</label>
            <select id={`${id}-situation`} value={draft.situation} onChange={(e) => set({ situation: e.target.value })}>
              <option value="">Todas</option>
              {Object.entries(SITUATION_LABELS).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
          </div>
        ) : (
          <div className="modal-field">
            <label htmlFor={`${id}-origin`}>Origen</label>
            <select id={`${id}-origin`} value={draft.origin} onChange={(e) => set({ origin: e.target.value })}>
              <option value="">Todos</option>
              {ORIGIN_OPTIONS.map((origin) => <option key={origin.value} value={origin.value}>{origin.label}</option>)}
            </select>
          </div>
        )}
        <div className="modal-field">
          <label htmlFor={`${id}-courier`}>Courier</label>
          <select id={`${id}-courier`} value={draft.courier} onChange={(e) => set({ courier: e.target.value })}>
            <option value="">Todos</option>
            {couriers.map((courier) => <option key={courier.value} value={courier.value}>{courier.label}</option>)}
          </select>
        </div>
        {!assignment && !registry && (
          <label className="filter-switch">
            <input type="checkbox" role="switch" checked={draft.onlyActive} onChange={(e) => toggleOnlyActive(e.target.checked)} />
            <span>Solo en curso</span>
            <small>Oculta las órdenes Entregadas</small>
          </label>
        )}
        <fieldset className="filter-statuses">
          <legend>Estado</legend>
          {visibleStatuses.map((status) => (
            <label key={status.value}>
              <input type="checkbox" checked={draft.statuses.includes(status.value)} onChange={() => toggleStatus(status.value)} />
              {status.label}
            </label>
          ))}
        </fieldset>
        <div className="filter-dates">
          <div className="modal-field">
            <label htmlFor={`${id}-from`}>{registry ? "Cerrada desde" : "Desde"}</label>
            <input id={`${id}-from`} type="date" value={draft.from} aria-invalid={!!dateError} onChange={(e) => { setDateError(null); set({ from: e.target.value }); }} />
          </div>
          <div className="modal-field">
            <label htmlFor={`${id}-to`}>{registry ? "Cerrada hasta" : "Hasta"}</label>
            <input id={`${id}-to`} type="date" value={draft.to} aria-invalid={!!dateError} onChange={(e) => { setDateError(null); set({ to: e.target.value }); }} />
          </div>
        </div>
        {dateError && <p className="modal-field-error" role="alert">{dateError}</p>}
      </div>
    </FiltersModal>
  );
}
