"use client";

import { useEffect, useId, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { CirclePlus, Download, FileDown, FileSpreadsheet, Filter, FilterX, Upload } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { DropdownMenu } from "@/components/DropdownMenu";
import { FiltersModal } from "@/components/FiltersModal";
import { Modal } from "@/components/Modal";
import { PageHeader } from "@/components/PageHeader";
import { Badge } from "@/components/StatusBadge";
import { showToast } from "@/components/Toast";
import { useDebounce } from "@/hooks/useDebounce";
import { listCompanies } from "@/services/core";
import { listWaves } from "@/services/orders";
import {
  disableProduct, downloadProductsTemplate, enableProduct, exportProducts, getCatalogProduct, importProducts, listCatalog,
  type CatalogProduct, type ProductFilters, type ProductType,
} from "@/services/products";
import { ImportDialog } from "@/features/clients";
import { useAuth } from "@/state/auth";
import { dateLabel } from "@/features/orders/components/OrderDetail";
import { useOrderResource } from "@/features/orders/hooks/useOrderResources";
import { usePagedList } from "@/features/clients/hooks/usePagedList";
import { waveLabel, waveOptions, type WaveOption } from "@/features/clients/waves";
import { TYPE_LABELS } from "../labels";
import { ProductDetail } from "./ProductDetail";
import { ProductFormModal } from "./ProductFormModal";

const PAGE_SIZES = [20, 50, 100];
type FilterValues = { companyId: string; type: "" | ProductType; status: "" | "active" | "inactive"; wave: string };
const EMPTY: FilterValues = { companyId: "", type: "", status: "", wave: "" };

// Productos (Tarea 09, H-04: SCRUM-105). Sin mockup: mismo patrón que Clientes › Todos los perfiles
// (barra, tabla, panel «Detalle de producto» y FormModal). Búsqueda, filtros, página y producto
// abierto (?producto=) viven en la URL. Consultan Administrador, Jefe de Operaciones y el Cliente
// (solo su empresa, lo filtra el servidor); crear, editar, activar y desactivar: solo el Administrador.
// TODO(matriz): la escritura del Cliente está «Por validar» en la matriz v1.2; se confirma con Ismael.
export function ProductsView() {
  const { session } = useAuth();
  const params = useSearchParams(), router = useRouter(), pathname = usePathname();
  const paramsKey = params.toString();
  const page = Math.max(0, Number(params.get("page")) || 0);
  const limit = PAGE_SIZES.includes(Number(params.get("limit"))) ? Number(params.get("limit")) : 20;
  const selected = Number(params.get("producto")) || null;
  const urlQuery = params.get("q") ?? "";
  const [query, setQuery] = useState(urlQuery);
  const debouncedQuery = useDebounce(query);
  const lastQuery = useRef(urlQuery);
  const [checks, setChecks] = useState<Set<number>>(new Set());
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [editing, setEditing] = useState<CatalogProduct | "new" | null>(null);
  const [toggling, setToggling] = useState<CatalogProduct | null>(null);
  const [importing, setImporting] = useState(false);
  const [mobile, setMobile] = useState(false);
  const [revision, setRevision] = useState(0);
  const role = session?.user.role;
  const canWrite = role === "admin";
  const isClient = role === "cliente";

  const filters = useMemo<FilterValues>(() => {
    const p = new URLSearchParams(paramsKey);
    const type = p.get("tipo"), status = p.get("estado");
    return {
      companyId: p.get("cliente") ?? "",
      type: type === "simple" || type === "kit" ? type : "",
      status: status === "active" || status === "inactive" ? status : "",
      wave: p.get("ola") ?? "",
    };
  }, [paramsKey]);
  const apiFilters = useMemo<ProductFilters>(() => ({
    ...(urlQuery ? { q: urlQuery } : {}),
    ...(filters.companyId ? { company_id: Number(filters.companyId) } : {}),
    ...(filters.type ? { type: filters.type } : {}),
    ...(filters.status ? { status: filters.status } : {}),
    ...(filters.wave ? { wave: filters.wave } : {}),
  }), [filters, urlQuery]);
  const filtersKey = JSON.stringify(apiFilters);
  const listKey = JSON.stringify({ ...apiFilters, limit, offset: page * limit, revision });
  const { rows, total, loading, error, reload } = usePagedList(() => listCatalog(JSON.parse(listKey)), listKey);
  // El Cliente no elige cliente (el servidor ya lo acota a su empresa).
  const companiesRes = useOrderResource(() => (isClient ? Promise.resolve({ data: [] }) : listCompanies()), `products-companies:${isClient}`);
  const companies = (companiesRes.data?.data ?? []).map((c) => ({ id: c.id, name: c.name, active: c.active }));
  const wavesRes = useOrderResource(() => (canWrite || role === "jefe_operaciones" ? listWaves() : Promise.resolve({ data: [] })), `products-waves:${role}`);
  const waveList = wavesRes.data?.data ?? [];

  const inPage = rows.find((r) => r.id === selected);
  // El detalle siempre se pide: trae la composición del kit y quién lo modificó.
  const detailRes = useOrderResource<CatalogProduct | null>(
    () => (selected ? getCatalogProduct(selected).catch(() => null) : Promise.resolve(null)),
    `${selected}:${revision}`, `${selected}`,
  );
  const product = detailRes.data ?? (inPage && inPage.type !== "kit" ? inPage : null);

  const updateUrl = (change: (next: URLSearchParams) => void, replace = false) => {
    const next = new URLSearchParams(params.toString());
    change(next);
    const href = `${pathname}?${next.toString()}`;
    if (replace) router.replace(href); else router.push(href);
  };
  const select = (id: number | null) => updateUrl((next) => { if (id) next.set("producto", String(id)); else next.delete("producto"); }, true);

  useEffect(() => { const m = matchMedia("(max-width:1279px)"); const update = () => setMobile(m.matches); update(); m.addEventListener("change", update); return () => m.removeEventListener("change", update); }, []);
  useEffect(() => { setChecks(new Set()); }, [page, limit, filtersKey]);
  useEffect(() => { if (urlQuery !== lastQuery.current) { lastQuery.current = urlQuery; setQuery(urlQuery); } }, [urlQuery]);
  useEffect(() => {
    if (debouncedQuery === lastQuery.current) return;
    lastQuery.current = debouncedQuery;
    updateUrl((next) => { if (debouncedQuery) next.set("q", debouncedQuery); else next.delete("q"); next.delete("page"); }, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedQuery]);

  const applyFilters = (v: FilterValues) => {
    setFiltersOpen(false);
    updateUrl((next) => {
      const map: [string, string][] = [["cliente", v.companyId], ["tipo", v.type], ["estado", v.status], ["ola", v.wave]];
      for (const [key, value] of map) { if (value) next.set(key, value); else next.delete(key); }
      next.delete("page");
    });
  };
  const clearAll = () => { lastQuery.current = ""; setQuery(""); router.push(pathname); };
  const toggle = (id: number) => setChecks((prev) => { const next = new Set(prev); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  const refresh = () => { reload(); setRevision((v) => v + 1); };
  const confirmToggle = async (reason: string) => {
    if (!toggling) return;
    const updated = toggling.active ? await disableProduct(toggling.id, reason) : await enableProduct(toggling.id, reason);
    setToggling(null);
    showToast(updated.active ? `Producto ${updated.sku} activado.` : `Producto ${updated.sku} desactivado.`);
    refresh();
  };

  const activeFilters = [filters.companyId, filters.type, filters.status, filters.wave].filter(Boolean).length;
  const run = async (task: () => Promise<void>, ok: string) => {
    try {
      await task();
      showToast(ok);
    } catch (e) {
      showToast((e as Error).message, "error");
    }
  };
  // Excel (Fase 2): plantilla e importación solo para quien escribe; exportar, para todos.
  const excelItems = [
    ...(canWrite ? [
      { label: "Descargar plantilla", icon: <FileDown size={18} aria-hidden="true" />, onSelect: () => run(downloadProductsTemplate, "Plantilla descargada.") },
      { label: "Importar productos", icon: <Upload size={18} aria-hidden="true" />, onSelect: () => setImporting(true) },
    ] : []),
    { label: `Exportar seleccionados${checks.size ? ` (${checks.size})` : ""}`, icon: <Download size={18} aria-hidden="true" />, disabled: checks.size === 0, onSelect: () => run(() => exportProducts({ ids: [...checks] }), "Exportación descargada.") },
    { label: "Exportar (filtros actuales)", icon: <Download size={18} aria-hidden="true" />, disabled: total === 0, onSelect: () => run(() => exportProducts(apiFilters), "Exportación descargada.") },
  ];
  const formWaves = (current?: string | null): WaveOption[] => waveOptions(waveList, current);
  const detail = product
    ? <ProductDetail product={product} canWrite={canWrite} inModal={mobile} onEdit={() => setEditing(product)} onToggle={() => setToggling(product)} />
    : selected && detailRes.loading ? <div className="detail-skeleton" aria-label="Cargando producto" />
    : selected ? <p className="detail-empty">Este producto no existe.</p>
    : <p className="detail-empty">Selecciona un producto para ver su detalle</p>;

  return (
    <div className="orders-view clients-view">
      <PageHeader title="Productos" />
      <div className="orders-toolbar clients-toolbar">
        <label className="orders-search">
          <span className="sr-only">Buscar SKU o descripción</span>
          <input placeholder="Buscar SKU o descripción" value={query} onChange={(e) => setQuery(e.target.value)} />
        </label>
        <button className="btn ghost" onClick={() => setFiltersOpen(true)}><Filter size={18} />Filtros{activeFilters ? ` (${activeFilters})` : ""}</button>
        <button className="btn ghost" onClick={clearAll}><FilterX size={18} />Limpiar filtros</button>
        <DropdownMenu className="excel-actions" triggerClassName="btn excel" trigger={<><FileSpreadsheet size={18} aria-hidden="true" />Excel acciones</>} items={excelItems} />
        {canWrite &&<button className="btn order-primary" onClick={() => setEditing("new")}><CirclePlus size={18} />Nuevo producto</button>}
      </div>
      <div className="orders-workspace">
        <section className="orders-list clients-list" aria-label="Productos">
          {error && rows.length > 0 && <p role="alert">No se pudo actualizar: {error}</p>}
          {error && rows.length === 0 ? <div role="alert">{error}<button className="btn ghost" onClick={reload}>Reintentar</button></div>
            : loading && rows.length === 0 ? <div className="orders-skeleton" aria-label="Cargando productos">{Array.from({ length: 6 }, (_, i) => <div key={i} />)}</div>
            : rows.length === 0 ? <p className="pad">{activeFilters || urlQuery ? "Ningún producto coincide con los filtros" : "No hay productos registrados"}</p>
            : (
              <div className="orders-table-scroll" tabIndex={0} aria-label="Tabla de productos con desplazamiento horizontal">
                <table>
                  <thead>
                    <tr>
                      <th><input aria-label="Seleccionar todos los productos de la página" type="checkbox" checked={rows.every((r) => checks.has(r.id))} onChange={(e) => setChecks(e.target.checked ? new Set(rows.map((r) => r.id)) : new Set())} /></th>
                      {["SKU", "Descripción", "Tipo", "Cliente", "Ola", "Estado", "Fecha de actualización"].map((h) => <th key={h}>{h}</th>)}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((p) => (
                      <tr key={p.id} onClick={() => select(p.id)} className={selected === p.id ? "order-selected" : ""}>
                        <td onClick={(e) => e.stopPropagation()}><input type="checkbox" aria-label={`Seleccionar producto ${p.sku}`} checked={checks.has(p.id)} onChange={() => toggle(p.id)} /></td>
                        <td><button className="order-number" aria-label={`Ver detalle del producto ${p.sku}`} onClick={() => select(p.id)}>{p.sku}</button></td>
                        <td title={p.description}>{p.description}</td>
                        <td><Badge tone={p.type === "kit" ? "processing" : "info"} label={TYPE_LABELS[p.type]} /></td>
                        <td>{p.companyName || "—"}</td>
                        <td>{waveLabel(p.waveCode)}</td>
                        <td><Badge tone={p.active ? "ok" : "neutral"} label={p.active ? "Activo" : "Inactivo"} /></td>
                        <td>{dateLabel(p.updatedAt)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          <footer className="orders-pagination">
            <span>{total} productos · {checks.size} seleccionados</span>
            <label>Por página <select value={limit} onChange={(e) => updateUrl((next) => { next.set("limit", e.target.value); next.set("page", "0"); })}>{PAGE_SIZES.map((n) => <option key={n} value={n}>{n}</option>)}</select></label>
            <button className="btn ghost" disabled={page === 0} onClick={() => updateUrl((next) => { next.set("page", String(page - 1)); })}>Anterior</button>
            <button className="btn ghost" disabled={(page + 1) * limit >= total} onClick={() => updateUrl((next) => { next.set("page", String(page + 1)); })}>Siguiente</button>
          </footer>
        </section>
        {!mobile && <aside className="orders-detail-panel clients-panel">{detail}</aside>}
      </div>
      {mobile && selected !== null && (
        <Modal title={`Detalle de producto ${product?.sku ?? ""}`} onClose={() => select(null)} panelClassName="registry-modal">{detail}</Modal>
      )}
      {filtersOpen && (
        <ProductFiltersModal values={filters} companies={isClient ? undefined : companies} waves={formWaves(filters.wave && filters.wave !== "general" ? filters.wave : null)}
          onApply={applyFilters} onClose={() => setFiltersOpen(false)} />
      )}
      {editing && (
        <ProductFormModal product={editing === "new" ? undefined : editing} companies={companies} waves={formWaves(editing === "new" ? null : editing.waveCode)}
          onClose={() => setEditing(null)}
          onSaved={(saved) => {
            setEditing(null);
            showToast(editing === "new" ? `Producto ${saved.sku} creado.` : `Producto ${saved.sku} actualizado.`);
            if (editing === "new") select(saved.id);
            refresh();
          }} />
      )}
      {importing && (
        <ImportDialog title="Importar productos" fileLabel="Archivo de productos (.xlsx)" missingFile="Elige el archivo .xlsx con los productos."
          hint="Usa la plantilla de productos (RUT cliente, SKU y descripción obligatorios). Solo artículos: los kits se crean con «Nuevo producto». Las filas válidas se crean y las inválidas se informan con su fila y motivo."
          created={(n) => (n === 1 ? "producto creado" : "productos creados")}
          onImport={importProducts} onClose={() => setImporting(false)} onImported={refresh} />
      )}
      {toggling && (
        <ConfirmDialog
          title={toggling.active ? `Desactivar producto ${toggling.sku}` : `Activar producto ${toggling.sku}`}
          reason={toggling.active ? "required" : "optional"} reasonLabel="Motivo" reasonRequiredMessage="Indica el motivo de la desactivación."
          confirmLabel={toggling.active ? "Desactivar" : "Activar"} destructive={toggling.active}
          onConfirm={confirmToggle} onClose={() => setToggling(null)}>
          <p>{toggling.active ? "No se podrá usar en órdenes nuevas ni como componente de un kit. No se borra: su historial se conserva." : "Volverá a estar disponible para órdenes y kits."}</p>
        </ConfirmDialog>
      )}
    </div>
  );
}

// Filtros: cliente (no para el Cliente), tipo, estado y ola.
function ProductFiltersModal({ values, companies, waves, onApply, onClose }: {
  values: FilterValues;
  companies?: { id: number; name: string }[];
  waves: WaveOption[];
  onApply: (v: FilterValues) => void;
  onClose: () => void;
}) {
  const id = useId();
  const [form, setForm] = useState(values);
  const select = (key: keyof FilterValues, label: string, options: [string, string][]) => (
    <div className="modal-field">
      <label htmlFor={`${id}-${key}`}>{label}</label>
      <select id={`${id}-${key}`} value={form[key]} onChange={(e) => setForm({ ...form, [key]: e.target.value })}>
        <option value="">Todos</option>
        {options.map(([value, text]) => <option key={value} value={value}>{text}</option>)}
      </select>
    </div>
  );
  return (
    <FiltersModal title="Filtros" onClose={onClose} footer={<>
      <button type="button" className="btn ghost" onClick={() => setForm(EMPTY)}>Limpiar</button>
      <button type="button" className="btn primary" onClick={() => onApply(form)}>Aplicar filtros</button>
    </>}>
      {companies && select("companyId", "Cliente", companies.map((c) => [String(c.id), c.name]))}
      {select("type", "Tipo", [["simple", TYPE_LABELS.simple], ["kit", TYPE_LABELS.kit]])}
      {select("status", "Estado", [["active", "Activo"], ["inactive", "Inactivo"]])}
      {select("wave", "Ola", waves.map((w) => [w.value, w.label]))}
    </FiltersModal>
  );
}
