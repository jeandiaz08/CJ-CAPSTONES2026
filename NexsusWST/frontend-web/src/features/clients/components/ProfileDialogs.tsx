"use client";

import { useId, useState } from "react";
import { Copy } from "lucide-react";
import { FileDrop } from "@/components/FileDrop";
import { FiltersModal } from "@/components/FiltersModal";
import { FormModal } from "@/components/FormModal";
import { apiErrorMessage } from "@/services/api";
import { importProfiles, type ProfileScope } from "@/services/clients";

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

// Carga masiva genérica (perfiles y productos): elige el .xlsx, importa y muestra el resultado con
// las filas creadas y las inválidas con su fila y motivo.
export type ImportOutcome = { created: unknown[]; errors: { row: number; field?: string; message: string }[] };

export function ImportDialog({ title, hint, fileLabel, missingFile, created, onImport, onClose, onImported }: {
  title: string;
  hint: string;
  fileLabel: string;
  missingFile: string;
  created: (n: number) => string; // «3 perfiles creados»
  onImport: (file: File) => Promise<ImportOutcome>;
  onClose: () => void;
  onImported: () => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<ImportOutcome | null>(null);
  const submit = async () => {
    if (!file) {
      setError(missingFile);
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const res = await onImport(file);
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
          <p><strong>{result.created.length}</strong> {created(result.created.length)} · <strong>{result.errors.length}</strong> {result.errors.length === 1 ? "fila" : "filas"} con error</p>
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
    <FormModal title={title} onClose={onClose} onSubmit={submit} submitLabel="Importar" busy={busy} error={error}>
      <p className="form-hint">{hint}</p>
      <FileDrop label={fileLabel} accept={[".xlsx"]} maxBytes={5 << 20} value={file} onChange={setFile} />
    </FormModal>
  );
}

// «Importar perfiles»: crea las filas válidas e informa las inválidas con su fila y motivo.
export function ImportProfilesDialog({ scope, onClose, onImported }: { scope: ProfileScope; onClose: () => void; onImported: () => void }) {
  return (
    <ImportDialog title="Importar perfiles" fileLabel="Archivo de perfiles (.xlsx)" missingFile="Elige el archivo .xlsx con los perfiles."
      hint={scope === "internal"
        ? "Usa la plantilla de perfiles WeStorage (correo y rol obligatorios; el personal sin correo se crea con «Nuevo perfil»). Las filas válidas se crean y las inválidas se informan con su fila y motivo."
        : "Usa la plantilla de perfiles (el correo es obligatorio). Las filas válidas se crean y las inválidas se informan con su fila y motivo."}
      created={(n) => (n === 1 ? "perfil creado" : "perfiles creados")}
      onImport={(file) => importProfiles(scope, file)} onClose={onClose} onImported={onImported} />
  );
}

// Contraseña temporal de un perfil interno sin correo (SCRUM-231): se muestra una sola vez y no se
// guarda en NexusWST.
export function TemporaryPasswordDialog({ name, login, password, onClose }: { name: string; login: string; password: string; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(password);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };
  return (
    <FormModal title="Contraseña temporal" onClose={onClose} closeLabel="Listo">
      <p>Entrégale a <strong>{name}</strong> (usuario <strong>{login}</strong>) esta contraseña. Deberá cambiarla al ingresar.</p>
      <p className="temp-password"><code>{password}</code>
        <button type="button" className="text-button" onClick={copy}><Copy size={16} aria-hidden="true" />{copied ? "Copiada" : "Copiar"}</button>
      </p>
      <p className="confirm-warning">Se muestra una sola vez: al cerrar ya no se puede volver a ver.</p>
    </FormModal>
  );
}
