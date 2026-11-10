"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { CirclePlus, Download, FileDown, FileSpreadsheet, Filter, FilterX, Upload } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { DropdownMenu } from "@/components/DropdownMenu";
import { Modal } from "@/components/Modal";
import { Badge } from "@/components/StatusBadge";
import { showToast } from "@/components/Toast";
import { useDebounce } from "@/hooks/useDebounce";
import {
  disableProfile, downloadProfilesTemplate, enableProfile, exportProfiles, getProfile, listProfiles, resetProfilePassword,
  type Profile, type ProfileFilters, type ProfileScope,
} from "@/services/clients";
import { listCompanies } from "@/services/core";
import { useAuth } from "@/state/auth";
import { dateLabel } from "@/features/orders/components/OrderDetail";
import { useOrderResource } from "@/features/orders/hooks/useOrderResources";
import { PERMISSION_LABELS, PROFILE_ROLE_LABELS, canManageProfiles } from "../access";
import { usePagedList } from "../hooks/usePagedList";
import { ImportProfilesDialog, ProfileFiltersModal, TemporaryPasswordDialog, countProfileFilters, type ProfileFilterValues } from "./ProfileDialogs";
import { ProfileDetail } from "./ProfileDetail";
import { ProfileFormModal } from "./ProfileFormModal";

const PAGE_SIZES = [20, 50, 100];

// Tabla de perfiles con su panel de detalle. Alcance «client»: Administración clientes › Perfiles
// («Todos los perfiles», Tarea 07). Alcance «internal»: Configuración › Usuarios WeStorage («Perfiles
// WeStorage», Tarea 08: correo o usuario, rol y permisos; sin empresa). Búsqueda, filtros, página y
// perfil abierto (?perfil=) viven en la URL. Sin «Gestionar perfiles» queda en lectura.
export function ProfilesTab({ scope, tab = "perfiles" }: { scope: ProfileScope; tab?: string }) {
  const { session } = useAuth();
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
  const [editing, setEditing] = useState<Profile | "new" | null>(null);
  const [toggling, setToggling] = useState<Profile | null>(null);
  const [resetting, setResetting] = useState<Profile | null>(null);
  const [importing, setImporting] = useState(false);
  const [temp, setTemp] = useState<{ name: string; login: string; password: string } | null>(null);
  const [mobile, setMobile] = useState(false);
  const [revision, setRevision] = useState(0);
  const canManage = canManageProfiles(session?.user, scope);
  const internal = scope === "internal";

  const filters = useMemo<ProfileFilterValues>(() => {
    const p = new URLSearchParams(paramsKey);
    const status = p.get("status");
    return { companyId: p.get("empresa_id") ?? "", status: status === "active" || status === "inactive" ? status : "", from: p.get("from") ?? "", to: p.get("to") ?? "" };
  }, [paramsKey]);
  const apiFilters = useMemo<ProfileFilters>(() => ({
    ...(urlQuery ? { q: urlQuery } : {}),
    ...(filters.companyId ? { company_id: Number(filters.companyId) } : {}),
    ...(filters.status ? { status: filters.status } : {}),
    ...(filters.from ? { from: filters.from } : {}),
    ...(filters.to ? { to: filters.to } : {}),
  }), [filters, urlQuery]);
  const filtersKey = JSON.stringify(apiFilters);
  const listKey = JSON.stringify({ scope, ...apiFilters, limit, offset: page * limit });
  const { rows, total, loading, error, reload } = usePagedList(() => listProfiles(scope, JSON.parse(listKey)), listKey);
  // Los perfiles de WeStorage no tienen empresa: no se pide la lista.
  const companiesRes = useOrderResource(() => (internal ? Promise.resolve({ data: [] }) : listCompanies()), `clients-companies:${scope}`);
  const companies = (companiesRes.data?.data ?? []).map((c) => ({ id: c.id, name: c.name, active: c.active }));

  const inPage = rows.find((r) => r.id === selected);
  const outside = useOrderResource<Profile | null>(
    () => (selected && !inPage ? getProfile(scope, selected).catch(() => null) : Promise.resolve(null)),
    `${selected}:${!!inPage}:${revision}`, `${selected}`,
  );
  const profile = inPage ?? outside.data ?? null;

  const updateUrl = (change: (next: URLSearchParams) => void, replace = false) => {
    const next = new URLSearchParams(params.toString());
    change(next);
    const href = `${pathname}?${next.toString()}`;
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
      const map: [string, string][] = [["empresa_id", v.companyId], ["status", v.status], ["from", v.from], ["to", v.to]];
      for (const [key, value] of map) { if (value) next.set(key, value); else next.delete(key); }
      next.delete("page");
    });
  };
  const clearAll = () => { lastQuery.current = ""; setQuery(""); router.push(`${pathname}?tab=${tab}`); };
  const toggle = (id: number) => setChecks((prev) => { const next = new Set(prev); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  const refresh = () => { reload(); setRevision((v) => v + 1); };
  const run = async (task: () => Promise<void>, ok: string) => {
    try {
      await task();
      showToast(ok);
    } catch (e) {
      showToast((e as Error).message, "error");
    }
  };
  const confirmToggle = async (reason: string) => {
    if (!toggling) return;
    const updated = toggling.active ? await disableProfile(scope, toggling.id, reason) : await enableProfile(scope, toggling.id, reason);
    setToggling(null);
    showToast(updated.active ? `Perfil ${updated.name} activado.` : `Perfil ${updated.name} desactivado. Ya no puede ingresar.`);
    refresh();
  };
  const confirmReset = async () => {
    if (!resetting) return;
    const res = await resetProfilePassword(scope, resetting.id);
    setResetting(null);
    if (res.temporaryPassword) setTemp({ name: resetting.name, login: resetting.username ?? "", password: res.temporaryPassword });
    else showToast(`Enviamos a ${resetting.email} el enlace para definir una contraseña nueva.`);
  };

  const activeFilters = countProfileFilters(filters);
  const searchLabel = internal ? "Buscar nombre o usuario" : "Buscar nombre del perfil o empresa";
  // TODO(PV-21): columna «Bodegas» en Usuarios WeStorage cuando exista la tabla de bodegas del usuario.
  const headers = internal
    ? ["Nombre completo", "Correo o usuario", "Rol", "Permisos", "Estado", "Fecha de creación"]
    : ["Nombre perfil", "Nombre empresa", "Correo", "Teléfono", "Estado", "Fecha de creación"];
  const excelItems = [
    ...(canManage ? [
      { label: "Descargar plantilla", icon: <FileDown size={18} aria-hidden="true" />, onSelect: () => run(() => downloadProfilesTemplate(scope), "Plantilla descargada.") },
      { label: "Importar perfiles", icon: <Upload size={18} aria-hidden="true" />, onSelect: () => setImporting(true) },
    ] : []),
    { label: `Exportar seleccionados${checks.size ? ` (${checks.size})` : ""}`, icon: <Download size={18} aria-hidden="true" />, disabled: checks.size === 0, onSelect: () => run(() => exportProfiles(scope, { ids: [...checks] }), "Exportación descargada.") },
    { label: "Exportar (filtros actuales)", icon: <Download size={18} aria-hidden="true" />, disabled: total === 0, onSelect: () => run(() => exportProfiles(scope, apiFilters), "Exportación descargada.") },
  ];
  const detail = profile
    ? <ProfileDetail profile={profile} scope={scope} canManage={canManage} inModal={mobile} onEdit={() => setEditing(profile)} onToggle={() => setToggling(profile)} onPassword={() => setResetting(profile)} />
    : selected && outside.loading ? <div className="detail-skeleton" aria-label="Cargando perfil" />
    : selected ? <p className="detail-empty">Este perfil no existe.</p>
    : <p className="detail-empty">Selecciona un perfil para ver su detalle</p>;

  return (
    <>
      <h2 className="section-title">{internal ? "Perfiles WeStorage" : "Todos los perfiles"}</h2>
      <div className="orders-toolbar clients-toolbar">
        <label className="orders-search">
          <span className="sr-only">{searchLabel}</span>
          <input placeholder={searchLabel} value={query} onChange={(e) => setQuery(e.target.value)} />
        </label>
        <button className="btn ghost" onClick={() => setFiltersOpen(true)}><Filter size={18} />Filtros{activeFilters ? ` (${activeFilters})` : ""}</button>
        <button className="btn ghost" onClick={clearAll}><FilterX size={18} />Limpiar filtros</button>
        <DropdownMenu className="excel-actions" triggerClassName="btn excel" trigger={<><FileSpreadsheet size={18} aria-hidden="true" />Excel acciones</>} items={excelItems} />
        {canManage && <button className="btn order-primary" onClick={() => setEditing("new")}><CirclePlus size={18} />Nuevo perfil</button>}
      </div>
      <div className="orders-workspace">
        <section className="orders-list clients-list" aria-label="Perfiles">
          {error && rows.length > 0 && <p role="alert">No se pudo actualizar: {error}</p>}
          {error && rows.length === 0 ? <div role="alert">{error}<button className="btn ghost" onClick={reload}>Reintentar</button></div>
            : loading && rows.length === 0 ? <div className="orders-skeleton" aria-label="Cargando perfiles">{Array.from({ length: 6 }, (_, i) => <div key={i} />)}</div>
            : rows.length === 0 ? <p className="pad">{activeFilters || urlQuery ? "Ningún perfil coincide con los filtros" : "No hay perfiles registrados"}</p>
            : (
              <div className="orders-table-scroll" tabIndex={0} aria-label="Tabla de perfiles con desplazamiento horizontal">
                <table>
                  <thead>
                    <tr>
                      <th><input aria-label="Seleccionar todos los perfiles de la página" type="checkbox" checked={rows.every((r) => checks.has(r.id))} onChange={(e) => setChecks(e.target.checked ? new Set(rows.map((r) => r.id)) : new Set())} /></th>
                      {headers.map((h) => <th key={h}>{h}</th>)}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((p) => (
                      <tr key={p.id} onClick={() => select(p.id)} className={selected === p.id ? "order-selected" : ""}>
                        <td onClick={(e) => e.stopPropagation()}><input type="checkbox" aria-label={`Seleccionar perfil ${p.name}`} checked={checks.has(p.id)} onChange={() => toggle(p.id)} /></td>
                        <td><button className="order-number" aria-label={`Ver detalle del perfil ${p.name}`} onClick={() => select(p.id)}>{p.name}</button></td>
                        {internal ? (
                          <>
                            <td title={p.email || p.username}>{p.email || (p.username ? `Usuario: ${p.username}` : "—")}</td>
                            <td>{PROFILE_ROLE_LABELS[p.role] ?? p.role}</td>
                            <td><span className="perm-chips">{p.permissions.length ? p.permissions.map((perm) => <Badge key={perm} tone="info" label={PERMISSION_LABELS[perm] ?? perm} />) : "—"}</span></td>
                          </>
                        ) : (
                          <>
                            <td>{p.companyName || "—"}</td>
                            <td title={p.email}>{p.email || "—"}</td>
                            <td>{p.phone || "—"}</td>
                          </>
                        )}
                        <td><Badge tone={p.active ? "ok" : "neutral"} label={p.active ? "Activo" : "Inactivo"} /></td>
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
      {mobile && selected !== null && (
        <Modal title={`Detalle de perfil ${profile?.name ?? ""}`} onClose={() => select(null)} panelClassName="registry-modal">{detail}</Modal>
      )}
      {filtersOpen && <ProfileFiltersModal values={filters} companies={internal ? undefined : companies} onApply={applyFilters} onClose={() => setFiltersOpen(false)} />}
      {editing && (
        <ProfileFormModal scope={scope} profile={editing === "new" ? undefined : editing} companies={companies} onClose={() => setEditing(null)}
          self={editing !== "new" && editing.id === session?.user.id}
          onSaved={(saved) => {
            setEditing(null);
            if (editing === "new") {
              if (saved.temporaryPassword) {
                setTemp({ name: saved.name, login: saved.username ?? "", password: saved.temporaryPassword });
                showToast(`Perfil ${saved.name} creado.`);
              } else showToast(`Perfil ${saved.name} creado. Le enviamos el enlace para definir su contraseña.`);
              select(saved.id);
            } else showToast(`Perfil ${saved.name} actualizado.`);
            refresh();
          }} />
      )}
      {toggling && (
        <ConfirmDialog
          title={toggling.active ? `Desactivar perfil ${toggling.name}` : `Activar perfil ${toggling.name}`}
          reason={toggling.active ? "required" : "optional"} reasonLabel="Motivo" reasonRequiredMessage="Indica el motivo de la desactivación."
          confirmLabel={toggling.active ? "Desactivar" : "Activar"} destructive={toggling.active}
          onConfirm={confirmToggle} onClose={() => setToggling(null)}>
          <p>{toggling.active ? "No podrá ingresar a NexusWST y se cerrarán sus sesiones abiertas. Su historial se conserva." : "Podrá volver a ingresar a NexusWST."}</p>
        </ConfirmDialog>
      )}
      {resetting && (
        <ConfirmDialog title={`Restablecer contraseña de ${resetting.name}`} confirmLabel="Restablecer" onConfirm={confirmReset} onClose={() => setResetting(null)}>
          <p>{resetting.hasEmail
            ? `Le enviaremos a ${resetting.email} un enlace para definir una contraseña nueva.`
            : "Se generará una contraseña temporal que verás una sola vez; sus sesiones abiertas se cerrarán."}</p>
        </ConfirmDialog>
      )}
      {temp && <TemporaryPasswordDialog name={temp.name} login={temp.login} password={temp.password} onClose={() => setTemp(null)} />}
      {importing && <ImportProfilesDialog scope={scope} onClose={() => setImporting(false)} onImported={refresh} />}
    </>
  );
}
