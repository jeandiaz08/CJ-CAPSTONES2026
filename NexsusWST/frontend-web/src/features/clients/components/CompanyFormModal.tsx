"use client";

import { useId, useState, type ReactNode } from "react";
import { FormModal } from "@/components/FormModal";
import { ApiError, apiErrorMessage } from "@/services/api";
import { createCompany, updateCompany, type AdminCompany, type CompanyInput } from "@/services/clients";
import { normalizeRut } from "../rut";
import { GENERAL_WAVE_VALUE, waveOptions } from "../waves";

type Field = "name" | "taxId" | "email" | "phone" | "waveCode" | "wmsCode";
const EMAIL = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;
const UNSET = ""; // ola sin elegir: el formulario no se envía

// «Nueva empresa» y «Editar empresa». Lo obligatorio es lo que exige la base de datos (nombre, RUT
// único) más la ola, que se elige siempre: «General (según reglas)» se guarda como NULL (PV-19).
// El código Invas solo lo ve y lo cambia el Administrador (showWms).
export function CompanyFormModal({ company, waves, wavesError, showWms, onClose, onSaved }: {
  company?: AdminCompany;
  waves: { code: string; name: string }[];
  wavesError?: string;
  showWms: boolean;
  onClose: () => void;
  onSaved: (company: AdminCompany) => void;
}) {
  const id = useId();
  const [form, setForm] = useState({
    name: company?.name ?? "",
    taxId: company?.taxId ?? "",
    email: company?.email ?? "",
    phone: company?.phone ?? "",
    wave: company ? (company.waveCode ?? GENERAL_WAVE_VALUE) : UNSET,
    wmsCode: company?.wmsCode ?? "",
  });
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({});
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const options = waveOptions(waves, company?.waveCode);
  const set = (change: Partial<typeof form>) => setForm((f) => ({ ...f, ...change }));

  const validate = () => {
    const e: Partial<Record<Field, string>> = {};
    if (!form.name.trim()) e.name = "El nombre es obligatorio.";
    if (!form.taxId.trim()) e.taxId = "El RUT es obligatorio.";
    else if (form.taxId.trim() !== company?.taxId && !normalizeRut(form.taxId)) e.taxId = "RUT inválido: revisa el número y el dígito verificador.";
    if (form.email.trim() && !EMAIL.test(form.email.trim())) e.email = "Correo inválido.";
    if (form.phone.trim() && (form.phone.trim().length < 6 || form.phone.trim().length > 30)) e.phone = "Teléfono inválido (entre 6 y 30 caracteres).";
    if (form.wave === UNSET) e.waveCode = "Elige la ola. Si no tiene una fija, elige «General (según reglas)».";
    return e;
  };

  const submit = async () => {
    setFormError(null);
    const e = validate();
    setErrors(e);
    if (Object.keys(e).length) return;
    const taxId = form.taxId.trim() === company?.taxId ? form.taxId.trim() : normalizeRut(form.taxId) ?? form.taxId.trim();
    const body: CompanyInput = {
      name: form.name.trim(), taxId, email: form.email.trim(), phone: form.phone.trim(),
      waveCode: form.wave === GENERAL_WAVE_VALUE ? null : form.wave,
      ...(showWms ? { wmsCode: form.wmsCode.trim() } : {}),
    };
    setBusy(true);
    try {
      onSaved(company ? await updateCompany(company.id, body) : await createCompany(body));
    } catch (err) {
      if (err instanceof ApiError && err.fields.length) {
        setErrors(Object.fromEntries(err.fields.map((f) => [f.field, f.message])));
      } else {
        setFormError(apiErrorMessage(err));
      }
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

  return (
    <FormModal title={company ? `Editar empresa ${company.name}` : "Nueva empresa"} onClose={onClose} onSubmit={submit}
      submitLabel={company ? "Guardar cambios" : "Crear empresa"} busy={busy} error={formError}>
      {field("name", "Nombre empresa *", <input {...aria("name")} maxLength={120} value={form.name} onChange={(e) => set({ name: e.target.value })} />)}
      {field("taxId", "RUT *", <input {...aria("taxId")} maxLength={20} placeholder="76.123.456-0" value={form.taxId}
        onChange={(e) => set({ taxId: e.target.value })} onBlur={() => { const r = normalizeRut(form.taxId); if (r) set({ taxId: r }); }} />,
        "Único: no puede repetirse entre empresas.")}
      {field("email", "Correo", <input {...aria("email")} type="email" maxLength={254} value={form.email} onChange={(e) => set({ email: e.target.value })} />)}
      {field("phone", "Teléfono", <input {...aria("phone")} type="tel" maxLength={30} placeholder="+56 9 1234 5678" value={form.phone} onChange={(e) => set({ phone: e.target.value })} />)}
      {field("waveCode", "Ola *", (
        <select {...aria("waveCode")} value={form.wave} onChange={(e) => set({ wave: e.target.value })}>
          <option value={UNSET} disabled>Elige la ola</option>
          {options.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
        </select>
      ), wavesError ? `No se pudieron cargar las olas de Invas (${wavesError}); puedes elegir «General (según reglas)».` : "«General (según reglas)»: sin ola fija; se usan las reglas de asignación.")}
      {showWms && field("wmsCode", "Código Invas", <input {...aria("wmsCode")} maxLength={40} value={form.wmsCode} onChange={(e) => set({ wmsCode: e.target.value })} />, "Opcional. Vínculo de la empresa con Invas.")}
    </FormModal>
  );
}
