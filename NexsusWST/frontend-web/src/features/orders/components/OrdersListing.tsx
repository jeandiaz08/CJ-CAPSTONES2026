"use client";
import {CutoffCards} from "./CutoffCards";
import {AssignmentActions} from "./AssignmentActions";
import {AssignmentSituation} from "./AssignmentSituation";
import {originLabel} from "../format";
import { useState, useEffect } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { Package, PackageOpen, Archive, AlertCircle, CirclePlus, Filter, FilterX, FileSpreadsheet, MoreVertical, Bell } from "lucide-react";
import { PageHeader } from "@/components/PageHeader";
import { Tabs } from "@/components/Tabs";
import { KpiCard } from "@/components/KpiCard";
import { Modal } from "@/components/Modal";
import { StatusBadge, LiveBadge } from "@/components/StatusBadge";
import { useAuth } from "@/state/auth";
import { useOrders } from "@/hooks/useOrders";
import { getSummary, getOrderMeta } from "@/services/orders";
import { listCompanies, getCatalog } from "@/services/core";
import { useOrderResource } from "../hooks/useOrderResources";
import { OrderDetail, dateLabel, todoAction } from "./OrderDetail";
export function OrdersListing({assignment=false}:{assignment?:boolean}) {
    const { session, companyId } = useAuth();
    const params = useSearchParams(), router = useRouter();
    const [selected, setSelected] = useState<number | null>(null), [checks, setChecks] = useState<Set<number>>(new Set()), [mobile, setMobile] = useState(false), [revision, setRevision] = useState(0), [query, setQuery] = useState("");
    const page = Math.max(0, Number(params.get("page")) || 0);
    const limit = [20, 50, 100].includes(Number(params.get("limit"))) ? Number(params.get("limit")) : 20;
    const { orders, total, loading, error, reload, live } = useOrders({ limit, offset: page * limit, ...(assignment?{view:"asignacion" as const}:{}) });
    const summary = useOrderResource<Awaited<ReturnType<typeof getSummary>>>(() => assignment ? Promise.resolve({data:{}}) : getSummary(companyId), `${companyId}:${revision}`, String(companyId));
    const refs = useOrderResource(() => Promise.all([listCompanies(), getCatalog("couriers"), getOrderMeta()]), "references");
    const labels = Object.fromEntries(refs.data?.[2].statuses.map(s => [s.code, s.label]) ?? []);
    useEffect(() => { const m = matchMedia("(max-width:1279px)"); const update = () => setMobile(m.matches); update(); m.addEventListener("change", update); return () => m.removeEventListener("change", update); }, []);
    useEffect(() => { setSelected(null); setChecks(new Set()); }, [companyId, page]);
    useEffect(() => { if (!loading)
        setRevision(v => v + 1); }, [orders, loading]);
    const canAssign = session?.user.role === "admin" || session?.user.role === "jefe_operaciones";
    const toggle = (id: number) => setChecks(previous => { const next = new Set(previous); if (next.has(id))
        next.delete(id);
    else
        next.add(id); return next; });
    const detail = selected && session ? <OrderDetail assignment={assignment} inModal={mobile} companyName={refs.data?.[0].data.find(c=>c.id===orders.find(o=>o.id===selected)?.companyId)?.name} courierName={refs.data?.[1].data.find(c=>c.code===orders.find(o=>o.id===selected)?.courier)?.name} id={selected} companyId={companyId} role={session.user.role} revision={revision} labels={labels}/> : <p className="detail-empty">Selecciona una orden para ver su detalle</p>;
    return <div className={`orders-view ${assignment?"assignment-view":""}`}>
    <PageHeader title="Órdenes" actions={<>
        <LiveBadge live={live}/>
        <button className="icon-btn" aria-label="Notificaciones" onClick={todoAction}>
        <Bell size={23}/>
        </button>
        </>}/>
    <Tabs items={[{ href: "/ordenes/todas/listado", label: "Listado", active: !assignment }, ...(canAssign ? [{ href: "/ordenes/todas/asignacion", label: "Asignación de órdenes", active: assignment }] : [])]}/>
    {assignment?<CutoffCards revision={revision}/>:<>     <div className="orders-kpis">{([{ status: "pending", label: "Pendiente", tone: "neutral", icon: PackageOpen }, { status: "picking", label: "En preparación", tone: "processing", icon: Package }, { status: "packing", label: "En empaque", tone: "warn", icon: PackageOpen }, { status: "ready_pickup", label: "Lista para retiro", tone: "ok", icon: Archive }, { status: "incident", label: "Incidencias", tone: "danger", icon: AlertCircle }] as const).map(k => <KpiCard key={k.status} label={k.label} count={summary.loading ? undefined : summary.data?.data[k.status] ?? 0} tone={k.tone} icon={<k.icon size={30}/>}/>)}</div>{summary.error && <p role="alert">No se pudieron cargar los indicadores.</p>}</>}<div className="orders-toolbar">
    <label className="orders-search">
    <span className="sr-only">Buscar n° de orden</span>
    <input placeholder="Buscar n° de orden" value={query} onChange={e => setQuery(e.target.value)}/>
    </label>
    <button className="btn ghost" onClick={todoAction}>
    <Filter size={18}/>Filtros</button>
    <button className="btn ghost" onClick={todoAction}>
    <FilterX size={18}/>Limpiar filtros</button>
    {!assignment&&<>     <button className="btn excel" onClick={todoAction}>
    <FileSpreadsheet size={18}/>Excel acciones</button>
    <button className="btn order-primary" onClick={todoAction}>
    <CirclePlus size={18}/>Nueva orden</button>
    <button className="btn ghost" onClick={todoAction}>
    <MoreVertical size={18}/>Acciones</button>
</>}    </div>
    {assignment&&<AssignmentActions/>}
    <div className="orders-workspace">
    <section className="orders-list" aria-label="Listado de órdenes">{refs.error && <p role="alert">No se pudieron cargar los catálogos.</p>}{error && orders.length === 0 ? <div role="alert">{error}<button className="btn ghost" onClick={reload}>Reintentar</button>
        </div> : loading ? <div className="orders-skeleton" aria-label="Cargando órdenes">{Array.from({ length: 6 }, (_, i) => <div key={i}/>)}</div> : orders.length === 0 ? <p className="pad">No hay órdenes</p> : <div className="orders-table-scroll" tabIndex={0} aria-label="Tabla de órdenes con desplazamiento horizontal">
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
            <button className="order-number" aria-label={`Ver detalle de orden ${o.id}`} onClick={() => setSelected(o.id)}>{o.externalRef}
            </button>
            </td>
            <td><span className="client-name" title={refs.data?.[0].data.find(c=>c.id===o.companyId)?.name??`Empresa ${o.companyId}`}>{refs.data?.[0].data.find(c=>c.id===o.companyId)?.name??`Empresa ${o.companyId}`}</span></td>
            <td>{assignment?<AssignmentSituation value={o.assignmentSituation}/>:originLabel(o)}</td>
            <td>{refs.data?.[1].data.find(c => c.code === o.courier)?.name ?? o.courier}</td>
            <td>{o.packagesCount}</td>
            <td>
            <StatusBadge status={o.status} label={o.statusLabel}/>
            </td>
            <td>{dateLabel(o.createdAt)}</td>
            </tr>)}</tbody>
        </table>
        </div>}<footer className="orders-pagination">
    <span>{total} órdenes · {checks.size} seleccionadas</span>
    <label>Por página <select value={limit} onChange={e => router.push(`?limit=${e.target.value}&page=0`)}><option value="20">20</option><option value="50">50</option><option value="100">100</option></select></label>
    <button className="btn ghost" disabled={page === 0} onClick={() => router.push(`?limit=${limit}&page=${page - 1}`)}>Anterior</button>
    <button className="btn ghost" disabled={(page + 1) * limit >= total} onClick={() => router.push(`?limit=${limit}&page=${page + 1}`)}>Siguiente</button>
    </footer>
    </section>{!mobile && <aside className="orders-detail-panel">{detail}</aside>}</div>{mobile && selected !== null && <Modal title={`Detalle de orden #${orders.find(o=>o.id===selected)?.externalRef??""}`} onClose={() => setSelected(null)}>{detail}</Modal>}</div>;
}
