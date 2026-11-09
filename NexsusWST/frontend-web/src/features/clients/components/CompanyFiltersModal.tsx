"use client";

import { useId, useState } from "react";
import { FiltersModal } from "@/components/FiltersModal";
import type { WaveOption } from "../waves";

export type CompanyFilterValues = { status: "" | "active" | "inactive"; wave: string; from: string; to: string };
export const EMPTY_COMPANY_FILTERS: CompanyFilterValues = { status: "", wave: "", from: "", to: "" };

export const countCompanyFilters = (f: CompanyFilterValues) => [f.status, f.wave, f.from || f.to].filter(Boolean).length;

// Filtros de Empresas: estado, ola y fecha de creación.
export function CompanyFiltersModal({ values, waves, onApply, onClose }: {
  values: CompanyFilterValues;
  waves: WaveOption[];
  onApply: (values: CompanyFilterValues) => void;
  onClose: () => void;
}) {
  const id = useId();
  const [form, setForm] = useState(values);
  const [error, setError] = useState<string | null>(null);
  const apply = () => {
    if (form.from && form.to && form.from > form.to) {
      setError("La fecha «desde» no puede ser posterior a «hasta».");
      return;
    }
    onApply(form);
  };
  return (
    <FiltersModal title="Filtros" onClose={onClose} footer={<>
      <button type="button" className="btn ghost" onClick={() => setForm(EMPTY_COMPANY_FILTERS)}>Limpiar</button>
      <button type="button" className="btn primary" onClick={apply}>Aplicar filtros</button>
    </>}>
      <div className="modal-field">
        <label htmlFor={`${id}-status`}>Estado</label>
        <select id={`${id}-status`} value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value as CompanyFilterValues["status"] })}>
          <option value="">Todos</option>
          <option value="active">Activa</option>
          <option value="inactive">Inactiva</option>
        </select>
      </div>
      <div className="modal-field">
        <label htmlFor={`${id}-wave`}>Ola</label>
        <select id={`${id}-wave`} value={form.wave} onChange={(e) => setForm({ ...form, wave: e.target.value })}>
          <option value="">Todas</option>
          {waves.map((w) => <option key={w.value} value={w.value}>{w.label}</option>)}
        </select>
      </div>
      <fieldset className="filter-dates">
        <legend>Fecha de creación</legend>
        <div className="modal-field">
          <label htmlFor={`${id}-from`}>Desde</label>
          <input id={`${id}-from`} type="date" value={form.from} onChange={(e) => setForm({ ...form, from: e.target.value })} />
        </div>
        <div className="modal-field">
          <label htmlFor={`${id}-to`}>Hasta</label>
          <input id={`${id}-to`} type="date" value={form.to} onChange={(e) => setForm({ ...form, to: e.target.value })} />
        </div>
      </fieldset>
      {error && <p className="modal-field-error" role="alert">{error}</p>}
    </FiltersModal>
  );
}
