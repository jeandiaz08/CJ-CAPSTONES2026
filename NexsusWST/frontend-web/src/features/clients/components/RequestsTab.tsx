"use client";

import { useEffect, useId, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { CircleCheck, CircleHelp, CircleX, Filter, FilterX } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { FiltersModal } from "@/components/FiltersModal";
import { KpiCard } from "@/components/KpiCard";
import { Modal } from "@/components/Modal";
import { Badge } from "@/components/StatusBadge";
import { showToast } from "@/components/Toast";
import { useDebounce } from "@/hooks/useDebounce";
import { approveRequest, getRequest, listRequests, rejectRequest, type ProfileRequest, type RequestCounts, type RequestFilters, type RequestKind, type RequestStatus } from "@/services/clients";
import { useAuth } from "@/state/auth";
import { dateLabel } from "@/features/orders/components/OrderDetail";
import { useOrderResource } from "@/features/orders/hooks/useOrderResources";
import { canManageProfiles } from "../access";
import { usePagedList } from "../hooks/usePagedList";
import { ProfileFormModal } from "./ProfileFormModal";
import { RequestDetail, STATUS_LABELS, STATUS_TONES } from "./RequestDetail";

const PAGE_SIZES = [20, 50, 100];
const KINDS: [RequestKind, string][] = [["new_profile", "Nuevo perfil"], ["activation", "Activación"], ["deactivation", "Desactivación"]];
type FilterValues = { kind: "" | RequestKind; status: "" | RequestStatus; from: string; to: string };

// Solicitudes de perfiles (Tarea 07). mode «admin»: Administración clientes › Solicitudes (todas las
// empresas; con «Gestionar perfiles», Aceptar, Editar y aceptar y Rechazar). mode «client»: «Mis
// solicitudes» del Cliente (solo su empresa, lo filtra el servidor; sin acciones). KPIs que filtran,
// filtros en la URL y panel «Detalle de solicitud» (modal en teléfono).
export function RequestsTab({ mode, tab = "solicitudes", refreshKey = 0 }: { mode: "admin" | "client"; tab?: string; refreshKey?: number }) {
  const { session } = useAuth();
  const params = useSearchParams(), router = useRouter(), pathname = usePathname();
  const paramsKey = params.toString();
  const page = Math.max(0, Number(params.get("page")) || 0);
  const limit = PAGE_SIZES.includes(Number(params.get("limit"))) ? Number(params.get("limit")) : 20;
  const selected = Number(params.get("solicitud")) || null;
  const urlQuery = params.get("q") ?? "";
  const [query, setQuery] = useState(urlQuery);
  const debouncedQuery = useDebounce(query);
  const lastQuery = useRef(urlQuery);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [mobile, setMobile] = useState(false);
  const [revision, setRevision] = useState(0);
  const [accepting, setAccepting] = useState<ProfileRequest | null>(null);
  const [editing, setEditing] = useState<ProfileRequest | null>(null);
  const [rejecting, setRejecting] = useState<ProfileRequest | null>(null);
  const canManage = mode === "admin" && canManageProfiles(session?.user, "client");

  const filters = useMemo<FilterValues>(() => {
    const p = new URLSearchParams(paramsKey);
    const kind = p.get("tipo") as RequestKind | null, status = p.get("estado") as RequestStatus | null;
    return {
      kind: kind && KINDS.some(([k]) => k === kind) ? kind : "",
      status: status && status in STATUS_LABELS ? status : "",
      from: p.get("from") ?? "", to: p.get("to") ?? "",
    };
  }, [paramsKey]);
  const apiFilters = useMemo<RequestFilters>(() => ({
    ...(urlQuery ? { q: urlQuery } : {}),
    ...(filters.kind ? { kind: filters.kind } : {}),
    ...(filters.status ? { status: filters.status } : {}),
    ...(filters.from ? { from: filters.from } : {}),
    ...(filters.to ? { to: filters.to } : {}),
  }), [filters, urlQuery]);
  const listKey = JSON.stringify({ ...apiFilters, limit, offset: page * limit, refreshKey, revision });
  const [counts, setCounts] = useState<RequestCounts | null>(null);
  const { rows, total, loading, error, reload } = usePagedList(async () => {
    const res = await listRequests(JSON.parse(listKey));
    setCounts(res.counts);
    return res;
  }, listKey);

  const inPage = rows.find((r) => r.id === selected);
  const outside = useOrderResource<ProfileRequest | null>(
    () => (selected && !inPage ? getRequest(selected).catch(() => null) : Promise.resolve(null)),
    `${selected}:${!!inPage}:${revision}`, `${selected}`,
  );
  const request = inPage ?? outside.data ?? null;

  const updateUrl = (change: (next: URLSearchParams) => void, replace = false) => {
    const next = new URLSearchParams(params.toString());
    change(next);
    const href = `${pathname}?${next.toString()}`;
    if (replace) router.replace(href); else router.push(href);
  };
  const select = (id: number | null) => updateUrl((next) => { if (id) next.set("solicitud", String(id)); else next.delete("solicitud"); }, true);
  const setStatus = (status: RequestStatus) => updateUrl((next) => { if (filters.status === status) next.delete("estado"); else next.set("estado", status); next.delete("page"); });

  useEffect(() => { const m = matchMedia("(max-width:1279px)"); const update = () => setMobile(m.matches); update(); m.addEventListener("change", update); return () => m.removeEventListener("change", update); }, []);
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
      const map: [string, string][] = [["tipo", v.kind], ["estado", v.status], ["from", v.from], ["to", v.to]];
      for (const [key, value] of map) { if (value) next.set(key, value); else next.delete(key); }
      next.delete("page");
    });
  };
  const clearAll = () => { lastQuery.current = ""; setQuery(""); router.push(`${pathname}?tab=${tab}`); };
  const done = (r: ProfileRequest) => {
    setAccepting(null); setEditing(null); setRejecting(null);
    showToast(r.status === "approved" ? `Solicitud aceptada: ${r.kindLabel.toLowerCase()} de ${r.profile.name}.` : `Solicitud de ${r.profile.name} rechazada.`);
    setRevision((v) => v + 1);
  };

  const activeFilters = [filters.kind, filters.status, filters.from || filters.to].filter(Boolean).length;
  const columns = mode === "admin"
    ? ["Empresa", "Tipo", "Solicitado por", "Nombre perfil", "Estado", "Fecha de solicitud"]
    : ["Motivo", "Tipo", "Nombre perfil", "Estado", "Fecha de solicitud"];
  const detail = request
    ? <RequestDetail request={request} inModal={mobile} actions={canManage ? {
      onAccept: () => setAccepting(request),
      onEditAccept: request.kind === "new_profile" ? () => setEditing(request) : undefined,
      onReject: () => setRejecting(request),
    } : undefined} />
    : selected && outside.loading ? <div className="detail-skeleton" aria-label="Cargando solicitud" />
    : selected ? <p className="detail-empty">Esta solicitud no existe.</p>
    : <p className="detail-empty">Selecciona una solicitud para ver su detalle</p>;

  return (
    <>
      <div className="orders-kpis">
        <KpiCard label="Pendientes" count={counts?.pending} tone="warn" icon={<CircleHelp size={30} />} active={filters.status === "pending"} onClick={() => setStatus("pending")} />
        <KpiCard label="Aprobadas" count={counts?.approved} tone="ok" icon={<CircleCheck size={30} />} active={filters.status === "approved"} onClick={() => setStatus("approved")} />
        <KpiCard label="Rechazadas" count={counts?.rejected} tone="danger" icon={<CircleX size={30} />} active={filters.status === "rejected"} onClick={() => setStatus("rejected")} />
      </div>
      <div className="orders-toolbar clients-toolbar">
        {mode === "admin" && (
          <label className="orders-search">
            <span className="sr-only">Buscar nombre de empresa</span>
            <input placeholder="Buscar nombre de empresa" value={query} onChange={(e) => setQuery(e.target.value)} />
          </label>
        )}
        <button className="btn ghost" onClick={() => setFiltersOpen(true)}><Filter size={18} />Filtros{activeFilters ? ` (${activeFilters})` : ""}</button>
        <button className="btn ghost" onClick={clearAll}><FilterX size={18} />Limpiar filtros</button>
      </div>
      <div className="orders-workspace">
        <section className="orders-list clients-list" aria-label="Solicitudes">
          {error && rows.length > 0 && <p role="alert">No se pudo actualizar: {error}</p>}
          {error && rows.length === 0 ? <div role="alert">{error}<button className="btn ghost" onClick={reload}>Reintentar</button></div>
            : loading && rows.length === 0 ? <div className="orders-skeleton" aria-label="Cargando solicitudes">{Array.from({ length: 6 }, (_, i) => <div key={i} />)}</div>
            : rows.length === 0 ? <p className="pad">{activeFilters || urlQuery ? "Ninguna solicitud coincide con los filtros" : "No hay solicitudes"}</p>
            : (
              <div className="orders-table-scroll" tabIndex={0} aria-label="Tabla de solicitudes con desplazamiento horizontal">
                <table>
                  <thead><tr>{columns.map((h) => <th key={h}>{h}</th>)}</tr></thead>
                  <tbody>
                    {rows.map((r) => (
                      <tr key={r.id} onClick={() => select(r.id)} className={selected === r.id ? "order-selected" : ""}>
                        {mode === "admin"
                          ? <td><button className="order-number" aria-label={`Ver la solicitud de ${r.profile.name} (${r.companyName})`} onClick={() => select(r.id)}>{r.companyName}</button></td>
                          : <td title={r.reason}><button className="order-number" aria-label={`Ver la solicitud de ${r.profile.name}`} onClick={() => select(r.id)}>{r.reason}</button></td>}
                        <td>{r.kindLabel}</td>
                        {mode === "admin" && <td>{r.requestedBy.name || "—"}</td>}
                        <td>{r.profile.name}</td>
                        <td><Badge tone={STATUS_TONES[r.status]} label={STATUS_LABELS[r.status]} /></td>
                        <td>{dateLabel(r.requestedAt)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          <footer className="orders-pagination">
            <span>{total} solicitudes</span>
            <label>Por página <select value={limit} onChange={(e) => updateUrl((next) => { next.set("limit", e.target.value); next.set("page", "0"); })}>{PAGE_SIZES.map((n) => <option key={n} value={n}>{n}</option>)}</select></label>
            <button className="btn ghost" disabled={page === 0} onClick={() => updateUrl((next) => { next.set("page", String(page - 1)); })}>Anterior</button>
            <button className="btn ghost" disabled={(page + 1) * limit >= total} onClick={() => updateUrl((next) => { next.set("page", String(page + 1)); })}>Siguiente</button>
          </footer>
        </section>
        {!mobile && <aside className="orders-detail-panel clients-panel">{detail}</aside>}
      </div>
      {mobile && selected !== null && <Modal title="Detalle de solicitud" onClose={() => select(null)} panelClassName="registry-modal">{detail}</Modal>}
      {filtersOpen && <RequestFiltersModal values={filters} onApply={applyFilters} onClose={() => setFiltersOpen(false)} />}
      {accepting && (
        <ConfirmDialog title="Aceptar solicitud" confirmLabel="Aceptar solicitud" onConfirm={async () => done(await approveRequest(accepting.id))} onClose={() => setAccepting(null)}>
          <p>{accepting.kind === "new_profile"
            ? `Se creará el perfil Cliente de ${accepting.profile.name} en ${accepting.companyName}; recibirá en ${accepting.profile.email} el enlace para definir su contraseña.`
            : accepting.kind === "activation" ? `Se activará el perfil de ${accepting.profile.name}.` : `Se desactivará el perfil de ${accepting.profile.name}; no podrá ingresar a NexusWST.`}</p>
        </ConfirmDialog>
      )}
      {editing && (
        <ProfileFormModal scope="client" title="Editar y aceptar solicitud" submitLabel="Aceptar solicitud" lockCompany
          initial={{ name: editing.profile.name, email: editing.profile.email, phone: editing.profile.phone, jobTitle: editing.profile.jobTitle, companyId: editing.companyId }}
          companies={[{ id: editing.companyId, name: editing.companyName, active: true }]}
          submitData={async (body) => done(await approveRequest(editing.id, { name: body.name, email: body.email, phone: body.phone, jobTitle: body.jobTitle }))}
          onClose={() => setEditing(null)} />
      )}
      {rejecting && (
        <ConfirmDialog title="Rechazar solicitud" reason="required" reasonLabel="Motivo del rechazo" reasonRequiredMessage="Indica el motivo del rechazo."
          confirmLabel="Rechazar" destructive onConfirm={async (reason) => done(await rejectRequest(rejecting.id, reason))} onClose={() => setRejecting(null)}>
          <p>{rejecting.kindLabel} de {rejecting.profile.name} ({rejecting.companyName}). El Cliente verá el motivo.</p>
        </ConfirmDialog>
      )}
    </>
  );
}

function RequestFiltersModal({ values, onApply, onClose }: { values: FilterValues; onApply: (v: FilterValues) => void; onClose: () => void }) {
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
      <button type="button" className="btn ghost" onClick={() => setForm({ kind: "", status: "", from: "", to: "" })}>Limpiar</button>
      <button type="button" className="btn primary" onClick={apply}>Aplicar filtros</button>
    </>}>
      <div className="modal-field">
        <label htmlFor={`${id}-kind`}>Tipo</label>
        <select id={`${id}-kind`} value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value as FilterValues["kind"] })}>
          <option value="">Todos</option>
          {KINDS.map(([value, label]) => <option key={value} value={value}>{label}</option>)}
        </select>
      </div>
      <div className="modal-field">
        <label htmlFor={`${id}-status`}>Estado</label>
        <select id={`${id}-status`} value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value as FilterValues["status"] })}>
          <option value="">Todos</option>
          {(Object.keys(STATUS_LABELS) as RequestStatus[]).map((s) => <option key={s} value={s}>{STATUS_LABELS[s]}</option>)}
        </select>
      </div>
      <fieldset className="filter-dates">
        <legend>Fecha de solicitud</legend>
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
