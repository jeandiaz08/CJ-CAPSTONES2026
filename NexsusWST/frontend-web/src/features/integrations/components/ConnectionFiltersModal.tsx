"use client";

import { useId, useState } from "react";
import { FiltersModal } from "@/components/FiltersModal";
import type { IntegrationKind } from "@/services/integrations";
import { supportedProviders } from "../providers";

export type ConnectionFilterValues = { company: string; kind: "" | IntegrationKind; provider: string; status: "" | "active" | "inactive" };
export const EMPTY_CONNECTION_FILTERS: ConnectionFilterValues = { company: "", kind: "", provider: "", status: "" };
export const countConnectionFilters = (f: ConnectionFilterValues) => Object.values(f).filter(Boolean).length;

// Proveedores sin repetir código («generic» existe como canal y como courier).
export function providerOptions(kind: "" | IntegrationKind) {
  const seen = new Set<string>();
  return supportedProviders(kind).filter((p) => !seen.has(p.code) && seen.add(p.code));
}

// Filtros de conexiones: empresa, tipo, proveedor y estado.
export function ConnectionFiltersModal({ values, companies, onApply, onClose }: {
  values: ConnectionFilterValues;
  companies: { id: number; name: string }[];
  onApply: (values: ConnectionFilterValues) => void;
  onClose: () => void;
}) {
  const id = useId();
  const [form, setForm] = useState(values);
  const providers = providerOptions(form.kind);
  return (
    <FiltersModal title="Filtros" onClose={onClose} footer={<>
      <button type="button" className="btn ghost" onClick={() => setForm(EMPTY_CONNECTION_FILTERS)}>Limpiar</button>
      <button type="button" className="btn primary" onClick={() => onApply(form)}>Aplicar filtros</button>
    </>}>
      <div className="modal-field">
        <label htmlFor={`${id}-company`}>Empresa</label>
        <select id={`${id}-company`} value={form.company} onChange={(e) => setForm({ ...form, company: e.target.value })}>
          <option value="">Todas</option>
          {companies.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
      </div>
      <div className="modal-field">
        <label htmlFor={`${id}-kind`}>Tipo</label>
        <select id={`${id}-kind`} value={form.kind} onChange={(e) => {
          const kind = e.target.value as ConnectionFilterValues["kind"];
          setForm({ ...form, kind, provider: providerOptions(kind).some((p) => p.code === form.provider) ? form.provider : "" });
        }}>
          <option value="">Todos</option>
          <option value="channel">Canal</option>
          <option value="courier">Courier</option>
        </select>
      </div>
      <div className="modal-field">
        <label htmlFor={`${id}-provider`}>Proveedor</label>
        <select id={`${id}-provider`} value={form.provider} onChange={(e) => setForm({ ...form, provider: e.target.value })}>
          <option value="">Todos</option>
          {providers.map((p) => <option key={p.code} value={p.code}>{p.name}</option>)}
        </select>
      </div>
      <div className="modal-field">
        <label htmlFor={`${id}-status`}>Estado</label>
        <select id={`${id}-status`} value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value as ConnectionFilterValues["status"] })}>
          <option value="">Todos</option>
          <option value="active">Activa</option>
          <option value="inactive">Inactiva</option>
        </select>
      </div>
    </FiltersModal>
  );
}
