"use client";

import { useId, useState } from "react";
import { Copy } from "lucide-react";
import { FormModal } from "@/components/FormModal";
import { ApiError, apiErrorMessage } from "@/services/api";
import type { CatalogItem } from "@/services/core";
import { createIntegration, type IntegrationKind, type IntegrationWithSecret } from "@/services/integrations";
import { supportedProviders } from "../providers";

// «Conectar cliente»: empresa, tipo y proveedor (solo los que acepta POST /v1/integrations) y, en un
// canal, el courier por defecto (opcional). El backend genera el secreto y lo devuelve una sola vez.
export function ConnectFormModal({ companies, couriers, initial, onClose, onCreated }: {
  companies: { id: number; name: string }[];
  couriers: CatalogItem[];
  initial?: { company?: string; kind?: "" | IntegrationKind; provider?: string };
  onClose: () => void;
  onCreated: (created: IntegrationWithSecret) => void;
}) {
  const id = useId();
  const startKind: IntegrationKind = initial?.kind || "channel";
  const [form, setForm] = useState({
    company: initial?.company ?? "",
    kind: startKind,
    provider: supportedProviders(startKind).some((p) => p.code === initial?.provider) ? initial!.provider! : "",
    defaultCourier: "",
  });
  const [errors, setErrors] = useState<{ company?: string; provider?: string }>({});
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const providers = supportedProviders(form.kind);

  const submit = async () => {
    const e: typeof errors = {};
    if (!form.company) e.company = "Elige la empresa.";
    if (!form.provider) e.provider = "Elige el proveedor.";
    setErrors(e);
    setFormError(null);
    if (Object.keys(e).length) return;
    setBusy(true);
    try {
      onCreated(await createIntegration({
        companyId: Number(form.company), kind: form.kind, provider: form.provider,
        ...(form.kind === "channel" && form.defaultCourier ? { defaultCourier: form.defaultCourier } : {}),
      }));
    } catch (err) {
      setFormError(err instanceof ApiError && err.status === 422 ? err.message : apiErrorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <FormModal title="Conectar cliente" onClose={onClose} onSubmit={submit} submitLabel="Conectar" submitTone="highlight" busy={busy} error={formError}>
      <div className="modal-field">
        <label htmlFor={`${id}-company`}>Empresa *</label>
        <select id={`${id}-company`} value={form.company} aria-invalid={!!errors.company} onChange={(e) => setForm({ ...form, company: e.target.value })}>
          <option value="">Selecciona una empresa</option>
          {companies.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
        </select>
        {errors.company && <span className="modal-field-error" role="alert">{errors.company}</span>}
      </div>
      <div className="modal-field">
        <label htmlFor={`${id}-kind`}>Tipo *</label>
        <select id={`${id}-kind`} value={form.kind} onChange={(e) => setForm({ ...form, kind: e.target.value as IntegrationKind, provider: "", defaultCourier: "" })}>
          <option value="channel">Canal (recibe órdenes)</option>
          <option value="courier">Courier (recibe seguimiento)</option>
        </select>
      </div>
      <div className="modal-field">
        <label htmlFor={`${id}-provider`}>Proveedor *</label>
        <select id={`${id}-provider`} value={form.provider} aria-invalid={!!errors.provider} onChange={(e) => setForm({ ...form, provider: e.target.value })}>
          <option value="">Selecciona un proveedor</option>
          {providers.map((p) => <option key={p.code} value={p.code}>{p.name}</option>)}
        </select>
        {errors.provider && <span className="modal-field-error" role="alert">{errors.provider}</span>}
      </div>
      {form.kind === "channel" && (
        <div className="modal-field">
          <label htmlFor={`${id}-courier`}>Courier por defecto · opcional</label>
          <select id={`${id}-courier`} value={form.defaultCourier} onChange={(e) => setForm({ ...form, defaultCourier: e.target.value })}>
            <option value="">Sin courier por defecto</option>
            {couriers.map((c) => <option key={c.code} value={c.code}>{c.name}</option>)}
          </select>
          <span className="form-hint">Se usa cuando la orden del canal llega sin courier.</span>
        </div>
      )}
      <p className="form-hint">Al conectar se genera el secreto de la conexión. Se muestra una sola vez.</p>
    </FormModal>
  );
}

function CopyRow({ label, value }: { label: string; value: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  };
  return (
    <div className="modal-field">
      <span className="secret-label">{label}</span>
      <p className="secret-row"><code>{value}</code>
        <button type="button" className="text-button" aria-label={`Copiar ${label.toLowerCase()}`} onClick={copy}><Copy size={16} aria-hidden="true" />{copied ? "Copiado" : "Copiar"}</button>
      </p>
    </div>
  );
}

// Secreto y URL del webhook tras crear o rotar. El secreto solo existe en las props de este
// diálogo (estado local de la pestaña): al cerrarlo se descarta y no se puede volver a ver.
export function SecretDialog({ title, name, secret, url, onClose }: { title: string; name: string; secret: string; url: string; onClose: () => void }) {
  return (
    <FormModal title={title} onClose={onClose} closeLabel="Listo">
      <p>Entrega estos datos a quien configure <strong>{name}</strong>.</p>
      <CopyRow label="URL del webhook" value={url} />
      <CopyRow label="Secreto" value={secret} />
      <p className="confirm-warning">El secreto se muestra una sola vez: al cerrar ya no se puede volver a ver. Si se pierde, hay que rotarlo.</p>
    </FormModal>
  );
}
