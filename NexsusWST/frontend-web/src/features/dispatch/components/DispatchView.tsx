"use client";

import { useEffect, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Archive, Bell, LoaderCircle } from "lucide-react";
import { KpiCard } from "@/components/KpiCard";
import { PageHeader } from "@/components/PageHeader";
import { LiveBadge } from "@/components/StatusBadge";
import { Tabs } from "@/components/Tabs";
import { showToast } from "@/components/Toast";
import { CourierMark, useOrderResource } from "@/features/orders";
import { apiErrorMessage } from "@/services/api";
import { getCatalog, listCompanies } from "@/services/core";
import { createManifest, scanPackage } from "@/services/dispatch";
import { useAuth } from "@/state/auth";
import { usePickupList } from "../hooks/usePickupList";
import { clearMarks, pruneMarks, readCode, readMarks, saveMarks, scannedCodes, totalPackages, type LocalMarks, type ScanNotice } from "../pickupList";
import { PickupList } from "./PickupList";
import { PickupOrderModal } from "./PickupOrderModal";
import { ManifestModal } from "./ManifestModal";
import type { LineInfo } from "./ManifestAccordions";
import type { Manifest, ManifestLine } from "@/services/dispatch";
import { productsForCompany } from "@/services/orderReference";

export const DISPATCH_PATH = "/ordenes/retiro";
const NO_MARKS: LocalMarks = { marked: [], selected: [] };

// Retiro y entrega (H-13): se elige el courier, se leen los bultos y se genera el manifiesto.
export function DispatchView() {
  const { session, companyId } = useAuth();
  const params = useSearchParams(), router = useRouter(), pathname = usePathname();
  const courier = params.get("courier");
  // El Administrador consulta (matriz: «Lectura»); operar exige el permiso Despacho.
  const canOperate = !!session?.user.permissions.includes("despacho");
  const { couriers, rows, openManifest, error, reload, live } = usePickupList(courier, companyId);
  const refs = useOrderResource(() => Promise.all([listCompanies(), getCatalog("couriers")]), "dispatch-references");
  const companyIds = [...new Set((rows ?? []).map((r) => r.order.companyId))].sort((a, b) => a - b);
  const products = useOrderResource(async () => Object.fromEntries(await Promise.all(companyIds.map(async (id) => [id, await productsForCompany(id)] as const))), `dispatch-products:${companyIds.join(",")}`);
  // Lo marcado es de cada courier: se lee al elegirlo, antes de guardar nada.
  const [state, setState] = useState(() => ({ courier, marks: courier ? readMarks(courier) : NO_MARKS, notice: null as ScanNotice | null }));
  if (state.courier !== courier) setState({ courier, marks: courier ? readMarks(courier) : NO_MARKS, notice: null });
  const { marks, notice } = state;
  const setMarks = (update: (m: LocalMarks) => LocalMarks) => setState((s) => ({ ...s, marks: update(s.marks) }));
  const setNotice = (n: ScanNotice | null) => setState((s) => ({ ...s, notice: n }));
  const [generating, setGenerating] = useState(false);
  const [viewing, setViewing] = useState<number | null>(null);
  // Manifiesto en el modal. Se conserva mientras se muestra el resumen, aunque ya esté cerrado.
  const [shownManifest, setShownManifest] = useState<Manifest | null>(null);
  // Lo que salió de la lista (cambió de courier, pasó a Incidencia) deja de contar.
  useEffect(() => {
    if (rows) setMarks((m) => pruneMarks(m, rows));
  }, [rows]);
  useEffect(() => {
    if (state.courier) saveMarks(state.courier, state.marks);
  }, [state.courier, state.marks]);

  const courierName = (code: string) => refs.data?.[1].data.find((c) => c.code === code)?.name ?? code;
  const companyName = (id: number) => refs.data?.[0].data.find((c) => c.id === id)?.name ?? `Empresa ${id}`;
  const selectCourier = (code: string) => {
    const next = new URLSearchParams(params.toString());
    next.set("courier", code);
    router.replace(`${pathname}?${next}`);
  };

  const scan = async (raw: string) => {
    if (!rows || !courier) return;
    const result = readCode(rows, marks, raw, courierName(courier));
    if (result.kind === "rejected") return setNotice(result.notice);
    if (result.kind === "local") {
      setMarks((m) => ({ marked: [...m.marked, result.code], selected: m.selected.includes(result.row.order.id) ? m.selected : [...m.selected, result.row.order.id] }));
      return setNotice(result.notice);
    }
    try {
      await scanPackage(result.manifestId, result.code);
      setNotice({ tone: "ok", text: `Bulto ${result.code} · orden #${result.row.order.externalRef} marcado en el manifiesto #${result.manifestId}.` });
      reload();
    } catch (e) {
      setNotice({ tone: "error", text: apiErrorMessage(e) });
    }
  };
  const toggle = (orderId: number) =>
    setMarks((m) => ({ ...m, selected: m.selected.includes(orderId) ? m.selected.filter((id) => id !== orderId) : [...m.selected, orderId] }));

  // Crea el manifiesto con las seleccionadas y le pasa lo ya leído.
  const generate = async () => {
    if (!courier || !rows || marks.selected.length === 0) return;
    setGenerating(true);
    try {
      const ids = rows.filter((r) => marks.selected.includes(r.order.id) && scannedCodes(r, marks).length > 0).map((r) => r.order.id);
      const { manifest, excluded } = await createManifest(courier, ids);
      const included = new Set(manifest.lines.filter((l) => l.status === "included").map((l) => l.orderId));
      const codes = rows.filter((r) => included.has(r.order.id)).flatMap((r) => scannedCodes(r, marks));
      const failed: string[] = [];
      for (const code of codes) await scanPackage(manifest.id, code).catch(() => failed.push(code));
      clearMarks(courier);
      setMarks(() => NO_MARKS);
      setShownManifest(manifest);
      showToast(`Manifiesto #${manifest.id} generado con ${included.size} ${included.size === 1 ? "orden" : "órdenes"}.`, "success");
      if (excluded.length) showToast(`No entraron al manifiesto: ${excluded.join(" · ")}`, "warning");
      if (failed.length) showToast(`No se pudieron marcar: ${failed.join(", ")}. Vuelve a leerlos.`, "warning");
      reload();
    } catch (e) {
      showToast(apiErrorMessage(e), "error");
    } finally {
      setGenerating(false);
    }
  };

  const totalReady = couriers?.reduce((total, q) => total + q.orders, 0);
  const visibleRows = rows ?? [];
  const expected = visibleRows.reduce((total, row) => total + totalPackages(row), 0);
  const markedCount = visibleRows.reduce((total, row) => total + scannedCodes(row, marks).length, 0);
  const selectedCount = visibleRows.filter((row) => marks.selected.includes(row.order.id) && scannedCodes(row, marks).length > 0).length;
  // Datos de cada línea del manifiesto: la orden puede ya no estar en la lista (pasó a Incidencia).
  const lineInfo = (line: ManifestLine): LineInfo => {
    const order = rows?.find((r) => r.order.id === line.orderId)?.order;
    return { companyName: companyName(line.companyId), labelCode: order?.label.status === "ready" ? order.label.trackingCode ?? "" : "", items: order?.items ?? [] };
  };
  const productName = (companyId: number, sku: string) => products.data?.[companyId]?.find((p) => p.sku === sku)?.description ?? sku;
  // Si la orden sale de la lista mientras se mira (otra persona la movió), el detalle se cierra.
  const viewingRow = visibleRows.find((row) => row.order.id === viewing);

  return (
    <div className="orders-view dispatch-view">
      <PageHeader title="Retiro y entrega" actions={<>
        <LiveBadge live={live} />
        <button className="icon-btn" aria-label="Notificaciones"><Bell size={23} /></button>
      </>} />
      <Tabs items={[{ href: courier ? `${DISPATCH_PATH}?courier=${courier}` : DISPATCH_PATH, label: "Entrega de órdenes", active: true }]} />
      <div className="dispatch-top">
        <div className="orders-kpis">
          <KpiCard label="Lista para el retiro" count={totalReady} tone="ok" icon={<Archive size={30} />} />
        </div>
        <div className="dispatch-couriers" role="group" aria-labelledby="dispatch-couriers-title">
          <h2 id="dispatch-couriers-title">Couriers:</h2>
          <div className="dispatch-courier-strip">
            {couriers?.length === 0 && <p className="muted">No hay órdenes Listas para retiro.</p>}
            {couriers?.map((q) => (
              <button key={q.courier} type="button" className={`dispatch-courier ${courier === q.courier ? "is-active" : ""}`} aria-pressed={courier === q.courier} onClick={() => selectCourier(q.courier)}>
                <CourierMark code={q.courier} name={courierName(q.courier)} />
                <span className="dispatch-courier-name">{courierName(q.courier)}</span>
                <strong aria-label={`${q.orders} órdenes listas`}>{q.orders}</strong>
              </button>
            ))}
          </div>
        </div>
      </div>
      {error && <p role="alert">No se pudo actualizar: {error} <button className="btn ghost" onClick={reload}>Reintentar</button></p>}
      {!canOperate && <p className="form-notice">Solo lectura: para leer bultos y generar manifiestos necesitas el permiso Despacho.</p>}
      {courier && openManifest && <p className="form-notice">El manifiesto #{openManifest.id} de {courierName(courier)} está abierto: sus órdenes se leen directo en él.</p>}
      <div className="dispatch-work">
        {!courier ? (
          <section className="dispatch-panel"><h2>Lista de retiro</h2><p className="muted">Elige un courier para ver sus órdenes.</p></section>
        ) : rows ? (
          <PickupList courierName={courierName(courier)} rows={visibleRows} marked={marks.marked} selected={marks.selected} notice={notice} canOperate={canOperate} companyName={companyName} onScan={scan} onToggle={toggle} onView={(row) => setViewing(row.order.id)} />
        ) : (
          <section className="dispatch-panel"><div className="detail-skeleton dispatch-skeleton" aria-label="Cargando órdenes" /></section>
        )}
        <aside className="dispatch-panel dispatch-count-panel" aria-labelledby="dispatch-count-title">
          <h2 id="dispatch-count-title">Conteo al retirar</h2>
          <div className="dispatch-tally">
            <div><span>Esperados</span><strong>{expected}</strong></div>
            <div className="is-marked"><span>Marcados</span><strong aria-live="polite">{markedCount}</strong></div>
          </div>
          <div className="dispatch-count-foot">
            <p>Órdenes seleccionadas {selectedCount} de {visibleRows.length}</p>
            <button type="button" className="btn form-submit-success dispatch-action" disabled={!canOperate || selectedCount === 0 || generating || !!openManifest} aria-busy={generating} onClick={generate}>
              {generating && <LoaderCircle size={18} className="spin" aria-hidden="true" />}
              Generar manifiesto
            </button>
            {openManifest && (
              <>
                <button type="button" className="btn dispatch-action dispatch-open-manifest" onClick={() => setShownManifest(openManifest)}>Ver manifiesto abierto #{openManifest.id}</button>
                <p className="muted">Confirma la entrega de ese manifiesto antes de generar otro.</p>
              </>
            )}
          </div>
        </aside>
      </div>
      {shownManifest && (
        <ManifestModal key={shownManifest.id} manifest={openManifest?.id === shownManifest.id ? openManifest : shownManifest} courierName={courierName(shownManifest.courier)}
          canOperate={canOperate} userName={session?.user.name ?? ""} info={lineInfo} productName={productName}
          onClose={() => setShownManifest(null)} onClosed={() => { setShownManifest(null); reload(); }} />
      )}
      {viewingRow && (
        <PickupOrderModal row={viewingRow} companyName={companyName(viewingRow.order.companyId)} courierName={courierName(viewingRow.order.courier)} canOperate={canOperate}
          onClose={() => setViewing(null)} onReported={() => { setViewing(null); reload(); }} />
      )}
    </div>
  );
}
