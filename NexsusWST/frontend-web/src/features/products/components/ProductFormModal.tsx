"use client";

import { useEffect, useId, useState, type ReactNode } from "react";
import { Plus, Trash2 } from "lucide-react";
import { FormModal } from "@/components/FormModal";
import { useDebounce } from "@/hooks/useDebounce";
import { ApiError, apiErrorMessage } from "@/services/api";
import { createCatalogProduct, listCatalog, updateCatalogProduct, type CatalogProduct, type ProductType } from "@/services/products";
import { GENERAL_WAVE_VALUE, type WaveOption } from "@/features/clients/waves";
import { TYPE_LABELS } from "../labels";

type Field = "companyId" | "sku" | "description" | "type" | "waveCode" | "components";
type Line = { key: number; sku: string; quantity: string };
export type CompanyChoice = { id: number; name: string; active: boolean };

// «Nuevo producto» y «Editar producto» (Tarea 09, SCRUM-106 y 236). Campos: cliente, SKU,
// descripción, tipo y ola («General (según reglas)» = sin ola propia). Cliente, SKU y tipo no se
// editan después de crear. Si es kit: componentes del mismo cliente (buscador de SKU) con su
// cantidad, al menos uno. Un kit no tiene stock propio.
export function ProductFormModal({ product, companies, waves, onClose, onSaved }: {
  product?: CatalogProduct;
  companies: CompanyChoice[];
  waves: WaveOption[];
  onClose: () => void;
  onSaved: (p: CatalogProduct) => void;
}) {
  const id = useId();
  const [form, setForm] = useState({
    companyId: String(product?.companyId ?? ""),
    sku: product?.sku ?? "",
    description: product?.description ?? "",
    type: (product?.type ?? "simple") as ProductType,
    wave: product?.waveCode ?? GENERAL_WAVE_VALUE,
  });
  const [lines, setLines] = useState<Line[]>(
    (product?.components ?? []).map((c, i) => ({ key: i, sku: c.sku, quantity: String(c.quantity) })),
  );
  const [nextKey, setNextKey] = useState(1000);
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({});
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const set = (change: Partial<typeof form>) => setForm((f) => ({ ...f, ...change }));
  const isKit = form.type === "kit";
  const options = companies.filter((c) => c.active || c.id === product?.companyId);

  // Buscador de componentes: SKU activos y simples del mismo cliente (sugerencias del <datalist>).
  const [term, setTerm] = useState("");
  const debounced = useDebounce(term);
  const [suggestions, setSuggestions] = useState<CatalogProduct[]>([]);
  useEffect(() => {
    if (!isKit || !form.companyId) return;
    let live = true;
    listCatalog({ company_id: Number(form.companyId), q: debounced, type: "simple", status: "active", limit: 20 })
      .then((r) => { if (live) setSuggestions(r.data); })
      .catch(() => { if (live) setSuggestions([]); });
    return () => { live = false; };
  }, [isKit, form.companyId, debounced]);

  const addLine = () => { setLines((l) => [...l, { key: nextKey, sku: "", quantity: "1" }]); setNextKey((k) => k + 1); };
  const setLine = (key: number, change: Partial<Line>) => setLines((l) => l.map((x) => (x.key === key ? { ...x, ...change } : x)));
  const removeLine = (key: number) => setLines((l) => l.filter((x) => x.key !== key));

  const validate = () => {
    const e: Partial<Record<Field, string>> = {};
    if (!product) {
      if (!form.companyId) e.companyId = "Elige el cliente.";
      if (!form.sku.trim()) e.sku = "El SKU es obligatorio.";
      else if (/\s/.test(form.sku.trim())) e.sku = "El SKU no puede tener espacios.";
    }
    if (!form.description.trim()) e.description = "La descripción es obligatoria.";
    if (isKit) {
      const filled = lines.filter((l) => l.sku.trim());
      if (filled.length === 0) e.components = "El kit debe tener al menos un componente.";
      else if (filled.some((l) => !(Number(l.quantity) > 0) || !Number.isInteger(Number(l.quantity)))) e.components = "La cantidad debe ser un número entero mayor que 0.";
    }
    return e;
  };

  const submit = async () => {
    setFormError(null);
    const e = validate();
    setErrors(e);
    if (Object.keys(e).length) return;
    const components = isKit ? lines.filter((l) => l.sku.trim()).map((l) => ({ sku: l.sku.trim().toUpperCase(), quantity: Number(l.quantity) })) : undefined;
    const waveCode = form.wave === GENERAL_WAVE_VALUE ? null : form.wave;
    setBusy(true);
    try {
      const saved = product
        ? await updateCatalogProduct(product.id, { description: form.description.trim(), waveCode, ...(components ? { components } : {}) })
        : await createCatalogProduct({ companyId: Number(form.companyId), sku: form.sku.trim(), description: form.description.trim(), type: form.type, waveCode, ...(components ? { components } : {}) });
      onSaved(saved);
    } catch (err) {
      if (err instanceof ApiError && err.fields.length) setErrors(Object.fromEntries(err.fields.map((f) => [f.field, f.message])));
      else setFormError(apiErrorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  const field = (name: Field, label: string, input: ReactNode, hint?: string) => (
    <div className="modal-field">
      <label htmlFor={`${id}-${name}`}>{label}</label>
      {input}
      {hint && !errors[name] && <span className="form-hint">{hint}</span>}
      {errors[name] && <span id={`${id}-${name}-error`} className="modal-field-error" role="alert">{errors[name]}</span>}
    </div>
  );
  const aria = (name: Field) => ({ id: `${id}-${name}`, "aria-invalid": !!errors[name], "aria-describedby": errors[name] ? `${id}-${name}-error` : undefined });
  const fixed = product ? "No se edita después de crear el producto." : undefined;

  return (
    <FormModal title={product ? `Editar producto ${product.sku}` : "Nuevo producto"} onClose={onClose} onSubmit={submit}
      submitLabel={product ? "Guardar cambios" : "Crear producto"} busy={busy} error={formError} wide>
      {field("companyId", "Cliente *", (
        <select {...aria("companyId")} value={form.companyId} disabled={!!product} onChange={(e) => { set({ companyId: e.target.value }); setLines([]); }}>
          <option value="" disabled>Elige el cliente</option>
          {options.map((c) => <option key={c.id} value={c.id}>{c.name}{c.active ? "" : " (desactivado)"}</option>)}
        </select>
      ), fixed)}
      {field("sku", "SKU *", <input {...aria("sku")} maxLength={60} autoComplete="off" value={form.sku} disabled={!!product} onChange={(e) => set({ sku: e.target.value })} />,
        fixed ?? "Único por cliente. Se guarda en mayúsculas.")}
      {field("description", "Descripción *", <input {...aria("description")} maxLength={200} value={form.description} onChange={(e) => set({ description: e.target.value })} />)}
      {field("type", "Tipo *", (
        <select {...aria("type")} value={form.type} disabled={!!product} onChange={(e) => { set({ type: e.target.value as ProductType }); if (e.target.value === "kit" && lines.length === 0) addLine(); }}>
          {(["simple", "kit"] as const).map((t) => <option key={t} value={t}>{TYPE_LABELS[t]}</option>)}
        </select>
      ), fixed)}
      {field("waveCode", "Ola", (
        <select {...aria("waveCode")} value={form.wave} onChange={(e) => set({ wave: e.target.value })}>
          {waves.map((w) => <option key={w.value} value={w.value}>{w.label}</option>)}
        </select>
      ), "«General (según reglas)»: sin ola propia; se usa la del cliente o las reglas de asignación.")}
      {isKit && (
        <fieldset className="modal-field kit-components" aria-describedby={errors.components ? `${id}-components-error` : undefined}>
          <legend>Composición *</legend>
          <span className="form-hint">Kit: no tiene stock propio. Componentes: artículos activos del mismo cliente.</span>
          {!form.companyId ? <span className="form-hint">Elige primero el cliente.</span> : (
            <>
              <datalist id={`${id}-skus`}>{suggestions.map((s) => <option key={s.id} value={s.sku}>{s.description}</option>)}</datalist>
              {lines.map((l, i) => (
                <div key={l.key} className="kit-line">
                  <input aria-label={`SKU del componente ${i + 1}`} list={`${id}-skus`} autoComplete="off" placeholder="Buscar SKU" value={l.sku}
                    onChange={(e) => { setLine(l.key, { sku: e.target.value }); setTerm(e.target.value); }} />
                  <input aria-label={`Cantidad del componente ${i + 1}`} type="number" min={1} step={1} value={l.quantity} onChange={(e) => setLine(l.key, { quantity: e.target.value })} />
                  <button type="button" className="icon-btn" aria-label={`Quitar el componente ${i + 1}`} onClick={() => removeLine(l.key)}><Trash2 size={18} /></button>
                </div>
              ))}
              <button type="button" className="btn ghost" onClick={addLine}><Plus size={18} />Agregar componente</button>
            </>
          )}
          {errors.components && <span id={`${id}-components-error`} className="modal-field-error" role="alert">{errors.components}</span>}
        </fieldset>
      )}
    </FormModal>
  );
}
