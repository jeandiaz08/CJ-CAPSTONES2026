"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { LoaderCircle } from "lucide-react";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { ReportProblemDialog } from "@/components/ReportProblemDialog";
import { showToast, type ToastTone } from "@/components/Toast";
import { dispatchLabel, useOrderResource } from "@/features/orders";
import { ApiError, apiErrorMessage } from "@/services/api";
import { productsForCompany } from "@/services/orderReference";
import { confirmPacking, releasePacking, reportIncident, setPackages, takeForPacking, type Order, type Package, type PackageInput } from "@/services/orders";
import { useAuth } from "@/state/auth";
import { allVerified, clearCounts, readCounts, saveCounts, type Counts } from "../verification";
import { LABEL_STATUS_TEXT, LabelStatusAction } from "./LabelStatusAction";
import { PackagesEditor } from "./PackagesEditor";
import { PACKING_PATH } from "./PackingQueue";
import { ProductVerification } from "./ProductVerification";

const MANAGER_ROLES = ["admin", "jefe_operaciones"];
const PROBLEM_KINDS = [{ value: "diferencia_empaque", label: "Diferencia de productos" }, { value: "otro", label: "Otro" }];

function holdNotice(order: Order, mine: boolean, manager: boolean) {
  if (order.status !== "packing") return `Esta orden ya no está En empaque (estado: ${order.statusLabel}).`;
  if (!order.claimedByName) return "La tomó un usuario deshabilitado. Puedes tomarla o soltarla.";
  if (!mine && !manager) return `La tiene tomada ${order.claimedByName}.`;
  return null;
}

// Trabajo sobre una orden en empaque: verificación, bultos, etiqueta y cierre.
export function PackingWork({ order, packages, courierName, onChanged }: {
  order: Order;
  packages: Package[];
  courierName: string;
  onChanged: () => Promise<void> | void;
}) {
  const { session } = useAuth();
  const router = useRouter();
  const items = order.items ?? [];
  const [counts, setCounts] = useState<Counts>(() => readCounts(order.id, items));
  const [dialog, setDialog] = useState<"courier" | "problem" | "release" | null>(null);
  const [savingPackages, setSavingPackages] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const products = useOrderResource(() => productsForCompany(order.companyId), `packing-products:${order.companyId}`);
  useEffect(() => saveCounts(order.id, counts), [order.id, counts]);

  const manager = MANAGER_ROLES.includes(session?.user.role ?? "");
  const mine = order.claimedBy === session?.user.id;
  const orphan = order.status === "packing" && !order.claimedByName;
  const canWork = order.status === "packing" && (mine || manager);
  const notice = holdNotice(order, mine, manager);
  const names = Object.fromEntries((products.data ?? []).map((p) => [p.sku, p.description]));
  const max = order.courierMaxPackages;
  const overLimit = max != null && packages.length > max;
  const manifestId = packages.find((p) => p.manifestId)?.manifestId;
  const canConfirm = canWork && allVerified(items, counts) && packages.length > 0 && !overLimit;

  const finish = (message: string, tone: ToastTone) => {
    clearCounts(order.id);
    showToast(message, tone);
    router.push(PACKING_PATH);
  };
  const savePackages = async (next: PackageInput[]) => {
    setSavingPackages(true);
    setError(null);
    try {
      await setPackages(order.id, next);
      await onChanged();
    } catch (e) {
      setError(apiErrorMessage(e));
    } finally {
      setSavingPackages(false);
    }
  };
  const confirm = async () => {
    setConfirming(true);
    setError(null);
    try {
      const res = await confirmPacking(order.id, items.map((i) => ({ sku: i.sku, quantity: counts[i.sku] ?? 0 })), order.version);
      if (res.order.status !== "ready_pickup") {
        const detail = res.discrepancies?.length ? res.discrepancies.join("; ") : res.order.incidentLabel ?? "revisa la orden";
        finish(`La orden #${order.externalRef} pasó a Incidencia: ${detail}.`, "warning");
      } else if (res.order.label.status === "ready" || res.order.label.status === "office") {
        finish(`La orden #${order.externalRef} está lista para retiro.`, "success");
      } else {
        finish(`La orden #${order.externalRef} está lista para retiro, pero su etiqueta sigue pendiente.`, "warning");
      }
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        showToast(apiErrorMessage(e), "warning");
        await onChanged();
      } else {
        setError(apiErrorMessage(e));
      }
    } finally {
      setConfirming(false);
    }
  };
  const takeOver = async () => {
    setError(null);
    try {
      await takeForPacking(order.id);
      await onChanged();
    } catch (e) {
      setError(apiErrorMessage(e));
    }
  };
  const report = async (kind: string, reason: string) => {
    await reportIncident(order.id, kind, reason);
    finish(`La orden #${order.externalRef} pasó a Incidencia.`, "warning");
  };

  return (
    <>
      {notice && (
        <div className="form-notice packing-notice" role="status">
          <span>{notice}</span>
          {orphan && manager && <button type="button" className="btn primary packing-action" onClick={takeOver}>Tomar orden</button>}
        </div>
      )}
      <div className="packing-work">
        <ProductVerification reference={order.externalRef} items={items} names={names} counts={counts} onChange={setCounts} disabled={!canWork} />
        <section className="packing-panel packing-side">
          <div className="packing-panel-head"><h2>Envío y bultos</h2></div>
          <dl className="packing-data">
            <div><dt>Courier:</dt><dd>{courierName} - {dispatchLabel(order.dispatchType)}</dd></div>
            <div><dt>Cód. etiqueta:</dt><dd>{order.label.trackingCode || "—"}</dd></div>
            <div><dt>Estado etiqueta:</dt><dd>{LABEL_STATUS_TEXT[order.label.status]}</dd></div>
            <div><dt>Manifiesto:</dt><dd>{manifestId ? `#${manifestId}` : "—"}</dd></div>
          </dl>
          <LabelStatusAction order={order} packagesCount={packages.length} disabled={!canWork} onChanged={onChanged} />
          {overLimit && (
            <p className="packing-limit" role="alert">
              {courierName} admite {max === 1 ? "una sola etiqueta" : `${max} etiquetas`} por orden. Con {packages.length} bultos no se puede enviar por este courier. Informa el cambio de courier.
            </p>
          )}
          <div className="packing-side-foot">
            <PackagesEditor packages={packages} busy={savingPackages} disabled={!canWork} onSave={savePackages} />
            {error && <p className="form-modal-error" role="alert">{error}</p>}
            <button type="button" className="btn form-submit-success packing-action" disabled={!canConfirm || confirming} aria-busy={confirming} onClick={confirm}>
              {confirming && <LoaderCircle size={18} className="spin" aria-hidden="true" />}
              Confirmar Empaque
            </button>
            <button type="button" className="btn packing-action packing-action-courier" disabled={!canWork} onClick={() => setDialog("courier")}>Informar cambio de courier</button>
            <button type="button" className="btn packing-action" disabled={!canWork} onClick={() => setDialog("problem")}>Informar otro problema</button>
            <button type="button" className="btn ghost packing-action" disabled={!canWork} onClick={() => setDialog("release")}>Soltar orden</button>
          </div>
        </section>
      </div>
      {dialog === "courier" && (
        <ConfirmDialog title="Informar cambio de courier" reason="required" reasonLabel="Motivo" confirmLabel="Informar" onClose={() => setDialog(null)} onConfirm={(reason) => report("courier_bultos", reason)}>
          <p>La orden pasa a Incidencia y el Jefe de Operaciones decide el cambio de courier.</p>
        </ConfirmDialog>
      )}
      {dialog === "problem" && (
        <ReportProblemDialog title="Informar otro problema" kinds={PROBLEM_KINDS} onClose={() => setDialog(null)} onConfirm={(kind, reason) => report(kind, reason)} />
      )}
      {dialog === "release" && (
        <ConfirmDialog title="Soltar orden" confirmLabel="Soltar orden" onClose={() => setDialog(null)} onConfirm={async () => {
          await releasePacking(order.id);
          finish(`La orden #${order.externalRef} volvió a la cola.`, "info");
        }}>
          <p>La orden vuelve a «Por empacar» y otra persona podrá tomarla. Los bultos registrados se conservan.</p>
        </ConfirmDialog>
      )}
    </>
  );
}
