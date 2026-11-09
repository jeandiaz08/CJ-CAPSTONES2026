"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { CirclePlus, Download, FileSpreadsheet, Filter, FilterX } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { DropdownMenu } from "@/components/DropdownMenu";
import { Modal } from "@/components/Modal";
import { Badge } from "@/components/StatusBadge";
import { showToast } from "@/components/Toast";
import { useDebounce } from "@/hooks/useDebounce";
import { disableCompany, enableCompany, exportCompanies, getAdminCompany, listAdminCompanies, type AdminCompany, type CompanyFilters } from "@/services/clients";
import { listWaves } from "@/services/orders";
import { useAuth } from "@/state/auth";
import { dateLabel } from "@/features/orders/components/OrderDetail";
import { useOrderResource } from "@/features/orders/hooks/useOrderResources";
import { canManageProfiles } from "../access";
import { usePagedList } from "../hooks/usePagedList";
import { waveLabel, waveOptions } from "../waves";
import { CompanyDetail } from "./CompanyDetail";
import { CompanyFiltersModal, countCompanyFilters, type CompanyFilterValues } from "./CompanyFiltersModal";
import { CompanyFormModal } from "./CompanyFormModal";

const PAGE_SIZES = [20, 50, 100];

// Administración clientes › Empresas (Tarea 07, Fase 2). Búsqueda, filtros, página y empresa abierta
// (?empresa=) viven en la URL. Sin «Gestionar perfiles» se ve en lectura: sin Nueva empresa, Editar,
// Activar ni Desactivar (el servidor responde 403 igual).
export function CompaniesTab() {
  const { session } = useAuth();
  const params = useSearchParams(), router = useRouter(), pathname = usePathname();
  const paramsKey = params.toString();
  const page = Math.max(0, Number(params.get("page")) || 0);
  const limit = PAGE_SIZES.includes(Number(params.get("limit"))) ? Number(params.get("limit")) : 20;
  const selected = Number(params.get("empresa")) || null;
  const urlQuery = params.get("q") ?? "";
  const [query, setQuery] = useState(urlQuery);
  const debouncedQuery = useDebounce(query);
  const lastQuery = useRef(urlQuery);
  const [checks, setChecks] = useState<Set<number>>(new Set());
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [editing, setEditing] = useState<AdminCompany | "new" | null>(null);
  const [toggling, setToggling] = useState<AdminCompany | null>(null);
  const [mobile, setMobile] = useState(false);

  const canManage = canManageProfiles(session?.user, "client");
  const showWms = session?.user.role === "admin";

  const filters = useMemo<CompanyFilterValues>(() => {
    const p = new URLSearchParams(paramsKey);
    const status = p.get("status");
    return { status: status === "active" || status === "inactive" ? status : "", wave: p.get("wave") ?? "", from: p.get("from") ?? "", to: p.get("to") ?? "" };
  }, [paramsKey]);
  const apiFilters = useMemo<CompanyFilters>(() => ({
    ...(urlQuery ? { q: urlQuery } : {}),
    ...(filters.status ? { status: filters.status } : {}),
    ...(filters.wave ? { wave: filters.wave } : {}),
    ...(filters.from ? { from: filters.from } : {}),
    ...(filters.to ? { to: filters.to } : {}),
  }), [filters, urlQuery]);
  const filtersKey = JSON.stringify(apiFilters);
  const listKey = JSON.stringify({ ...apiFilters, limit, offset: page * limit });
  const { rows, total, loading, error, reload } = usePagedList(() => listAdminCompanies(JSON.parse(listKey)), listKey);
  const waves = useOrderResource(listWaves, "waves");
  const waveList = waves.data?.data ?? [];
  const waveChoices = waveOptions(waveList, filters.wave && filters.wave !== "general" ? filters.wave : null);

  // La empresa abierta puede no estar en esta página (por ejemplo, al llegar desde un enlace).
  const [revision, setRevision] = useState(0);
  const inPage = rows.find((r) => r.id === selected);
  const outside = useOrderResource<AdminCompany | null>(
    () => (selected && !inPage ? getAdminCompany(selected).catch(() => null) : Promise.resolve(null)),
    `${selected}:${!!inPage}:${revision}`, `${selected}`,
  );
  const company = inPage ?? outside.data ?? null;

  const updateUrl = (change: (next: URLSearchParams) => void, replace = false) => {
    const next = new URLSearchParams(params.toString());
    change(next);
    const href = `${pathname}?${next.toString()}`;
    if (replace) router.replace(href); else router.push(href);
  };
  const select = (id: number | null) => updateUrl((next) => { if (id) next.set("empresa", String(id)); else next.delete("empresa"); }, true);

  useEffect(() => { const m = matchMedia("(max-width:1279px)"); const update = () => setMobile(m.matches); update(); m.addEventListener("change", update); return () => m.removeEventListener("change", update); }, []);
  useEffect(() => { setChecks(new Set()); }, [page, limit, filtersKey]);
  useEffect(() => { if (urlQuery !== lastQuery.current) { lastQuery.current = urlQuery; setQuery(urlQuery); } }, [urlQuery]);
  useEffect(() => {
    if (debouncedQuery === lastQuery.current) return;
    lastQuery.current = debouncedQuery;
    updateUrl((next) => { if (debouncedQuery) next.set("q", debouncedQuery); else next.delete("q"); next.delete("page"); }, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedQuery]);

  const applyFilters = (v: CompanyFilterValues) => {
    setFiltersOpen(false);
    updateUrl((next) => {
      for (const key of ["status", "wave", "from", "to"] as const) { if (v[key]) next.set(key, v[key]); else next.delete(key); }
      next.delete("page");
    });
  };
  const clearAll = () => { lastQuery.current = ""; setQuery(""); router.push(`${pathname}?tab=empresas`); };
  const toggle = (id: number) => setChecks((prev) => { const next = new Set(prev); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  const refresh = () => { reload(); setRevision((v) => v + 1); };
  const download = async (f: Omit<CompanyFilters, "limit" | "offset">) => {
    try {
      await exportCompanies(f);
      showToast("Exportación descargada.");
    } catch (e) {
      showToast((e as Error).message, "error");
    }
  };
  const confirmToggle = async (reason: string) => {
    if (!toggling) return;
    const updated = toggling.active ? await disableCompany(toggling.id, reason) : await enableCompany(toggling.id, reason);
    setToggling(null);
    showToast(updated.active ? `Empresa ${updated.name} activada.` : `Empresa ${updated.name} desactivada. Sus usuarios ya no pueden operar.`);
    refresh();
  };

  const activeFilters = countCompanyFilters(filters);
  const detail = company
    ? <CompanyDetail company={company} canManage={canManage} showWms={showWms} inModal={mobile} onEdit={() => setEditing(company)} onToggle={() => setToggling(company)} />
    : selected && outside.loading ? <div className="detail-skeleton" aria-label="Cargando empresa" />
    : selected ? <p className="detail-empty">Esta empresa no existe.</p>
    : <p className="detail-empty">Selecciona una empresa para ver su detalle</p>;

  return (
    <>
      <div className="orders-toolbar clients-toolbar">
        <label className="orders-search">
          <span className="sr-only">Buscar empresa o RUT</span>
          <input placeholder="Buscar empresa o RUT" value={query} onChange={(e) => setQuery(e.target.value)} />
        </label>
        <button className="btn ghost" onClick={() => setFiltersOpen(true)}><Filter size={18} />Filtros{activeFilters ? ` (${activeFilters})` : ""}</button>
        <button className="btn ghost" onClick={clearAll}><FilterX size={18} />Limpiar filtros</button>
        <DropdownMenu className="excel-actions" triggerClassName="btn excel" trigger={<><FileSpreadsheet size={18} aria-hidden="true" />Excel acciones</>} items={[
          { label: `Exportar seleccionadas${checks.size ? ` (${checks.size})` : ""}`, icon: <Download size={18} aria-hidden="true" />, disabled: checks.size === 0, onSelect: () => download({ ids: [...checks] }) },
          { label: "Exportar empresas (filtros actuales)", icon: <Download size={18} aria-hidden="true" />, disabled: total === 0, onSelect: () => download(apiFilters) },
        ]} />
        {canManage && <button className="btn order-primary" onClick={() => setEditing("new")}><CirclePlus size={18} />Nueva empresa</button>}
      </div>
      <div className="orders-workspace">
        <section className="orders-list clients-list" aria-label="Empresas">
          {error && rows.length > 0 && <p role="alert">No se pudo actualizar: {error}</p>}
          {error && rows.length === 0 ? <div role="alert">{error}<button className="btn ghost" onClick={reload}>Reintentar</button></div>
            : loading && rows.length === 0 ? <div className="orders-skeleton" aria-label="Cargando empresas">{Array.from({ length: 6 }, (_, i) => <div key={i} />)}</div>
            : rows.length === 0 ? <p className="pad">{activeFilters || urlQuery ? "Ninguna empresa coincide con los filtros" : "No hay empresas registradas"}</p>
            : (
              <div className="orders-table-scroll" tabIndex={0} aria-label="Tabla de empresas con desplazamiento horizontal">
                <table>
                  <thead>
                    <tr>
                      <th><input aria-label="Seleccionar todas las empresas de la página" type="checkbox" checked={rows.every((r) => checks.has(r.id))} onChange={(e) => setChecks(e.target.checked ? new Set(rows.map((r) => r.id)) : new Set())} /></th>
                      {["Nombre empresa", "RUT", "Correo", "Teléfono", "Ola", "N° perfiles", "Estado", "Fecha de creación"].map((h) => <th key={h}>{h}</th>)}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((c) => (
                      <tr key={c.id} onClick={() => select(c.id)} className={selected === c.id ? "order-selected" : ""}>
                        <td onClick={(e) => e.stopPropagation()}><input type="checkbox" aria-label={`Seleccionar empresa ${c.name}`} checked={checks.has(c.id)} onChange={() => toggle(c.id)} /></td>
                        <td><button className="order-number" aria-label={`Ver detalle de la empresa ${c.name}`} onClick={() => select(c.id)}>{c.name}</button></td>
                        <td>{c.taxId}</td>
                        <td title={c.email}>{c.email || "—"}</td>
                        <td>{c.phone || "—"}</td>
                        <td>{waveLabel(c.waveCode)}</td>
                        <td>{c.profilesCount}</td>
                        <td><Badge tone={c.active ? "ok" : "neutral"} label={c.active ? "Activa" : "Inactiva"} /></td>
                        <td>{dateLabel(c.createdAt)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          <footer className="orders-pagination">
            <span>{total} empresas · {checks.size} seleccionadas</span>
            <label>Por página <select value={limit} onChange={(e) => updateUrl((next) => { next.set("limit", e.target.value); next.set("page", "0"); })}>{PAGE_SIZES.map((n) => <option key={n} value={n}>{n}</option>)}</select></label>
            <button className="btn ghost" disabled={page === 0} onClick={() => updateUrl((next) => { next.set("page", String(page - 1)); })}>Anterior</button>
            <button className="btn ghost" disabled={(page + 1) * limit >= total} onClick={() => updateUrl((next) => { next.set("page", String(page + 1)); })}>Siguiente</button>
          </footer>
        </section>
        {!mobile && <aside className="orders-detail-panel clients-panel">{detail}</aside>}
      </div>
      {mobile && selected !== null && (
        <Modal title={`Detalle de empresa ${company?.name ?? ""}`} onClose={() => select(null)} panelClassName="registry-modal">{detail}</Modal>
      )}
      {filtersOpen && <CompanyFiltersModal values={filters} waves={waveChoices} onApply={applyFilters} onClose={() => setFiltersOpen(false)} />}
      {editing && (
        <CompanyFormModal company={editing === "new" ? undefined : editing} waves={waveList} wavesError={waves.error} showWms={showWms}
          onClose={() => setEditing(null)}
          onSaved={(saved) => {
            setEditing(null);
            showToast(editing === "new" ? `Empresa ${saved.name} creada.` : `Empresa ${saved.name} actualizada.`);
            if (editing === "new") select(saved.id);
            refresh();
          }} />
      )}
      {toggling && (
        <ConfirmDialog
          title={toggling.active ? `Desactivar empresa ${toggling.name}` : `Activar empresa ${toggling.name}`}
          reason={toggling.active ? "required" : "optional"}
          reasonLabel="Motivo"
          reasonRequiredMessage="Indica el motivo de la desactivación."
          confirmLabel={toggling.active ? "Desactivar" : "Activar"}
          destructive={toggling.active}
          onConfirm={confirmToggle}
          onClose={() => setToggling(null)}>
          <p>{toggling.active
            ? "Sus usuarios dejarán de operar en NexusWST desde su siguiente acción. El historial de órdenes y acciones se conserva."
            : "Sus usuarios activos podrán volver a operar."}</p>
        </ConfirmDialog>
      )}
    </>
  );
}
