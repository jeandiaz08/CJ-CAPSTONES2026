"use client";

import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Check, FileSpreadsheet, Filter, FilterX } from "lucide-react";
import { showToast } from "@/components/Toast";
import { useDebounce } from "@/hooks/useDebounce";
import { exportActivity, type ActivityFilters, type ActivitySection } from "@/services/activity";
import { COLUMNS, SECTIONS, orderLink } from "../activityColumns";
import { useActivity } from "../hooks/useActivity";
import { ActivityFiltersDrawer, type ActivityFilterValues } from "./ActivityFiltersDrawer";

const PAGE_SIZES = [20, 50, 100];
const day = (d: Date) => new Intl.DateTimeFormat("en-CA", { timeZone: "America/Santiago" }).format(d); // AAAA-MM-DD

// Por defecto, los últimos 7 días (hoy incluido).
export function defaultRange() {
  const today = new Date();
  return { from: day(new Date(today.getTime() - 6 * 86_400_000)), to: day(today) };
}

// Pestaña «Registro de acciones» (Tarea 06, Fase 3): un apartado a la vez; apartado, búsqueda,
// filtros y página viven en la URL. Al hacer clic en una fila se despliega el antes y el después.
export function ActionsRegistry({ onLive }: { onLive: (live: boolean) => void }) {
  const params = useSearchParams(), router = useRouter(), pathname = usePathname();
  const paramsKey = params.toString();
  const section = (SECTIONS.some((s) => s.value === params.get("section")) ? params.get("section") : "orders") as ActivitySection;
  const page = Math.max(0, Number(params.get("page")) || 0);
  const limit = PAGE_SIZES.includes(Number(params.get("limit"))) ? Number(params.get("limit")) : 50;
  const urlQuery = params.get("q") ?? "";
  const [query, setQuery] = useState(urlQuery);
  const debouncedQuery = useDebounce(query);
  const lastQuery = useRef(urlQuery);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [defaults] = useState(() => ({ ...defaultRange(), actor: "", role: "", kind: "" }));

  const values = useMemo<ActivityFilterValues>(() => {
    const p = new URLSearchParams(paramsKey);
    return { from: p.get("from") ?? defaults.from, to: p.get("to") ?? defaults.to, actor: p.get("actor") ?? "", role: p.get("role") ?? "", kind: p.get("kind") ?? "" };
  }, [paramsKey, defaults]);
  const filters = useMemo<ActivityFilters>(() => ({
    module: section,
    ...(urlQuery ? { q: urlQuery } : {}),
    ...Object.fromEntries(Object.entries(values).filter(([, v]) => v)),
  }), [section, urlQuery, values]);
  const { items, total, loading, error, reload, live } = useActivity({ ...filters, limit, offset: page * limit });
  const columns = COLUMNS[section];
  const label = SECTIONS.find((s) => s.value === section)!.label;
  const activeFilters = [values.from !== defaults.from || values.to !== defaults.to, values.actor, values.role, values.kind].filter(Boolean).length;

  const updateUrl = (change: (next: URLSearchParams) => void, replace = false) => {
    const next = new URLSearchParams(params.toString());
    change(next);
    next.set("tab", "acciones");
    const href = `${pathname}?${next}`;
    if (replace) router.replace(href); else router.push(href);
  };
  useEffect(() => onLive(live), [live, onLive]);
  useEffect(() => setExpanded(null), [paramsKey]);
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

  const chooseSection = (value: ActivitySection) => updateUrl((next) => { next.set("section", value); next.delete("page"); });
  const applyFilters = (v: ActivityFilterValues) => {
    setFiltersOpen(false);
    updateUrl((next) => {
      for (const key of ["from", "to", "actor", "role", "kind"] as const) {
        if (v[key] && v[key] !== defaults[key]) next.set(key, v[key]); else next.delete(key);
      }
      next.delete("page");
    });
  };
  const clearAll = () => { lastQuery.current = ""; setQuery(""); router.push(`${pathname}?tab=acciones&section=${section}`); };
  const exportExcel = async () => {
    setExporting(true);
    try {
      await exportActivity(filters);
      showToast("Exportación descargada.");
    } catch (e) {
      showToast((e as Error).message, "error");
    } finally {
      setExporting(false);
    }
  };
  const width = columns.reduce((sum, c) => sum + c.width, 0);

  return (
    <>
      <div className="packing-chips activity-chips" role="group" aria-label="Apartado del registro de acciones">
        {SECTIONS.map((s) => (
          <button key={s.value} type="button" className={`packing-chip ${section === s.value ? "is-active" : ""}`} aria-pressed={section === s.value} onClick={() => chooseSection(s.value)}>
            {section === s.value && <Check size={14} aria-hidden="true" />}{s.label}
          </button>
        ))}
      </div>
      <h2 className="activity-title">{label} · {total} {total === 1 ? "acción" : "acciones"}</h2>
      <div className="orders-toolbar">
        <label className="orders-search">
          <span className="sr-only">Buscar</span>
          <input placeholder={section === "users" ? "Buscar usuario" : section === "products" ? "Buscar SKU" : section === "dispatch" ? "Buscar n° de orden o #manifiesto" : "Buscar n° de orden"} value={query} onChange={(e) => setQuery(e.target.value)} />
        </label>
        <button className="btn ghost" onClick={() => setFiltersOpen(true)}><Filter size={18} />Filtros{activeFilters ? ` (${activeFilters})` : ""}</button>
        <button className="btn ghost" onClick={clearAll}><FilterX size={18} />Limpiar filtros</button>
        <button className="btn excel activity-export" disabled={exporting || total === 0} onClick={exportExcel}>
          <FileSpreadsheet size={18} aria-hidden="true" />{exporting ? "Exportando…" : "Exportar registro"}
        </button>
      </div>
      <section className="activity-list" aria-label={`Registro de acciones · ${label}`}>
        {error && items.length > 0 && <p role="alert">No se pudo actualizar: {error}</p>}
        {error && items.length === 0 ? <div role="alert">{error}<button className="btn ghost" onClick={reload}>Reintentar</button></div>
          : loading && items.length === 0 ? <div className="orders-skeleton" aria-label="Cargando acciones">{Array.from({ length: 6 }, (_, i) => <div key={i} />)}</div>
          : items.length === 0 ? <p className="pad">Sin acciones de «{label}» para los filtros elegidos. Prueba con otro rango de fechas.</p>
          : (
            <div className="orders-table-scroll" tabIndex={0} aria-label="Tabla de acciones con desplazamiento horizontal">
              <table style={{ width, minWidth: width }}>
                <colgroup>{columns.map((c) => <col key={c.label} style={{ width: c.width }} />)}</colgroup>
                <thead><tr>{columns.map((c) => <th key={c.label}>{c.label}</th>)}</tr></thead>
                <tbody>
                  {items.map((it) => {
                    const open = expanded === it.id;
                    return (
                      <Fragment key={it.id}>
                        <tr className={open ? "is-expanded" : ""} aria-expanded={open} onClick={() => setExpanded(open ? null : it.id)}>
                          {columns.map((c) => <td key={c.label} title={c.text(it)}>{c.cell ? c.cell(it) : c.text(it)}</td>)}
                        </tr>
                        {open && (
                          <tr className="activity-detail-row">
                            <td colSpan={columns.length}>
                              <dl>
                                <div><dt>Acción</dt><dd>{it.actionLabel}{it.entity.type === "order" && section !== "orders" && section !== "packing" ? <> · Orden {orderLink(it)}</> : null}</dd></div>
                                {it.statusChange && <div><dt>Estado</dt><dd>{it.statusChange}</dd></div>}
                                {it.reason && <div><dt>Motivo u observación</dt><dd>{it.reason}</dd></div>}
                                {it.detailLines.map((line) => {
                                  const [name, ...rest] = line.split(": ");
                                  return <div key={line}><dt>{name}</dt><dd>{rest.join(": ") || "—"}</dd></div>;
                                })}
                              </dl>
                            </td>
                          </tr>
                        )}
                      </Fragment>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        <footer className="orders-pagination">
          <span>{total} {total === 1 ? "acción" : "acciones"}</span>
          <label>Por página <select value={limit} onChange={(e) => updateUrl((next) => { next.set("limit", e.target.value); next.set("page", "0"); })}>{PAGE_SIZES.map((n) => <option key={n} value={n}>{n}</option>)}</select></label>
          <button className="btn ghost" disabled={page === 0} onClick={() => updateUrl((next) => { next.set("limit", String(limit)); next.set("page", String(page - 1)); })}>Anterior</button>
          <button className="btn ghost" disabled={(page + 1) * limit >= total} onClick={() => updateUrl((next) => { next.set("limit", String(limit)); next.set("page", String(page + 1)); })}>Siguiente</button>
        </footer>
      </section>
      {filtersOpen && <ActivityFiltersDrawer values={values} defaults={defaults} onApply={applyFilters} onClose={() => setFiltersOpen(false)} />}
    </>
  );
}
