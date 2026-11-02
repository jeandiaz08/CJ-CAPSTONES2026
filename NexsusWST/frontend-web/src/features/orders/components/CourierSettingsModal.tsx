"use client";

import { useId, useState } from "react";
import { Pencil } from "lucide-react";
import { FormModal } from "@/components/FormModal";
import { OptionCards } from "@/components/OptionCards";
import { showToast } from "@/components/Toast";
import { apiErrorMessage } from "@/services/api";
import { getCatalog, type CatalogItem } from "@/services/core";
import { getCutoffs, listCourierSettings, saveCourierSettings, type CourierSettings } from "@/services/orders";
import { useOrderResource } from "../hooks/useOrderResources";

function CourierForm({ settings, name, onClose, onSaved }: { settings: CourierSettings; name: string; onClose: () => void; onSaved: () => void }) {
  const id = useId();
  const [maxPackages, setMaxPackages] = useState(settings.maxPackages != null ? String(settings.maxPackages) : "");
  const [coverage, setCoverage] = useState(settings.coversAll ? "all" : "list");
  const [selected, setSelected] = useState<string[]>(settings.communes ?? []);
  const [search, setSearch] = useState("");
  const [errors, setErrors] = useState<{ max?: string; communes?: string }>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const catalog = useOrderResource(() => getCatalog("communes").then((r) => (r.data as unknown as { name: string }[]).map((c) => c.name)), "communes-names");
  const term = search.trim().toLowerCase();
  const visible = (catalog.data ?? []).filter((commune) => commune.toLowerCase().includes(term));
  const toggle = (commune: string) => setSelected((current) => current.includes(commune) ? current.filter((value) => value !== commune) : [...current, commune]);
  const submit = async () => {
    setFormError(null);
    const parsed = maxPackages.trim() ? Number(maxPackages) : null;
    const found = {
      ...(parsed !== null && (!Number.isInteger(parsed) || parsed <= 0) ? { max: "Debe ser un entero mayor que 0." } : {}),
      ...(coverage === "list" && selected.length === 0 ? { communes: "Elige al menos una comuna." } : {}),
    };
    setErrors(found);
    if (Object.keys(found).length) return;
    setBusy(true);
    try {
      await saveCourierSettings(settings.courier, { maxPackages: parsed, coversAll: coverage === "all", communes: coverage === "all" ? [] : selected });
      showToast(`Courier ${name} actualizado.`);
      onSaved();
    } catch (e) {
      setFormError(apiErrorMessage(e));
    } finally {
      setBusy(false);
    }
  };
  const communes = (
    <div className="config-communes">
      <div className="modal-field">
        <label htmlFor={`${id}-search`}>Buscar comuna</label>
        <input id={`${id}-search`} value={search} onChange={(e) => setSearch(e.target.value)} />
      </div>
      <p className="form-hint" role="status">{selected.length} {selected.length === 1 ? "comuna elegida" : "comunas elegidas"}</p>
      <div className="config-commune-list">
        {visible.map((commune) => (
          <label key={commune}>
            <input type="checkbox" checked={selected.includes(commune)} onChange={() => toggle(commune)} />
            {commune}
          </label>
        ))}
      </div>
    </div>
  );
  return (
    <FormModal title={`Courier ${name}`} onClose={onClose} onSubmit={submit} submitLabel="Guardar courier" busy={busy} error={formError}>
      <div className="filter-fields">
        <div className="modal-field">
          <label htmlFor={`${id}-max`}>Máximo de bultos por orden</label>
          <input id={`${id}-max`} type="number" min={1} step={1} value={maxPackages} placeholder="Sin límite" aria-invalid={!!errors.max} onChange={(e) => setMaxPackages(e.target.value)} />
          {errors.max && <span className="modal-field-error" role="alert">{errors.max}</span>}
        </div>
        <OptionCards legend="Cobertura" value={coverage} onChange={setCoverage} error={errors.communes} options={[
          { value: "all", title: "Todas las comunas" },
          { value: "list", title: "Lista de comunas", children: communes },
        ]} />
      </div>
    </FormModal>
  );
}

// «Configurar courier»: los couriers los crea Soporte. Aquí se ajustan bultos y cobertura;
// los horarios se ven en solo lectura para no tener dos lugares que editen lo mismo.
export function CourierSettingsModal({ couriers, onClose, onChanged, onOpenCutoffs }: { couriers: CatalogItem[]; onClose: () => void; onChanged: () => void; onOpenCutoffs: () => void }) {
  const [version, setVersion] = useState(0);
  const settings = useOrderResource(listCourierSettings, `courier-settings:${version}`, "courier-settings");
  const cuts = useOrderResource(getCutoffs, "courier-cutoffs");
  const [editing, setEditing] = useState<CourierSettings | null>(null);
  const courierName = (code: string) => couriers.find((c) => c.code === code)?.name ?? code;
  return (
    <FormModal wide title="Configurar courier" onClose={onClose}>
      <p className="form-notice config-notice">Los couriers los crea Soporte junto con su integración. Aquí se ajusta su máximo de bultos y su cobertura.</p>
      {settings.error && <p className="modal-field-error" role="alert">No se pudieron cargar los couriers: {settings.error}</p>}
      {settings.loading && <div className="detail-skeleton config-skeleton" aria-label="Cargando couriers" />}
      <ul className="config-list">
        {settings.data?.data.map((courier) => {
          const stops = (cuts.data?.data ?? []).flatMap((cutoff) => cutoff.couriers.filter((c) => c.code === courier.courier).map((c) => `${cutoff.name || `Corte ${cutoff.number}`} (${cutoff.cutoffTime}) · llega ${c.arrivalTime}`));
          return (
            <li key={courier.courier} className="config-row">
              <div className="config-text">
                <strong>{courierName(courier.courier)}</strong>
                <small>Máx. bultos: {courier.maxPackages ?? "sin límite"} · Cobertura: {courier.coversAll ? "todas las comunas" : `${courier.communes?.length ?? 0} comunas`}</small>
                <small>{stops.length ? stops.join(" | ") : "No está en ningún corte"} · <button type="button" className="text-button config-link" onClick={onOpenCutoffs}>Editar en Configurar cortes</button></small>
              </div>
              <div className="config-actions">
                <button type="button" className="toast-close" aria-label={`Editar ${courierName(courier.courier)}`} onClick={() => setEditing(courier)}><Pencil size={18} /></button>
              </div>
            </li>
          );
        })}
      </ul>
      {editing && <CourierForm settings={editing} name={courierName(editing.courier)} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); setVersion((v) => v + 1); onChanged(); }} />}
    </FormModal>
  );
}
