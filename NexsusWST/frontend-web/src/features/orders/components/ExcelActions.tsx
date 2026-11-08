"use client";

import { useId, useState } from "react";
import { Download, FileDown, FileSpreadsheet, ListFilter, Upload } from "lucide-react";
import { DropdownMenu, type DropdownItem } from "@/components/DropdownMenu";
import { FileDrop } from "@/components/FileDrop";
import { FormModal } from "@/components/FormModal";
import { showToast } from "@/components/Toast";
import { apiErrorMessage, saveBlob } from "@/services/api";
import type { CatalogItem, Company } from "@/services/core";
import { downloadImportTemplate, exportOrders, importOrders, type ImportResult } from "@/services/orders";
import type { FilterValues } from "../filters";

const MAX_IMPORT_BYTES = 5 << 20;
type Option = { value: string; label: string };
type Exporter = (params: URLSearchParams, fallbackName: string) => Promise<void>;

function ExportDialog({ filters, companies, couriers, statuses, exporter, fileName, onClose }: {
  filters: FilterValues;
  companies: Option[] | null;
  couriers: Option[];
  statuses: Option[];
  exporter: Exporter;
  fileName: string;
  onClose: () => void;
}) {
  const id = useId();
  const [form, setForm] = useState({ from: filters.from, to: filters.to, companyId: filters.companyId, courier: filters.courier, format: "xlsx" });
  const [selected, setSelected] = useState(filters.statuses);
  const [dateError, setDateError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const set = (changes: Partial<typeof form>) => setForm((current) => ({ ...current, ...changes }));
  const submit = async () => {
    setError(null);
    if (!form.from || !form.to) return setDateError("Indica el rango de fechas.");
    if (form.from > form.to) return setDateError("«Desde» no puede ser posterior a «Hasta».");
    setDateError(null);
    const params = new URLSearchParams({ from: form.from, to: form.to, format: form.format });
    if (form.companyId) params.set("company_id", form.companyId);
    if (form.courier) params.set("courier", form.courier);
    if (selected.length) params.set("status", selected.join(","));
    setBusy(true);
    try {
      await exporter(params, `${fileName}.${form.format}`);
      showToast("Exportación descargada.");
      onClose();
    } catch (e) {
      setError(apiErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <FormModal title="Exportar por filtros" onClose={onClose} onSubmit={submit} submitLabel="Exportar" busy={busy} error={error}>
      <div className="filter-fields">
        <p className="form-notice">Se exportan hasta 10.000 filas. Si el filtro entrega más, acota las fechas o el estado.</p>
        <div className="filter-dates">
          <div className="modal-field">
            <label htmlFor={`${id}-from`}>Desde *</label>
            <input id={`${id}-from`} type="date" value={form.from} aria-invalid={!!dateError} onChange={(e) => { setDateError(null); set({ from: e.target.value }); }} />
          </div>
          <div className="modal-field">
            <label htmlFor={`${id}-to`}>Hasta *</label>
            <input id={`${id}-to`} type="date" value={form.to} aria-invalid={!!dateError} onChange={(e) => { setDateError(null); set({ to: e.target.value }); }} />
          </div>
        </div>
        {dateError && <p className="modal-field-error" role="alert">{dateError}</p>}
        {companies && (
          <div className="modal-field">
            <label htmlFor={`${id}-company`}>Cliente</label>
            <select id={`${id}-company`} value={form.companyId} onChange={(e) => set({ companyId: e.target.value })}>
              <option value="">Todos</option>
              {companies.map((company) => <option key={company.value} value={company.value}>{company.label}</option>)}
            </select>
          </div>
        )}
        <div className="modal-field">
          <label htmlFor={`${id}-courier`}>Courier</label>
          <select id={`${id}-courier`} value={form.courier} onChange={(e) => set({ courier: e.target.value })}>
            <option value="">Todos</option>
            {couriers.map((courier) => <option key={courier.value} value={courier.value}>{courier.label}</option>)}
          </select>
        </div>
        <fieldset className="filter-statuses">
          <legend>Estado</legend>
          {statuses.map((status) => (
            <label key={status.value}>
              <input type="checkbox" checked={selected.includes(status.value)} onChange={() => setSelected((current) => current.includes(status.value) ? current.filter((value) => value !== status.value) : [...current, status.value])} />
              {status.label}
            </label>
          ))}
        </fieldset>
        <fieldset className="filter-statuses">
          <legend>Formato</legend>
          {["xlsx", "csv"].map((format) => (
            <label key={format}>
              <input type="radio" name={`${id}-format`} checked={form.format === format} onChange={() => set({ format })} />
              {format}
            </label>
          ))}
        </fieldset>
      </div>
    </FormModal>
  );
}

function errorsCsv(result: ImportResult) {
  const cell = (value: string | number) => `"${String(value).replaceAll('"', '""')}"`;
  const lines = [["Fila", "Referencia", "Mensaje"], ...result.errors.map((e) => [e.row, e.ref, e.message])];
  return new Blob(["﻿" + lines.map((line) => line.map(cell).join(";")).join("\r\n")], { type: "text/csv;charset=utf-8" });
}

function ImportDialog({ companies, defaultCompanyId, onClose, onImported }: {
  companies: Company[];
  defaultCompanyId: number | null;
  onClose: () => void;
  onImported: () => void;
}) {
  const id = useId();
  const active = companies.filter((company) => company.active !== false);
  const [companyId, setCompanyId] = useState<number>(defaultCompanyId ?? (active.length === 1 ? active[0].id : 0));
  const [file, setFile] = useState<File | null>(null);
  const [result, setResult] = useState<ImportResult | null>(null);
  const [companyError, setCompanyError] = useState<string | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const submit = async () => {
    setError(null);
    setCompanyError(companyId ? null : "Elige el cliente.");
    setFileError(file ? null : "Elige el archivo.");
    if (!companyId || !file) return;
    setBusy(true);
    try {
      const res = await importOrders(companyId, file);
      setResult(res);
      showToast(`${res.ordersCreated} ${res.ordersCreated === 1 ? "orden creada" : "órdenes creadas"}${res.errors.length ? `, ${res.failedRows} ${res.failedRows === 1 ? "fila" : "filas"} con error` : ""}.`, res.errors.length ? "warning" : "success");
      onImported();
    } catch (e) {
      setError(apiErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  const template = async () => {
    try {
      await downloadImportTemplate(companyId || null);
    } catch (e) {
      setError(apiErrorMessage(e));
    }
  };
  if (result) {
    return (
      <FormModal wide title="Resultado de la importación" onClose={onClose}>
        <div className="bulk-summary">
          <p><strong>{result.ordersCreated}</strong> {result.ordersCreated === 1 ? "orden creada" : "órdenes creadas"} · <strong>{result.failedRows}</strong> {result.failedRows === 1 ? "fila" : "filas"} con error</p>
          {result.errors.length > 0 && (
            <>
              <div className="import-errors">
                <table>
                  <thead><tr><th>Fila</th><th>Referencia</th><th>Mensaje</th></tr></thead>
                  <tbody>{result.errors.map((e, index) => <tr key={`${e.row}-${index}`}><td>{e.row}</td><td>{e.ref || "—"}</td><td>{e.message}</td></tr>)}</tbody>
                </table>
              </div>
              <button type="button" className="text-button order-form-add" onClick={() => saveBlob(errorsCsv(result), "errores-importacion.csv")}><Download size={16} aria-hidden="true" /> Descargar errores</button>
            </>
          )}
        </div>
      </FormModal>
    );
  }
  return (
    <FormModal title="Importar órdenes" onClose={onClose} onSubmit={submit} submitLabel="Importar" busy={busy} error={error}>
      <div className="filter-fields">
        <div className="modal-field">
          <label htmlFor={`${id}-company`}>Cliente *</label>
          {active.length === 1 ? (
            <input id={`${id}-company`} value={active[0].name} disabled />
          ) : (
            <select id={`${id}-company`} value={companyId || ""} aria-invalid={!!companyError} onChange={(e) => { setCompanyError(null); setCompanyId(Number(e.target.value)); }}>
              <option value="">Elige un cliente</option>
              {active.map((company) => <option key={company.id} value={company.id}>{company.name}</option>)}
            </select>
          )}
          {companyError && <span className="modal-field-error" role="alert">{companyError}</span>}
        </div>
        <div>
          <button type="button" className="text-button order-form-add" onClick={template}><Download size={16} aria-hidden="true" /> Descargar plantilla</button>
          <p className="form-hint">{companyId ? "Incluye listas de couriers, comunas y los productos activos de este cliente." : "Elige un cliente para incluir sus productos activos en la plantilla."}</p>
        </div>
        <FileDrop label="Archivo *" accept={[".xlsx", ".csv"]} maxBytes={MAX_IMPORT_BYTES} value={file} onChange={(next) => { setFileError(null); setFile(next); }} />
        {fileError && <span className="modal-field-error" role="alert">{fileError}</span>}
      </div>
    </FormModal>
  );
}

// «Excel acciones»: exportar lo seleccionado o lo filtrado e importar órdenes. El Registro histórico
// lo usa con su propio exportador, sin importar y con «Exportar masivo» (todo lo filtrado en pantalla).
export function ExcelActions({ selectedIds, filters, companies, companyOptions, couriers, statuses, canImport, defaultCompanyId, onImported,
  exporter = exportOrders, fileName = "ordenes", bulkExport, bulkDisabled = false }: {
  selectedIds: number[];
  filters: FilterValues;
  companies: Company[];
  companyOptions: Option[] | null;
  couriers: CatalogItem[];
  statuses: Option[];
  canImport: boolean;
  defaultCompanyId: number | null;
  onImported: () => void;
  exporter?: Exporter;
  fileName?: string;
  bulkExport?: () => Promise<void>;
  bulkDisabled?: boolean;
}) {
  const [dialog, setDialog] = useState<"export" | "import" | null>(null);
  const exportSelected = async () => {
    try {
      await exporter(new URLSearchParams({ ids: selectedIds.join(","), format: "xlsx" }), `${fileName}.xlsx`);
      showToast("Exportación descargada.");
    } catch (e) {
      showToast(apiErrorMessage(e), "error");
    }
  };
  const items: DropdownItem[] = [
    { label: `Exportar seleccionadas${selectedIds.length ? ` (${selectedIds.length})` : ""}`, icon: <Download size={18} aria-hidden="true" />, disabled: selectedIds.length === 0, onSelect: exportSelected },
    { label: "Exportar por filtros", icon: <ListFilter size={18} aria-hidden="true" />, onSelect: () => setDialog("export") },
    ...(bulkExport ? [{ label: "Exportar masivo", icon: <FileDown size={18} aria-hidden="true" />, disabled: bulkDisabled, onSelect: bulkExport }] : []),
    ...(canImport ? [{ label: "Importar órdenes (Excel)", icon: <Upload size={18} aria-hidden="true" />, onSelect: () => setDialog("import") }] : []),
  ];
  return (
    <>
      <DropdownMenu className="excel-actions" triggerClassName="btn excel" trigger={<><FileSpreadsheet size={18} aria-hidden="true" />Excel acciones</>} items={items} />
      {dialog === "export" && <ExportDialog filters={filters} companies={companyOptions} couriers={couriers.map((c) => ({ value: c.code, label: c.name }))} statuses={statuses} exporter={exporter} fileName={fileName} onClose={() => setDialog(null)} />}
      {dialog === "import" && <ImportDialog companies={companies} defaultCompanyId={defaultCompanyId} onClose={() => setDialog(null)} onImported={onImported} />}
    </>
  );
}
