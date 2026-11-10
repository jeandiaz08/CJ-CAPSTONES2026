"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Filter, FilterX } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { Modal } from "@/components/Modal";
import { Badge } from "@/components/StatusBadge";
import { showToast } from "@/components/Toast";
import { useDebounce } from "@/hooks/useDebounce";
import { listCompanies } from "@/services/core";
import { listIntegrations, setIntegrationActive, type Integration } from "@/services/integrations";
import { dateLabel } from "@/features/orders/components/OrderDetail";
import { useOrderResource } from "@/features/orders/hooks/useOrderResources";
import { usePagedList } from "@/features/clients/hooks/usePagedList";
import { KIND_LABELS, providerName } from "../providers";
import { actorName, ConnectionDetail } from "./ConnectionDetail";
import { ConnectionFiltersModal, countConnectionFilters, type ConnectionFilterValues } from "./ConnectionFiltersModal";

const FILTER_PARAMS = { company: "empresa", kind: "tipo", provider: "proveedor", status: "estado" } as const;
const loadConnections = () => listIntegrations().then((r) => ({ data: r.data, total: r.data.length }));

// Pestaña «Clientes»: las conexiones de cada empresa. Búsqueda, filtros y conexión abierta
// (?conexion=) viven en la URL. GET /v1/integrations no filtra ni pagina: se filtra aquí.
// TODO(api): columnas «Último evento» y «Errores 24 h» cuando el backend entregue esos datos.
export function ConnectionsTab() {
  const params = useSearchParams(), router = useRouter(), pathname = usePathname();
  const paramsKey = params.toString();
  const selected = Number(params.get("conexion")) || null;
  const urlQuery = params.get("q") ?? "";
  const [query, setQuery] = useState(urlQuery);
  const debouncedQuery = useDebounce(query);
  const lastQuery = useRef(urlQuery);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [toggling, setToggling] = useState<Integration | null>(null);
  const [mobile, setMobile] = useState(false);

  const filters = useMemo<ConnectionFilterValues>(() => {
    const p = new URLSearchParams(paramsKey);
    const kind = p.get("tipo"), status = p.get("estado"), company = p.get("empresa") ?? "";
    return {
      company: /^\d+$/.test(company) ? company : "",
      kind: kind === "channel" || kind === "courier" ? kind : "",
      provider: p.get("proveedor") ?? "",
      status: status === "active" || status === "inactive" ? status : "",
    };
  }, [paramsKey]);

  const { rows, loading, error, reload } = usePagedList(loadConnections, "integrations");
  const companies = useOrderResource(listCompanies, "companies");
  const companyList = useMemo(() => companies.data?.data ?? [], [companies.data]);
  const companyName = (id: number) => companyList.find((c) => c.id === id)?.name ?? `Empresa ${id}`;

  const visible = useMemo(() => {
    const term = urlQuery.trim().toLowerCase();
    const name = (id: number) => companyList.find((c) => c.id === id)?.name ?? "";
    return rows.filter((i) =>
      (!filters.company || i.companyId === Number(filters.company))
      && (!filters.kind || i.kind === filters.kind)
      && (!filters.provider || i.provider === filters.provider)
      && (!filters.status || i.active === (filters.status === "active"))
      && (!term || name(i.companyId).toLowerCase().includes(term) || providerName(i.provider, i.kind).toLowerCase().includes(term) || i.provider.includes(term)));
  }, [rows, filters, urlQuery, companyList]);
  const connection = rows.find((r) => r.id === selected) ?? null;

  const updateUrl = (change: (next: URLSearchParams) => void, replace = false) => {
    const next = new URLSearchParams(params.toString());
    next.set("tab", "clientes");
    change(next);
    const href = `${pathname}?${next.toString()}`;
    if (replace) router.replace(href); else router.push(href);
  };
  const select = (id: number | null) => updateUrl((next) => { if (id) next.set("conexion", String(id)); else next.delete("conexion"); }, true);

  useEffect(() => { const m = matchMedia("(max-width:1279px)"); const update = () => setMobile(m.matches); update(); m.addEventListener("change", update); return () => m.removeEventListener("change", update); }, []);
  useEffect(() => { if (urlQuery !== lastQuery.current) { lastQuery.current = urlQuery; setQuery(urlQuery); } }, [urlQuery]);
  useEffect(() => {
    if (debouncedQuery === lastQuery.current) return;
    lastQuery.current = debouncedQuery;
    updateUrl((next) => { if (debouncedQuery) next.set("q", debouncedQuery); else next.delete("q"); }, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedQuery]);

  const applyFilters = (v: ConnectionFilterValues) => {
    setFiltersOpen(false);
    updateUrl((next) => {
      for (const key of Object.keys(FILTER_PARAMS) as (keyof typeof FILTER_PARAMS)[]) { if (v[key]) next.set(FILTER_PARAMS[key], v[key]); else next.delete(FILTER_PARAMS[key]); }
    });
  };
  const clearAll = () => { lastQuery.current = ""; setQuery(""); router.push(`${pathname}?tab=clientes`); };
  const confirmToggle = async () => {
    if (!toggling) return;
    const active = !toggling.active;
    await setIntegrationActive(toggling.id, active);
    setToggling(null);
    showToast(active ? "Conexión activada." : "Conexión desactivada. Ya no recibe eventos.");
    reload();
  };

  const activeFilters = countConnectionFilters(filters);
  const title = (i: Integration) => `${companyName(i.companyId)} · ${providerName(i.provider, i.kind)}`;
  const detail = connection
    ? <ConnectionDetail connection={connection} companyName={companyName(connection.companyId)} inModal={mobile} onToggle={() => setToggling(connection)} />
    : selected && loading ? <div className="detail-skeleton" aria-label="Cargando conexión" />
    : selected ? <p className="detail-empty">Esta conexión no existe.</p>
    : <p className="detail-empty">Selecciona una conexión para ver su detalle</p>;

  return (
    <>
      <div className="orders-toolbar clients-toolbar">
        <label className="orders-search">
          <span className="sr-only">Buscar empresa o proveedor</span>
          <input placeholder="Buscar empresa o proveedor" value={query} onChange={(e) => setQuery(e.target.value)} />
        </label>
        <button className="btn ghost" onClick={() => setFiltersOpen(true)}><Filter size={18} />Filtros{activeFilters ? ` (${activeFilters})` : ""}</button>
        <button className="btn ghost" onClick={clearAll}><FilterX size={18} />Limpiar filtros</button>
      </div>
      <div className="orders-workspace">
        <section className="orders-list clients-list connections-list" aria-label="Conexiones">
          {error && rows.length > 0 && <p role="alert">No se pudo actualizar: {error}</p>}
          {error && rows.length === 0 ? <div role="alert">{error}<button className="btn ghost" onClick={reload}>Reintentar</button></div>
            : loading && rows.length === 0 ? <div className="orders-skeleton" aria-label="Cargando conexiones">{Array.from({ length: 6 }, (_, i) => <div key={i} />)}</div>
            : visible.length === 0 ? <p className="pad">{activeFilters || urlQuery ? "Ninguna conexión coincide con los filtros" : "No hay conexiones registradas"}</p>
            : (
              <div className="orders-table-scroll" tabIndex={0} aria-label="Tabla de conexiones con desplazamiento horizontal">
                <table>
                  <thead><tr>{["Empresa", "Tipo", "Proveedor", "Estado", "Creada"].map((h) => <th key={h}>{h}</th>)}</tr></thead>
                  <tbody>
                    {visible.map((i) => (
                      <tr key={i.id} onClick={() => select(i.id)} className={selected === i.id ? "order-selected" : ""}>
                        <td><button className="order-number" aria-label={`Ver detalle de la conexión ${title(i)}`} onClick={() => select(i.id)}>{companyName(i.companyId)}</button></td>
                        <td>{KIND_LABELS[i.kind] ?? i.kind}</td>
                        <td>{providerName(i.provider, i.kind)}</td>
                        <td><Badge tone={i.active ? "ok" : "neutral"} label={i.active ? "Activa" : "Inactiva"} /></td>
                        <td>{dateLabel(i.createdAt)} · {actorName(i.createdBy)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          <footer className="orders-pagination"><span>{visible.length} {visible.length === 1 ? "conexión" : "conexiones"}</span></footer>
        </section>
        {!mobile && <aside className="orders-detail-panel clients-panel">{detail}</aside>}
      </div>
      {mobile && selected !== null && (
        <Modal title={connection ? `Detalle de conexión ${title(connection)}` : "Detalle de conexión"} onClose={() => select(null)} panelClassName="registry-modal">{detail}</Modal>
      )}
      {filtersOpen && <ConnectionFiltersModal values={filters} companies={companyList} onApply={applyFilters} onClose={() => setFiltersOpen(false)} />}
      {toggling && (
        <ConfirmDialog
          title={`${toggling.active ? "Desactivar" : "Activar"} conexión ${title(toggling)}`}
          confirmLabel={toggling.active ? "Desactivar" : "Activar"}
          destructive={toggling.active}
          onConfirm={confirmToggle}
          onClose={() => setToggling(null)}>
          <p>{toggling.active
            ? "La conexión dejará de recibir eventos: los avisos que lleguen a su webhook se rechazarán. El secreto se conserva."
            : "La conexión volverá a recibir eventos con el mismo secreto."}</p>
        </ConfirmDialog>
      )}
    </>
  );
}
