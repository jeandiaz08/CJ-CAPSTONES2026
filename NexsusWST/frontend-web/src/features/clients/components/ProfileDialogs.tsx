"use client";

import { useId, useState } from "react";
import { FileDrop } from "@/components/FileDrop";
import { FiltersModal } from "@/components/FiltersModal";
import { FormModal } from "@/components/FormModal";
import { apiErrorMessage } from "@/services/api";
import { importProfiles, type ImportResult, type ProfileScope } from "@/services/clients";

// Filtros de Perfiles: empresa, estado y fecha de creación.
export type ProfileFilterValues = { companyId: string; status: "" | "active" | "inactive"; from: string; to: string };
export const EMPTY_PROFILE_FILTERS: ProfileFilterValues = { companyId: "", status: "", from: "", to: "" };
export const countProfileFilters = (f: ProfileFilterValues) => [f.companyId, f.status, f.from || f.to].filter(Boolean).length;

export function ProfileFiltersModal({ values, companies, onApply, onClose }: {
  values: ProfileFilterValues;
  companies?: { id: number; name: string }[]; // sin empresas (Cliente), no se muestra el filtro
  onApply: (values: ProfileFilterValues) => void;
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
      <button type="button" className="btn ghost" onClick={() => setForm(EMPTY_PROFILE_FILTERS)}>Limpiar</button>
      <button type="button" className="btn primary" onClick={apply}>Aplicar filtros</button>
    </>}>
      {companies && <div className="modal-field">
        <label htmlFor={`${id}-company`}>Empresa</label>
        <select id={`${id}-company`} value={form.companyId} onChange={(e) => setForm({ ...form, companyId: e.target.value })}>
          <option value="">Todas</option>
          {companies.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
      </div>}
      <div className="modal-field">
        <label htmlFor={`${id}-status`}>Estado</label>
        <select id={`${id}-status`} value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value as ProfileFilterValues["status"] })}>
          <option value="">Todos</option>
          <option value="active">Activo</option>
          <option value="inactive">Inactivo</option>
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

// «Importar perfiles»: crea las filas válidas e informa las inválidas con su fila y motivo.
export function ImportProfilesDialog({ scope, onClose, onImported }: { scope: ProfileScope; onClose: () => void; onImported: () => void }) {
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);
  const submit = async () => {
    if (!file) {
      setError("Elige el archivo .xlsx con los perfiles.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const res = await importProfiles(scope, file);
      setResult(res);
      if (res.created.length) onImported();
    } catch (e) {
      setError(apiErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  if (result) {
    return (
      <FormModal title="Resultado de la importación" onClose={onClose} closeLabel="Cerrar" wide>
        <div className="import-result">
          <p><strong>{result.created.length}</strong> {result.created.length === 1 ? "perfil creado" : "perfiles creados"} · <strong>{result.errors.length}</strong> {result.errors.length === 1 ? "fila" : "filas"} con error</p>
          {result.errors.length > 0 && (
            <div className="import-errors">
              <table>
                <thead><tr><th>Fila</th><th>Motivo</th></tr></thead>
                <tbody>{result.errors.map((e, i) => <tr key={`${e.row}-${i}`}><td>{e.row}</td><td>{e.message}</td></tr>)}</tbody>
              </table>
            </div>
          )}
        </div>
      </FormModal>
    );
  }
  return (
    <FormModal title="Importar perfiles" onClose={onClose} onSubmit={submit} submitLabel="Importar" busy={busy} error={error}>
      <p className="form-hint">Usa la plantilla de perfiles (el correo es obligatorio). Las filas válidas se crean y las inválidas se informan con su fila y motivo.</p>
      <FileDrop label="Archivo de perfiles (.xlsx)" accept={[".xlsx"]} maxBytes={5 << 20} value={file} onChange={setFile} />
    </FormModal>
  );
}
