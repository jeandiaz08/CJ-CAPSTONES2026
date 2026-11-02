"use client";
import {CutoffCards} from "./CutoffCards";
import {AssignmentActions} from "./AssignmentActions";
import {AssignmentSituation} from "./AssignmentSituation";
import {BulkActions} from "./BulkActions";
import {OrderFiltersDrawer} from "./OrderFiltersDrawer";
import {OrderFormModal} from "./OrderFormModal";
import {PrioritySlot} from "./PriorityMark";
import {ExcelActions} from "./ExcelActions";
import {canManageOrders} from "../rules";
import {originLabel} from "../format";
import {countFilters, readFilters, toOrderFilters, writeFilters, type FilterValues} from "../filters";
import { useState, useEffect, useMemo, useRef } from "react";
import { useSearchParams, useRouter, usePathname } from "next/navigation";
import { Package, PackageOpen, Archive, AlertCircle, CirclePlus, Filter, FilterX, Bell } from "lucide-react";
import { PageHeader } from "@/components/PageHeader";
import { Tabs } from "@/components/Tabs";
import { KpiCard } from "@/components/KpiCard";
import { Modal } from "@/components/Modal";
import { StatusBadge, LiveBadge } from "@/components/StatusBadge";
import { useAuth } from "@/state/auth";
import { useDebounce } from "@/hooks/useDebounce";
import { useOrders } from "@/hooks/useOrders";
import { getSummary, getOrderMeta, type Order } from "@/services/orders";
import { listCompanies, getCatalog } from "@/services/core";
import { useOrderResource } from "../hooks/useOrderResources";
import { OrderDetail, dateLabel, todoAction } from "./OrderDetail";
export function OrdersListing({assignment=false}:{assignment?:boolean}) {
    const { session, companyId } = useAuth();
    const params = useSearchParams(), router = useRouter(), pathname = usePathname();
    const [selected, setSelected] = useState<number | null>(null), [checks, setChecks] = useState<Set<number>>(new Set()), [mobile, setMobile] = useState(false), [revision, setRevision] = useState(0), [filtersOpen, setFiltersOpen] = useState(false), [creating, setCreating] = useState(false);
    const page = Math.max(0, Number(params.get("page")) || 0);
    const limit = [20, 50, 100].includes(Number(params.get("limit"))) ? Number(params.get("limit")) : 20;
    const urlQuery = params.get("q") ?? "";
    const [query, setQuery] = useState(urlQuery);
    const debouncedQuery = useDebounce(query);
    const lastQuery = useRef(urlQuery);
    const paramsKey = params.toString();
    const filters = useMemo(() => readFilters(new URLSearchParams(paramsKey), assignment), [paramsKey, assignment]);
    const orderFilters = useMemo(() => toOrderFilters(filters), [filters]);
    const filtersKey = JSON.stringify([orderFilters, urlQuery]);
    // Empresa efectiva: la del filtro Cliente o la de la sesión. Es la única que afecta a los KPI.
    const scopeCompany = filters.companyId ? Number(filters.companyId) : companyId;
    const { orders, total, loading, error, reload, live } = useOrders({ limit, offset: page * limit, ...orderFilters, ...(urlQuery ? { q: urlQuery } : {}), ...(assignment?{view:"asignacion" as const}:{}) });
    const summary = useOrderResource<Awaited<ReturnType<typeof getSummary>>>(() => assignment ? Promise.resolve({data:{}}) : getSummary(scopeCompany), `${scopeCompany}:${revision}`, String(scopeCompany));
    const refs = useOrderResource(() => Promise.all([listCompanies(), getCatalog("couriers"), getOrderMeta()]), "references");
    const labels = Object.fromEntries(refs.data?.[2].statuses.map(s => [s.code, s.label]) ?? []);
    const companyName = (id?: number) => refs.data?.[0].data.find(c => c.id === id)?.name;
    const courierName = (code?: string) => refs.data?.[1].data.find(c => c.code === code)?.name;
    const updateUrl = (change: (next: URLSearchParams) => void, replace = false) => {
        const next = new URLSearchParams(params.toString());
        change(next);
        const search = next.toString();
        const href = search ? `${pathname}?${search}` : pathname;
        if (replace) router.replace(href); else router.push(href);
    };
    useEffect(() => { const m = matchMedia("(max-width:1279px)"); const update = () => setMobile(m.matches); update(); m.addEventListener("change", update); return () => m.removeEventListener("change", update); }, []);
    useEffect(() => { setSelected(null); setChecks(new Set()); }, [scopeCompany, page, filtersKey]);
    useEffect(() => { if (!loading)
        setRevision(v => v + 1); }, [orders, loading]);
    // El buscador escribe en la URL con debounce; si la URL cambia por fuera (atrás, Limpiar), manda la URL.
    useEffect(() => {
        if (urlQuery !== lastQuery.current) { lastQuery.current = urlQuery; setQuery(urlQuery); }
    }, [urlQuery]);
    useEffect(() => {
        if (debouncedQuery === lastQuery.current) return;
        lastQuery.current = debouncedQuery;
        updateUrl(next => { if (debouncedQuery) next.set("q", debouncedQuery); else next.delete("q"); next.delete("page"); }, true);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [debouncedQuery]);
    const applyFilters = (values: FilterValues) => { setFiltersOpen(false); updateUrl(next => { writeFilters(next, values); next.delete("page"); }); };
    const clearAll = () => { lastQuery.current = ""; setQuery(""); router.push(pathname); };
    // Filtro rápido: reemplaza buscador y filtros. Un segundo clic vuelve a la bandeja completa.
    const kpiActive = (status: string) => filters.statuses.length === 1 && filters.statuses[0] === status;
    const toggleKpi = (status: string) => { lastQuery.current = ""; setQuery(""); router.push(kpiActive(status) ? pathname : `${pathname}?status=${status}`); };
    const canAssign = session?.user.role === "admin" || session?.user.role === "jefe_operaciones";
    const toggle = (id: number) => setChecks(previous => { const next = new Set(previous); if (next.has(id))
        next.delete(id);
    else
        next.add(id); return next; });
    const selectedOrder = orders.find(o => o.id === selected);
    const onOrderChanged = (order?: Order) => { if (order?.status === "cancelled") setSelected(null); reload(); };
    const activeFilters = countFilters(filters);
    const multiCompany = session?.user.allCompanies || (session?.user.companies.length ?? 0) > 1;
    const statusOptions = (refs.data?.[2].statuses ?? []).filter(s => assignment ? s.code === "validated" || s.code === "incident" : s.code !== "cancelled").map(s => ({ value: s.code as string, label: s.label }));
    const selectedOrders = orders.filter(o => checks.has(o.id));
    const companies = refs.data?.[0].data ?? [], couriers = refs.data?.[1].data ?? [];
    const companyOptions = multiCompany ? companies.map(c => ({ value: String(c.id), label: c.name })) : null;
    const canCreate = !!session && canManageOrders(session.user.role);
    const detail = selected && session ? <OrderDetail companies={companies} couriers={couriers} assignment={assignment} inModal={mobile} companyName={companyName(selectedOrder?.companyId)} courierName={courierName(selectedOrder?.courier)} id={selected} companyId={scopeCompany} role={session.user.role} revision={revision} labels={labels} onChanged={onOrderChanged}/> : <p className="detail-empty">Selecciona una orden para ver su detalle</p>;
    return <div className={`orders-view ${assignment?"assignment-view":""}`}>
    <PageHeader title="Órdenes" actions={<>
        <LiveBadge live={live}/>
        <button className="icon-btn" aria-label="Notificaciones" onClick={todoAction}>
        <Bell size={23}/>
        </button>
        </>}/>
    <Tabs items={[{ href: "/ordenes/todas/listado", label: "Listado", active: !assignment }, ...(canAssign ? [{ href: "/ordenes/todas/asignacion", label: "Asignación de órdenes", active: assignment }] : [])]}/>
    {assignment?<CutoffCards revision={revision}/>:<>     <div className="orders-kpis">{([{ status: "pending", label: "Pendiente", tone: "neutral", icon: PackageOpen }, { status: "picking", label: "En preparación", tone: "processing", icon: Package }, { status: "packing", label: "En empaque", tone: "warn", icon: PackageOpen }, { status: "ready_pickup", label: "Lista para retiro", tone: "ok", icon: Archive }, { status: "incident", label: "Incidencias", tone: "danger", icon: AlertCircle }] as const).map(k => <KpiCard key={k.status} label={k.label} count={summary.loading ? undefined : summary.data?.data[k.status] ?? 0} tone={k.tone} icon={<k.icon size={30}/>} active={kpiActive(k.status)} onClick={() => toggleKpi(k.status)}/>)}</div>{summary.error && <p role="alert">No se pudieron cargar los indicadores.</p>}</>}<div className="orders-toolbar">
    <label className="orders-search">
    <span className="sr-only">Buscar n° de orden</span>
    <input placeholder="Buscar n° de orden" value={query} onChange={e => setQuery(e.target.value)}/>
    </label>
    <button className="btn ghost" onClick={() => setFiltersOpen(true)}>
    <Filter size={18}/>Filtros{activeFilters ? ` (${activeFilters})` : ""}</button>
    <button className="btn ghost" onClick={clearAll}>
    <FilterX size={18}/>Limpiar filtros</button>
    {!assignment&&<>     <ExcelActions selectedIds={selectedOrders.map(o => o.id)} filters={filters} companies={companies} companyOptions={companyOptions} couriers={couriers} statuses={statusOptions} canImport={canCreate} defaultCompanyId={scopeCompany} onImported={reload}/>
    {canCreate&&<button className="btn order-primary" onClick={() => setCreating(true)}>
    <CirclePlus size={18}/>Nueva orden</button>}
    {session&&<BulkActions selected={selectedOrders} role={session.user.role} onDone={() => { setChecks(new Set()); reload(); }}/>}
</>}    </div>
    {assignment&&<AssignmentActions selected={selectedOrders} filters={{ ...orderFilters, ...(urlQuery ? { q: urlQuery } : {}) }} companyId={scopeCompany} companies={companies} couriers={couriers} onChanged={reload} onAssigned={() => { setChecks(new Set()); reload(); }}/>}
    <div className="orders-workspace">
    <section className="orders-list" aria-label="Listado de órdenes">{refs.error && <p role="alert">No se pudieron cargar los catálogos.</p>}{error && orders.length > 0 && <p role="alert">No se pudo actualizar: {error}</p>}{error && orders.length === 0 ? <div role="alert">{error}<button className="btn ghost" onClick={reload}>Reintentar</button>
        </div> : loading && orders.length === 0 ? <div className="orders-skeleton" aria-label="Cargando órdenes">{Array.from({ length: 6 }, (_, i) => <div key={i}/>)}</div> : orders.length === 0 ? <p className="pad">{activeFilters || urlQuery ? "Ninguna orden coincide con los filtros" : "No hay órdenes"}</p> : <div className="orders-table-scroll" tabIndex={0} aria-label="Tabla de órdenes con desplazamiento horizontal">
        <table>
        <thead>
        <tr>
        <th>
        <input aria-label="Seleccionar todas las órdenes de la página" type="checkbox" checked={orders.length > 0 && orders.every(o => checks.has(o.id))} onChange={e => setChecks(e.target.checked ? new Set(orders.map(o => o.id)) : new Set())}/>
        </th>{["N° orden", "Cliente", assignment?"Situación":"Origen", "Courier", "Bultos", "Estado", "Fecha"].map(h => <th key={h}>{h}</th>)}</tr>
        </thead>
        <tbody>{orders.map(o => <tr key={o.id} onClick={() => setSelected(o.id)} className={selected === o.id ? "order-selected" : ""}>
            <td onClick={e => e.stopPropagation()}>
            <input type="checkbox" aria-label={`Seleccionar orden ${o.id}`} checked={checks.has(o.id)} onChange={() => toggle(o.id)}/>
            </td>
            <td>
            <span className="order-number-cell"><PrioritySlot priority={o.priority}/><button className="order-number" aria-label={`Ver detalle de orden ${o.id}`} onClick={() => setSelected(o.id)}>{o.externalRef}
            </button></span>
            </td>
            <td><span className="client-name" title={companyName(o.companyId)??`Empresa ${o.companyId}`}>{companyName(o.companyId)??`Empresa ${o.companyId}`}</span></td>
            <td>{assignment?<AssignmentSituation value={o.assignmentSituation}/>:originLabel(o)}</td>
            <td>{courierName(o.courier) ?? o.courier}</td>
            <td>{o.packagesCount}</td>
            <td>
            <StatusBadge status={o.status} label={o.statusLabel}/>
            </td>
            <td>{dateLabel(o.createdAt)}</td>
            </tr>)}</tbody>
        </table>
        </div>}<footer className="orders-pagination">
    <span>{total} órdenes · {checks.size} seleccionadas</span>
    <label>Por página <select value={limit} onChange={e => updateUrl(next => { next.set("limit", e.target.value); next.set("page", "0"); })}><option value="20">20</option><option value="50">50</option><option value="100">100</option></select></label>
    <button className="btn ghost" disabled={page === 0} onClick={() => updateUrl(next => { next.set("limit", String(limit)); next.set("page", String(page - 1)); })}>Anterior</button>
    <button className="btn ghost" disabled={(page + 1) * limit >= total} onClick={() => updateUrl(next => { next.set("limit", String(limit)); next.set("page", String(page + 1)); })}>Siguiente</button>
    </footer>
    </section>{!mobile && <aside className="orders-detail-panel">{detail}</aside>}</div>{mobile && selected !== null && <Modal title={`Detalle de orden #${selectedOrder?.externalRef??""}`} onClose={() => setSelected(null)}>{detail}</Modal>}
    {filtersOpen&&<OrderFiltersDrawer assignment={assignment} values={filters} companies={companyOptions} couriers={couriers.map(c => ({ value: c.code, label: c.name }))} statuses={statusOptions} onApply={applyFilters} onClose={() => setFiltersOpen(false)}/>}
    {creating&&<OrderFormModal companies={companies} couriers={couriers} defaultCompanyId={scopeCompany} onClose={() => setCreating(false)} onSaved={order => { setCreating(false); setSelected(order.id); reload(); }}/>}
    </div>;
}
