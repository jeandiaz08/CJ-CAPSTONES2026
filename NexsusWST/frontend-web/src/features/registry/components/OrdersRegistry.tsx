"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { FileSpreadsheet, Filter, FilterX } from "lucide-react";
import { Modal } from "@/components/Modal";
import { StatusBadge } from "@/components/StatusBadge";
import { showToast } from "@/components/Toast";
import { useDebounce } from "@/hooks/useDebounce";
import { getOrderMeta } from "@/services/orders";
import { listCompanies, getCatalog } from "@/services/core";
import { exportRegistry, listRegistry, type RegistryEntry, type RegistryFilters } from "@/services/registry";
import { useAuth } from "@/state/auth";
import { OrderFiltersDrawer } from "@/features/orders/components/OrderFiltersDrawer";
import { dateLabel } from "@/features/orders/components/OrderDetail";
import { countFilters, readFilters, writeFilters, type FilterValues } from "@/features/orders/filters";
import { useOrderResource } from "@/features/orders/hooks/useOrderResources";
import { useRegistry } from "../hooks/useRegistry";
import { RegistryDetail, closedByLabel } from "./RegistryDetail";

const PAGE_SIZES = [20, 50, 100];

// Pestaña «Registro órdenes»: Canceladas y órdenes con más de 1 año sin movimiento (Tarea 06).
// Búsqueda, filtros, página y orden abierta (?order=) viven en la URL.
export function OrdersRegistry({ onLive }: { onLive: (live: boolean) => void }) {
  const { session, companyId } = useAuth();
  const params = useSearchParams(), router = useRouter(), pathname = usePathname();
  const paramsKey = params.toString();
  const page = Math.max(0, Number(params.get("page")) || 0);
  const limit = PAGE_SIZES.includes(Number(params.get("limit"))) ? Number(params.get("limit")) : 20;
  const selected = Number(params.get("order")) || null;
  const urlQuery = params.get("q") ?? "";
  const [query, setQuery] = useState(urlQuery);
  const debouncedQuery = useDebounce(query);
  const lastQuery = useRef(urlQuery);
  const [checks, setChecks] = useState<Set<number>>(new Set());
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [mobile, setMobile] = useState(false);
  const [revision, setRevision] = useState(0);

  // Solo valen Cliente, Courier, Estado y fechas (del cierre).
  const filters = useMemo<FilterValues>(() => ({ ...readFilters(new URLSearchParams(paramsKey), false), origin: "", onlyActive: false }), [paramsKey]);
  const registryFilters = useMemo<RegistryFilters>(() => ({
    ...(urlQuery ? { q: urlQuery } : {}),
    ...(filters.statuses.length ? { status: filters.statuses.join(",") } : {}),
    ...(filters.courier ? { courier: filters.courier } : {}),
    ...(filters.from ? { from: filters.from } : {}),
    ...(filters.to ? { to: filters.to } : {}),
  }), [filters, urlQuery]);
  const filtersKey = JSON.stringify(registryFilters);
  const scopeCompany = filters.companyId ? Number(filters.companyId) : companyId;
  const { rows, total, loading, error, reload, live } = useRegistry({ ...registryFilters, limit, offset: page * limit }, scopeCompany);
  const refs = useOrderResource(() => Promise.all([listCompanies(), getCatalog("couriers"), getOrderMeta()]), "registry-references");
  const labels = Object.fromEntries(refs.data?.[2].statuses.map((s) => [s.code, s.label]) ?? []);

  // La orden abierta puede no estar en esta página (por ejemplo, al llegar desde un enlace).
  const inPage = rows.find((r) => r.id === selected);
  const outside = useOrderResource<RegistryEntry | null>(
    () => selected && !inPage ? listRegistry({ ids: [selected], limit: 1 }, null).then((r) => r.data[0] ?? null) : Promise.resolve(null),
    `${selected}:${!!inPage}:${revision}`, `${selected}`,
  );
  const entry = inPage ?? outside.data ?? null;

  const updateUrl = (change: (next: URLSearchParams) => void, replace = false) => {
    const next = new URLSearchParams(params.toString());
    change(next);
    const search = next.toString();
    const href = search ? `${pathname}?${search}` : pathname;
    if (replace) router.replace(href); else router.push(href);
  };
  const select = (id: number | null) => updateUrl((next) => { if (id) next.set("order", String(id)); else next.delete("order"); }, true);

  useEffect(() => onLive(live), [live, onLive]);
  useEffect(() => { const m = matchMedia("(max-width:1279px)"); const update = () => setMobile(m.matches); update(); m.addEventListener("change", update); return () => m.removeEventListener("change", update); }, []);
  useEffect(() => { setChecks(new Set()); }, [scopeCompany, page, limit, filtersKey]);
  useEffect(() => { if (!loading) setRevision((v) => v + 1); }, [rows, loading]);
  // El buscador escribe en la URL con debounce; si la URL cambia por fuera (atrás, Limpiar), manda la URL.
  useEffect(() => {
    if (urlQuery !== lastQuery.current) { lastQuery.current = urlQuery; setQuery(urlQuery); }
  }, [urlQuery]);
  useEffect(() => {
    if (debouncedQuery === lastQuery.current) return;
    lastQuery.current = debouncedQuery;
    updateUrl((next) => { if (debouncedQuery) next.set("q", debouncedQuery); else next.delete("q"); next.delete("page"); }, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedQuery]);

  const applyFilters = (values: FilterValues) => { setFiltersOpen(false); updateUrl((next) => { writeFilters(next, values); next.delete("page"); }); };
  const clearAll = () => { lastQuery.current = ""; setQuery(""); router.push(pathname); };
  const toggle = (id: number) => setChecks((previous) => { const next = new Set(previous); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  const exportExcel = async () => {
    setExporting(true);
    try {
      await exportRegistry({ ...registryFilters, ...(checks.size ? { ids: [...checks] } : {}) }, scopeCompany);
    } catch (e) {
      showToast((e as Error).message, "error");
    } finally {
      setExporting(false);
    }
  };

  const multiCompany = session?.user.allCompanies || (session?.user.companies.length ?? 0) > 1;
  const companies = refs.data?.[0].data ?? [], couriers = refs.data?.[1].data ?? [];
  const statusOptions = (refs.data?.[2].statuses ?? []).map((s) => ({ value: s.code as string, label: s.label }));
  const activeFilters = countFilters(filters);
  const detail = entry
    ? <RegistryDetail entry={entry} labels={labels} revision={revision} inModal={mobile} />
    : selected && outside.loading ? <div className="detail-skeleton" aria-label="Cargando registro" />
    : selected ? <p className="detail-empty">Esta orden no está en el registro histórico.</p>
    : <p className="detail-empty">Selecciona una orden para ver su registro</p>;

  return (
    <>
      <div className="orders-toolbar">
        <label className="orders-search">
          <span className="sr-only">Buscar n° de orden</span>
          <input placeholder="Buscar n° de orden" value={query} onChange={(e) => setQuery(e.target.value)} />
        </label>
        <button className="btn ghost" onClick={() => setFiltersOpen(true)}><Filter size={18} />Filtros{activeFilters ? ` (${activeFilters})` : ""}</button>
        <button className="btn ghost" onClick={clearAll}><FilterX size={18} />Limpiar filtros</button>
        <button className="btn excel registry-export" disabled={exporting || total === 0} onClick={exportExcel}
          title={checks.size ? `Exporta las ${checks.size} seleccionadas` : "Exporta lo filtrado"}>
          <FileSpreadsheet size={18} aria-hidden="true" />{exporting ? "Exportando…" : checks.size ? `Exportar seleccionadas (${checks.size})` : "Exportar registro"}
        </button>
      </div>
      <div className="orders-workspace">
        <section className="orders-list registry-list" aria-label="Registro de órdenes">
          {error && rows.length > 0 && <p role="alert">No se pudo actualizar: {error}</p>}
          {error && rows.length === 0 ? <div role="alert">{error}<button className="btn ghost" onClick={reload}>Reintentar</button></div>
            : loading && rows.length === 0 ? <div className="orders-skeleton" aria-label="Cargando registro">{Array.from({ length: 6 }, (_, i) => <div key={i} />)}</div>
            : rows.length === 0 ? <p className="pad">{activeFilters || urlQuery ? "Ninguna orden coincide con los filtros" : "No hay órdenes en el registro histórico"}</p>
            : (
              <div className="orders-table-scroll" tabIndex={0} aria-label="Tabla del registro con desplazamiento horizontal">
                <table>
                  <thead>
                    <tr>
                      <th><input aria-label="Seleccionar todas las órdenes de la página" type="checkbox" checked={rows.every((r) => checks.has(r.id))} onChange={(e) => setChecks(e.target.checked ? new Set(rows.map((r) => r.id)) : new Set())} /></th>
                      {["N° orden", "Cliente", "Cerrada por", "Motivo", "Estado", "Fecha"].map((h) => <th key={h}>{h}</th>)}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((r) => (
                      <tr key={r.id} onClick={() => select(r.id)} className={selected === r.id ? "order-selected" : ""}>
                        <td onClick={(e) => e.stopPropagation()}><input type="checkbox" aria-label={`Seleccionar orden ${r.externalRef}`} checked={checks.has(r.id)} onChange={() => toggle(r.id)} /></td>
                        <td><button className="order-number" aria-label={`Ver registro de la orden ${r.externalRef}`} onClick={() => select(r.id)}>{r.externalRef}</button></td>
                        <td title={r.companyName}>{r.companyName || `Empresa ${r.companyId}`}</td>
                        <td title={closedByLabel(r)}>{r.closedBy.name}</td>
                        <td title={r.motive}>{r.motive || "—"}</td>
                        <td><StatusBadge status={r.status} label={r.statusLabel} /></td>
                        <td>{dateLabel(r.closedAt)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          <footer className="orders-pagination">
            <span>{total} órdenes · {checks.size} seleccionadas</span>
            <label>Por página <select value={limit} onChange={(e) => updateUrl((next) => { next.set("limit", e.target.value); next.set("page", "0"); })}>{PAGE_SIZES.map((n) => <option key={n} value={n}>{n}</option>)}</select></label>
            <button className="btn ghost" disabled={page === 0} onClick={() => updateUrl((next) => { next.set("limit", String(limit)); next.set("page", String(page - 1)); })}>Anterior</button>
            <button className="btn ghost" disabled={(page + 1) * limit >= total} onClick={() => updateUrl((next) => { next.set("limit", String(limit)); next.set("page", String(page + 1)); })}>Siguiente</button>
          </footer>
        </section>
        {!mobile && <aside className="orders-detail-panel registry-panel">{detail}</aside>}
      </div>
      {mobile && selected !== null && (
        <Modal title={`Registro orden #${entry?.externalRef ?? ""}`} onClose={() => select(null)} panelClassName="registry-modal">{detail}</Modal>
      )}
      {filtersOpen && (
        <OrderFiltersDrawer assignment={false} registry values={filters} companies={multiCompany ? companies.map((c) => ({ value: String(c.id), label: c.name })) : null}
          couriers={couriers.map((c) => ({ value: c.code, label: c.name }))} statuses={statusOptions} onApply={applyFilters} onClose={() => setFiltersOpen(false)} />
      )}
    </>
  );
}
