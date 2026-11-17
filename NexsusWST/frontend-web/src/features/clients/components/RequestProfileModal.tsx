"use client";

import { useId, useState, type ReactNode } from "react";
import { FormModal } from "@/components/FormModal";
import { ApiError, apiErrorMessage } from "@/services/api";
import { createRequest, type ProfileRequest } from "@/services/clients";

type Field = "name" | "email" | "phone" | "jobTitle" | "reason";
const EMAIL = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;

// «Solicitar nuevo perfil» (Cliente, Tarea 07 Fase 5). Obligatorios: nombre, correo (el perfil Cliente
// siempre tiene correo) y motivo u observación. El perfil será Cliente, de la empresa del Cliente.
export function RequestProfileModal({ onClose, onSent }: { onClose: () => void; onSent: (r: ProfileRequest) => void }) {
  const id = useId();
  const [form, setForm] = useState({ name: "", jobTitle: "", email: "", phone: "", reason: "" });
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({});
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const set = (change: Partial<typeof form>) => setForm((f) => ({ ...f, ...change }));

  const submit = async () => {
    const e: Partial<Record<Field, string>> = {};
    if (!form.name.trim()) e.name = "El nombre es obligatorio.";
    if (!form.email.trim()) e.email = "El correo es obligatorio: el perfil recibe ahí el enlace para definir su contraseña.";
    else if (!EMAIL.test(form.email.trim())) e.email = "Correo inválido.";
    if (form.phone.trim() && (form.phone.trim().length < 6 || form.phone.trim().length > 30)) e.phone = "Teléfono inválido (entre 6 y 30 caracteres).";
    if (form.reason.trim().length < 5) e.reason = "Indica el motivo u observación (al menos 5 caracteres).";
    setErrors(e);
    setFormError(null);
    if (Object.keys(e).length) return;
    setBusy(true);
    try {
      onSent(await createRequest({ kind: "new_profile", name: form.name.trim(), email: form.email.trim(), phone: form.phone.trim(), jobTitle: form.jobTitle.trim(), reason: form.reason.trim() }));
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

  return (
    <FormModal title="Solicitar nuevo perfil" onClose={onClose} onSubmit={submit} submitLabel="Enviar solicitud" busy={busy} error={formError}>
      {field("name", "Nombre completo *", <input {...aria("name")} maxLength={120} value={form.name} onChange={(e) => set({ name: e.target.value })} />)}
      {field("jobTitle", "Cargo", <input {...aria("jobTitle")} maxLength={80} value={form.jobTitle} onChange={(e) => set({ jobTitle: e.target.value })} />)}
      {field("email", "Correo *", <input {...aria("email")} type="email" maxLength={254} value={form.email} onChange={(e) => set({ email: e.target.value })} />,
        "Si se aprueba, recibirá ahí el enlace para definir su contraseña.")}
      {field("phone", "Teléfono", <input {...aria("phone")} type="tel" maxLength={30} placeholder="+56 9 1234 5678" value={form.phone} onChange={(e) => set({ phone: e.target.value })} />)}
      {field("reason", "Motivo u observación *", <textarea {...aria("reason")} rows={3} maxLength={500} value={form.reason} onChange={(e) => set({ reason: e.target.value })} />)}
      <p className="form-hint">El perfil será <strong>Cliente</strong> de tu empresa, sin permisos internos. Lo revisa el Administrador.</p>
    </FormModal>
  );
}
