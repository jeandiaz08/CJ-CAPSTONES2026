"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { CirclePlus, FileSpreadsheet, Filter, FilterX } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { Modal } from "@/components/Modal";
import { Badge } from "@/components/StatusBadge";
import { showToast } from "@/components/Toast";
import { useDebounce } from "@/hooks/useDebounce";
import { createRequest, exportProfiles, getProfile, listProfiles, listRequests, type Profile, type ProfileFilters, type RequestKind } from "@/services/clients";
import { dateLabel } from "@/features/orders/components/OrderDetail";
import { useOrderResource } from "@/features/orders/hooks/useOrderResources";
import { usePagedList } from "../hooks/usePagedList";
import { ProfileFiltersModal, countProfileFilters, type ProfileFilterValues } from "./ProfileDialogs";
import { ProfileDetail } from "./ProfileDetail";
import { RequestProfileModal } from "./RequestProfileModal";

const PAGE_SIZES = [20, 50, 100];
export const SENT_MESSAGE = "Solicitud enviada; te avisaremos cuando el Administrador la revise.";

// Clientes › Perfiles empresa › «Mis perfiles» (perfil Cliente, Tarea 07 Fase 5). Solo los perfiles de
// su empresa (lo fija el servidor). No crea ni cambia perfiles: solicita (nuevo perfil, activación o
// desactivación). Si ya hay una solicitud pendiente, el botón queda deshabilitado y la fila lo muestra.
export function MyProfilesTab({ companyName, onRequested }: { companyName: string; onRequested: () => void }) {
  const params = useSearchParams(), router = useRouter(), pathname = usePathname();
  const paramsKey = params.toString();
  const page = Math.max(0, Number(params.get("page")) || 0);
  const limit = PAGE_SIZES.includes(Number(params.get("limit"))) ? Number(params.get("limit")) : 20;
  const selected = Number(params.get("perfil")) || null;
  const urlQuery = params.get("q") ?? "";
  const [query, setQuery] = useState(urlQuery);
  const debouncedQuery = useDebounce(query);
  const lastQuery = useRef(urlQuery);
  const [checks, setChecks] = useState<Set<number>>(new Set());
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [requesting, setRequesting] = useState(false);
  const [asking, setAsking] = useState<{ profile: Profile; kind: RequestKind } | null>(null);
  const [mobile, setMobile] = useState(false);
  const [revision, setRevision] = useState(0);

  const filters = useMemo<ProfileFilterValues>(() => {
    const p = new URLSearchParams(paramsKey);
    const status = p.get("status");
    return { companyId: "", status: status === "active" || status === "inactive" ? status : "", from: p.get("from") ?? "", to: p.get("to") ?? "" };
  }, [paramsKey]);
  const apiFilters = useMemo<ProfileFilters>(() => ({
    ...(urlQuery ? { q: urlQuery } : {}),
    ...(filters.status ? { status: filters.status } : {}),
    ...(filters.from ? { from: filters.from } : {}),
    ...(filters.to ? { to: filters.to } : {}),
  }), [filters, urlQuery]);
  const filtersKey = JSON.stringify(apiFilters);
  const listKey = JSON.stringify({ ...apiFilters, limit, offset: page * limit, revision });
  const { rows, total, loading, error, reload } = usePagedList(() => listProfiles("client", JSON.parse(listKey)), listKey);
  // Solicitudes pendientes de su empresa: «Solicitud pendiente» en la fila y en el panel.
  const pending = useOrderResource(() => listRequests({ status: "pending", limit: 200 }), `pending:${revision}`, "pending");
  const pendingBy = new Map<number, RequestKind[]>();
  for (const r of pending.data?.data ?? []) if (r.targetUserId) pendingBy.set(r.targetUserId, [...(pendingBy.get(r.targetUserId) ?? []), r.kind]);

  const inPage = rows.find((r) => r.id === selected);
  const outside = useOrderResource<Profile | null>(
    () => (selected && !inPage ? getProfile("client", selected).catch(() => null) : Promise.resolve(null)),
    `${selected}:${!!inPage}:${revision}`, `${selected}`,
  );
  const profile = inPage ?? outside.data ?? null;

  const updateUrl = (change: (next: URLSearchParams) => void, replace = false) => {
    const next = new URLSearchParams(params.toString());
    change(next);
    const search = next.toString();
    const href = search ? `${pathname}?${search}` : pathname;
    if (replace) router.replace(href); else router.push(href);
  };
  const select = (id: number | null) => updateUrl((next) => { if (id) next.set("perfil", String(id)); else next.delete("perfil"); }, true);

  useEffect(() => { const m = matchMedia("(max-width:1279px)"); const update = () => setMobile(m.matches); update(); m.addEventListener("change", update); return () => m.removeEventListener("change", update); }, []);
  useEffect(() => { setChecks(new Set()); }, [page, limit, filtersKey]);
  useEffect(() => { if (urlQuery !== lastQuery.current) { lastQuery.current = urlQuery; setQuery(urlQuery); } }, [urlQuery]);
  useEffect(() => {
    if (debouncedQuery === lastQuery.current) return;
    lastQuery.current = debouncedQuery;
    updateUrl((next) => { if (debouncedQuery) next.set("q", debouncedQuery); else next.delete("q"); next.delete("page"); }, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedQuery]);

  const applyFilters = (v: ProfileFilterValues) => {
    setFiltersOpen(false);
    updateUrl((next) => {
      const map: [string, string][] = [["status", v.status], ["from", v.from], ["to", v.to]];
      for (const [key, value] of map) { if (value) next.set(key, value); else next.delete(key); }
      next.delete("page");
    });
  };
  const clearAll = () => { lastQuery.current = ""; setQuery(""); router.push(pathname); };
  const toggle = (id: number) => setChecks((prev) => { const next = new Set(prev); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  const sent = () => { showToast(SENT_MESSAGE); setRevision((v) => v + 1); onRequested(); };
  const exportExcel = async () => {
    try {
      await exportProfiles("client", checks.size ? { ids: [...checks] } : apiFilters);
      showToast("Exportación descargada.");
    } catch (e) {
      showToast((e as Error).message, "error");
    }
  };

  const activeFilters = countProfileFilters(filters);
  const requestAction = (p: Profile) => {
    const kind: RequestKind = p.active ? "deactivation" : "activation";
    const waiting = pendingBy.get(p.id)?.includes(kind);
    const label = p.active ? "Solicitar desactivación" : "Solicitar activación";
    return waiting
      ? <button type="button" className="btn ghost" disabled title="Ya hay una solicitud pendiente para este perfil">Solicitud pendiente</button>
      : <button type="button" className={p.active ? "btn danger" : "btn success"} onClick={() => setAsking({ profile: p, kind })}>{label}</button>;
  };
  const detail = profile
    ? <ProfileDetail profile={profile} canManage={false} inModal={mobile} actions={requestAction(profile)} onEdit={() => {}} onToggle={() => {}} onPassword={() => {}} />
    : selected && outside.loading ? <div className="detail-skeleton" aria-label="Cargando perfil" />
    : selected ? <p className="detail-empty">Este perfil no existe.</p>
    : <p className="detail-empty">Selecciona un perfil para ver su detalle</p>;

  return (
    <>
      <h2 className="section-title">Perfiles {companyName}</h2>
      <div className="orders-toolbar clients-toolbar">
        <label className="orders-search">
          <span className="sr-only">Buscar nombre del perfil</span>
          <input placeholder="Buscar nombre del perfil" value={query} onChange={(e) => setQuery(e.target.value)} />
        </label>
        <button className="btn ghost" onClick={() => setFiltersOpen(true)}><Filter size={18} />Filtros{activeFilters ? ` (${activeFilters})` : ""}</button>
        <button className="btn ghost" onClick={clearAll}><FilterX size={18} />Limpiar filtros</button>
        <button className="btn excel" disabled={total === 0} onClick={exportExcel}><FileSpreadsheet size={18} aria-hidden="true" />Exportar Excel{checks.size ? ` (${checks.size})` : ""}</button>
        <button className="btn order-primary" onClick={() => setRequesting(true)}><CirclePlus size={18} />Solicitar nuevo perfil</button>
      </div>
      <div className="orders-workspace">
        <section className="orders-list clients-list" aria-label="Perfiles de la empresa">
          {error && rows.length > 0 && <p role="alert">No se pudo actualizar: {error}</p>}
          {error && rows.length === 0 ? <div role="alert">{error}<button className="btn ghost" onClick={reload}>Reintentar</button></div>
            : loading && rows.length === 0 ? <div className="orders-skeleton" aria-label="Cargando perfiles">{Array.from({ length: 6 }, (_, i) => <div key={i} />)}</div>
            : rows.length === 0 ? <p className="pad">{activeFilters || urlQuery ? "Ningún perfil coincide con los filtros" : "Tu empresa aún no tiene perfiles"}</p>
            : (
              <div className="orders-table-scroll" tabIndex={0} aria-label="Tabla de perfiles con desplazamiento horizontal">
                <table>
                  <thead>
                    <tr>
                      <th><input aria-label="Seleccionar todos los perfiles de la página" type="checkbox" checked={rows.every((r) => checks.has(r.id))} onChange={(e) => setChecks(e.target.checked ? new Set(rows.map((r) => r.id)) : new Set())} /></th>
                      {["Nombre completo", "Correo", "Teléfono", "Estado", "Fecha de creación"].map((h) => <th key={h}>{h}</th>)}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((p) => (
                      <tr key={p.id} onClick={() => select(p.id)} className={selected === p.id ? "order-selected" : ""}>
                        <td onClick={(e) => e.stopPropagation()}><input type="checkbox" aria-label={`Seleccionar perfil ${p.name}`} checked={checks.has(p.id)} onChange={() => toggle(p.id)} /></td>
                        <td><button className="order-number" aria-label={`Ver detalle del perfil ${p.name}`} onClick={() => select(p.id)}>{p.name}</button></td>
                        <td title={p.email}>{p.email || "—"}</td>
                        <td>{p.phone || "—"}</td>
                        <td className="status-cell">
                          <Badge tone={p.active ? "ok" : "neutral"} label={p.active ? "Activo" : "Inactivo"} />
                          {pendingBy.has(p.id) && <Badge tone="warn" label="Solicitud pendiente" />}
                        </td>
                        <td>{dateLabel(p.createdAt)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          <footer className="orders-pagination">
            <span>{total} perfiles · {checks.size} seleccionados</span>
            <label>Por página <select value={limit} onChange={(e) => updateUrl((next) => { next.set("limit", e.target.value); next.set("page", "0"); })}>{PAGE_SIZES.map((n) => <option key={n} value={n}>{n}</option>)}</select></label>
            <button className="btn ghost" disabled={page === 0} onClick={() => updateUrl((next) => { next.set("page", String(page - 1)); })}>Anterior</button>
            <button className="btn ghost" disabled={(page + 1) * limit >= total} onClick={() => updateUrl((next) => { next.set("page", String(page + 1)); })}>Siguiente</button>
          </footer>
        </section>
        {!mobile && <aside className="orders-detail-panel clients-panel">{detail}</aside>}
      </div>
      {mobile && selected !== null && <Modal title={`Detalle de perfil ${profile?.name ?? ""}`} onClose={() => select(null)} panelClassName="registry-modal">{detail}</Modal>}
      {filtersOpen && <ProfileFiltersModal values={filters} onApply={applyFilters} onClose={() => setFiltersOpen(false)} />}
      {requesting && <RequestProfileModal onClose={() => setRequesting(false)} onSent={() => { setRequesting(false); sent(); }} />}
      {asking && (
        <ConfirmDialog title={asking.kind === "activation" ? `Solicitar activación de ${asking.profile.name}` : `Solicitar desactivación de ${asking.profile.name}`}
          reason="required" reasonLabel="Motivo" reasonRequiredMessage="Indica el motivo de la solicitud." confirmLabel="Enviar solicitud"
          onConfirm={async (reason) => { await createRequest({ kind: asking.kind, targetUserId: asking.profile.id, reason }); setAsking(null); sent(); }}
          onClose={() => setAsking(null)}>
          <p>El Administrador revisará la solicitud. Mientras tanto, el perfil sigue {asking.profile.active ? "activo" : "inactivo"}.</p>
        </ConfirmDialog>
      )}
    </>
  );
}
