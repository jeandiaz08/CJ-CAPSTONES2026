"use client";
import { ArrowBigRight, Star, StarOff } from "lucide-react";
import { CompactTable } from "@/components/CompactTable";
import { TrackingSection } from "./TrackingSection";
import {originLabel,channelLabel,dispatchLabel,warehouseLabel} from "../format";
import {useState} from "react";
import { getOrder,downloadOrderDocument,type Order } from "@/services/orders";
import { productsForCompany } from "@/services/orderReference";
import type { Role } from "@/state/auth";
import { useOrderResource } from "../hooks/useOrderResources";
import { StatusBadge } from "@/components/StatusBadge";
import { CancelOrderDialog, PriorityDialog } from "./OrderActionDialogs";
import { OrderFormModal } from "./OrderFormModal";
import { PriorityBadge } from "./PriorityMark";
import { ResolveIncidentDialog, ResolveVoidedDialog } from "./ResolveDialogs";
import { cancelBlockReason, canManageOrders, canOperate } from "../rules";
import type { CatalogItem, Company } from "@/services/core";
export function todoAction() { }
export const dateLabel = (s: string) => new Intl.DateTimeFormat("es-CL", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit", hourCycle: "h23", timeZone: "America/Santiago" }).format(new Date(s)).replace(",", "").replaceAll("-", "/") + " hrs";
export function OrderDetail({ id, companyId, role, revision, labels, inModal = false, companyName, courierName, assignment = false, onChanged, companies = [], couriers = [] }: {
    onChanged?: (order?: Order) => void;
    companies?: Company[];
    couriers?: CatalogItem[];
    assignment?: boolean;
    inModal?: boolean; companyName?:string; courierName?:string;
    id: number;
    companyId: number | null;
    role: Role;
    revision: number;
    labels: Record<string, string>;
}) {
    const [downloadError,setDownloadError]=useState<string|null>(null);
    const [dialog,setDialog]=useState<"cancel"|"priority"|"edit"|"resolve"|null>(null);
    const resource = useOrderResource(() => getOrder(id, companyId), `${id}:${companyId}:${revision}`, `${id}:${companyId}`);
    const order = resource.data?.order;
    const products = useOrderResource(() => order ? productsForCompany(order.companyId) : Promise.resolve([]), `products:${order?.companyId}`);
    if (resource.loading)
        return <div className="detail-skeleton" aria-label="Cargando detalle"/>;
    if (resource.error && !order)
        return <p role="alert">No se pudo cargar el detalle: {resource.error}</p>;
    if (!order || !resource.data)
        return null;
    const { events, packages, documents, tracking } = resource.data;
    const last = events.filter(e => e.type === "status_change" || e.type === "created").at(-1);
    const cancelReason = cancelBlockReason(order, role);
    const canEdit = !!order.editable && canManageOrders(role);
    const resolvable = order.status === "incident" || order.status === "voided";
    const resolveButton = <button className="btn order-primary" aria-disabled={!resolvable || !canOperate(role)} disabled={!resolvable || !canOperate(role)} title={resolvable && !canOperate(role) ? "Solo Jefe de Operaciones o Administrador." : undefined} onClick={()=>setDialog("resolve")}>Resolver incidencia</button>;
    const dialogProps = { order, companyName, courierName, onClose: () => setDialog(null), onConflict: () => onChanged?.(), onDone: (updated: Order) => { setDialog(null); onChanged?.(updated); } };
    const fmt=(value?:string)=>value?dateLabel(value):"—";
    const fields:[string,unknown][]=[["Origen",originLabel(order)],["Canal",channelLabel(order.channel)],["Cliente",companyName],["Courier",courierName||order.courier],["N° seguimiento",order.courierCode],["Tipo de despacho",dispatchLabel(order.dispatchType)],["Bodega",warehouseLabel(order.warehouse)],["Destinatario",order.recipientName],["Teléfono",order.recipientPhone],["Correo",order.recipientEmail],["Dirección",order.addressLine],["Comuna",order.commune],["Región",order.regionCode],["Notas del cliente",order.customerNotes],["Ola",order.waveId],["Referencia WMS",order.wmsRef],["Prioridad",order.priority?"Sí":"No"],["Tomada por",order.claimedByName],["Tipo de incidencia",order.incidentLabel],["Motivo de incidencia",order.note],["Creada",fmt(order.createdAt)],["Actualizada",fmt(order.updatedAt)],["Entregada",fmt(order.handedOverAt)],["Cancelada",fmt(order.cancelledAt)]];
    const previous=events.filter(e=>e.type==="status_change"||e.type==="created").at(-2);
    const documentLinks=documents.map(d=><a key={d.id} href={`/bff/orders/v1/orders/${order.id}/documents/${d.id}/download`} className="document-link" onClick={event=>{event.preventDefault();setDownloadError(null);downloadOrderDocument(order.id,d).catch(e=>setDownloadError(e.message))}}>{d.filename}</a>);
    return <div className="order-detail">
    {!inModal&&<h2>Detalle de orden #{order.externalRef}</h2>}
    <div className="order-detail-content"><div className="detail-status-row"><StatusBadge status={order.status} label={order.statusLabel}/>{order.priority&&<PriorityBadge/>}</div>
    <section>
    <h3>Información adicional</h3>
    <CompactTable label="Información adicional" columns={fields.map(([label])=>({label}))} rows={[{key:"order",cells:fields.map(([,value])=>value===undefined||value===null||value===""?"—":String(value))}]}/>
    </section>
    <section>
    <header>
    <h3>Historial de estados</h3>
    <button onClick={todoAction} className="text-button">Ver todo</button>
    </header>{last ? <>
        <div className="state-history"><div><strong>{labels[last.fromStatus]||"Creación"}</strong><small>{previous?.actor||"—"}</small><small>{previous?dateLabel(previous.occurredAt):"—"}</small></div><ArrowBigRight aria-label="Cambió a"/><div><strong>{labels[last.toStatus]||order.statusLabel}</strong><small>{last.actor}</small><small>{dateLabel(last.occurredAt)}</small></div></div>
        </> : <p>Sin cambios registrados</p>}</section>
    <section>
    <h3>Líneas de orden</h3>{products.error && <p role="status">No se pudieron cargar los nombres de productos.</p>}<div className="detail-scroll">
    <table>
    <thead>
    <tr>
    <th>SKU</th>
    <th>Producto</th>
    <th>Cantidad</th>
    </tr>
    </thead>
    <tbody>{order.items?.map(i => <tr key={i.sku}>
        <td>{i.sku}</td>
        <td>{products.data?.find(p => p.sku === i.sku)?.description ?? "—"}</td>
        <td>{i.quantity}</td>
        </tr>)}</tbody>
    </table>
    </div>
    </section>
    <section>
    <h3>Bultos y documentos</h3>{packages.length ? <div className="detail-scroll">
        <table>
        <thead>
        <tr>
        <th>Bulto</th>
        <th>Etiqueta</th>
        <th>Manifiesto</th>
        </tr>
        </thead>
        <tbody>{packages.map(p => <tr key={p.id}>
            <td>{p.code}</td>
            <td>{documents.length?documentLinks:"—"}</td>
            <td>{p.manifestId ? `#${p.manifestId}` : "Sin manifiesto"}</td>
            </tr>)}</tbody>
        </table>
        </div> : <><p>Sin bultos registrados</p>{documentLinks}</>}{downloadError&&<p role="alert">{downloadError}</p>}</section>
    <TrackingSection tracking={tracking}/>
    </div><div className="detail-actions">
    {assignment?<button className={order.priority?"btn priority-remove":"btn order-primary"} onClick={()=>setDialog("priority")}>{order.priority?<><StarOff size={18}/>Quitar prioridad</>:<><Star size={18}/>Enviar primero a la cola</>}</button>:resolveButton}
    {assignment&&order.status==="incident"&&resolveButton}
    <div>
    <button className="btn ghost" aria-disabled={!canEdit} disabled={!canEdit} title={order.editable ? undefined : "Ya no se puede editar: fue enviada a preparación."} onClick={()=>setDialog("edit")}>Editar</button>
    <span title={cancelReason} tabIndex={cancelReason ? 0 : undefined}>
    <button className="btn danger" aria-disabled={!!cancelReason} disabled={!!cancelReason} onClick={()=>setDialog("cancel")}>Cancelar</button>
    </span>
    </div>
    </div>
    {dialog==="cancel"&&<CancelOrderDialog {...dialogProps}/>}
    {dialog==="priority"&&<PriorityDialog {...dialogProps}/>}
    {dialog==="resolve"&&order.status==="voided"&&<ResolveVoidedDialog {...dialogProps} events={events} products={products.data??[]}/>}
    {dialog==="resolve"&&order.status==="incident"&&<ResolveIncidentDialog {...dialogProps} events={events} couriers={couriers} labels={labels}/>}
    {dialog==="edit"&&<OrderFormModal order={order} companies={companies} couriers={couriers} defaultCompanyId={order.companyId} onClose={()=>setDialog(null)} onConflict={()=>onChanged?.()} onSaved={updated=>{setDialog(null);onChanged?.(updated);}}/>}
    </div>;
}
