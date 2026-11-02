"use client";

import { useId, useState } from "react";
import { FormModal } from "@/components/FormModal";
import { showToast } from "@/components/Toast";
import { ApiError, apiErrorMessage } from "@/services/api";
import { assignWave, listOrders, listWaves, type Order, type OrderFilters } from "@/services/orders";
import { useOrderResource } from "../hooks/useOrderResources";

const PAGE = 200;

async function allAssignable(filters: OrderFilters, companyId: number | null) {
  const out: Order[] = [];
  for (let offset = 0; ; offset += PAGE) {
    const page = await listOrders({ ...filters, view: "asignacion", limit: PAGE, offset }, companyId);
    out.push(...page.data);
    if (out.length >= page.total || page.data.length === 0) return out;
  }
}

// «Asignar a ola de Invas»: las filas marcadas o, sin selección, las que muestran los filtros actuales.
export function AssignWaveModal({ selected, filters, companyId, onClose, onDone }: {
  selected: Order[];
  filters: OrderFilters;
  companyId: number | null;
  onClose: () => void;
  onDone: () => void;
}) {
  const id = useId();
  const usingSelection = selected.length > 0;
  const waves = useOrderResource(listWaves, "assign-waves");
  const candidates = useOrderResource(() => (usingSelection ? Promise.resolve(selected) : allAssignable(filters, companyId)), "assign-candidates");
  const [waveId, setWaveId] = useState("");
  const [waveError, setWaveError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<{ queued: number; skipped: number } | null>(null);
  const list = candidates.data ?? [];
  const eligible = list.filter((order) => order.status === "validated" && !order.waveId);
  const omitted = list.length - eligible.length;

  const submit = async () => {
    setFormError(null);
    setWaveError(waveId ? null : "Elige la ola.");
    if (!waveId) return;
    setBusy(true);
    try {
      const res = await assignWave(eligible.map((order) => order.id), waveId);
      const queued = res.queued.length;
      const skipped = omitted + res.skipped;
      showToast(`${queued} ${queued === 1 ? "orden encolada" : "órdenes encoladas"} para Invas.`, skipped ? "warning" : "success");
      setResult({ queued, skipped });
      onDone();
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) onDone();
      setFormError(apiErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  if (result) {
    return (
      <FormModal title="Resultado de la asignación" onClose={onClose}>
        <div className="bulk-summary">
          <p><strong>{result.queued}</strong> {result.queued === 1 ? "orden encolada" : "órdenes encoladas"}. Invas confirmará la asignación.</p>
          {result.skipped > 0 && <p><strong>{result.skipped}</strong> {result.skipped === 1 ? "omitida" : "omitidas"}: no estaban Validadas o ya tenían ola.</p>}
        </div>
      </FormModal>
    );
  }
  const count = eligible.length;
  return (
    <FormModal
      title="Asignar a ola de Invas"
      onClose={onClose}
      onSubmit={submit}
      submitLabel={usingSelection ? `Asignar ${count}` : `Sí, asignar ${count}`}
      submitDisabled={candidates.loading || count === 0}
      busy={busy}
      error={formError}
    >
      <div className="filter-fields">
        {candidates.error && <p className="modal-field-error" role="alert">No se pudieron cargar las órdenes: {candidates.error}</p>}
        {candidates.loading ? <div className="detail-skeleton config-skeleton" aria-label="Cargando órdenes" /> : (
          <div className="bulk-summary">
            <p>
              {usingSelection ? "Órdenes seleccionadas: " : "No hay filas seleccionadas. Se usarán las órdenes por asignar que muestran los filtros actuales: "}
              <strong>{count}</strong> {count === 1 ? "se asignará" : "se asignarán"}.
            </p>
            {omitted > 0 && <p>Se omiten <strong>{omitted}</strong>: no están Validadas o ya tienen ola.</p>}
          </div>
        )}
        <div className="modal-field">
          <label htmlFor={`${id}-wave`}>Ola de Invas *</label>
          <select id={`${id}-wave`} value={waveId} aria-invalid={!!waveError} onChange={(e) => { setWaveError(null); setWaveId(e.target.value); }}>
            <option value="">Elige una ola</option>
            {waves.data?.data.map((wave) => <option key={wave.id} value={wave.id}>{wave.name}</option>)}
          </select>
          {waveError && <span className="modal-field-error" role="alert">{waveError}</span>}
          {waves.error && <span className="modal-field-error" role="alert">No se pudieron cargar las olas de Invas: {waves.error}</span>}
        </div>
      </div>
    </FormModal>
  );
}
