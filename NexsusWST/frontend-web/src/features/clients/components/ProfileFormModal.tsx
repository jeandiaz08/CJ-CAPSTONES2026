"use client";

import { useId, useState, type ReactNode } from "react";
import { FormModal } from "@/components/FormModal";
import { ApiError, apiErrorMessage } from "@/services/api";
import { createProfile, updateProfile, type Profile, type ProfileInput, type ProfileScope } from "@/services/clients";

type Field = "name" | "companyId" | "email" | "username" | "phone" | "jobTitle";
const EMAIL = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;
const USERNAME = /^[a-z0-9][a-z0-9._-]{2,39}$/;
export type CompanyChoice = { id: number; name: string; active: boolean };

// «Nuevo perfil» y «Editar perfil». Lo obligatorio es lo que exige la base: nombre, empresa y correo
// o usuario. El rol queda Cliente fijo, sin permisos internos. Al editar, el correo y el usuario no
// cambian (son el acceso a Keycloak). `initial` completa el formulario (por ejemplo, «Editar y
// aceptar» una solicitud) y `onSaved` recibe el perfil creado o editado.
export function ProfileFormModal({ scope, profile, initial, companies, title, submitLabel, onClose, onSaved }: {
  scope: ProfileScope;
  profile?: Profile;
  initial?: Partial<ProfileInput>;
  companies: CompanyChoice[];
  title?: string;
  submitLabel?: string;
  onClose: () => void;
  onSaved: (profile: Profile) => void | Promise<void>;
}) {
  const id = useId();
  const [form, setForm] = useState({
    name: profile?.name ?? initial?.name ?? "",
    companyId: String(profile?.companyId ?? initial?.companyId ?? ""),
    jobTitle: profile?.jobTitle ?? initial?.jobTitle ?? "",
    phone: profile?.phone ?? initial?.phone ?? "",
    email: initial?.email ?? "",
    withoutEmail: !!initial && !initial.email,
    username: initial?.username ?? "",
  });
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({});
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const set = (change: Partial<typeof form>) => setForm((f) => ({ ...f, ...change }));
  // Solo empresas activas; al editar se conserva la actual aunque esté desactivada.
  const options = companies.filter((c) => c.active || c.id === profile?.companyId);

  const validate = () => {
    const e: Partial<Record<Field, string>> = {};
    if (!form.name.trim()) e.name = "El nombre es obligatorio.";
    if (!form.companyId) e.companyId = "Elige la empresa.";
    if (!profile) {
      if (form.withoutEmail) {
        if (!form.username.trim()) e.username = "Sin correo, el nombre de usuario es obligatorio.";
        else if (!USERNAME.test(form.username.trim().toLowerCase())) e.username = "De 3 a 40 caracteres: minúsculas, números, punto, guion o guion bajo; empieza con letra o número.";
      } else if (!form.email.trim()) e.email = "Indica el correo o marca «Usuario sin correo».";
      else if (!EMAIL.test(form.email.trim())) e.email = "Correo inválido.";
    }
    if (form.phone.trim() && (form.phone.trim().length < 6 || form.phone.trim().length > 30)) e.phone = "Teléfono inválido (entre 6 y 30 caracteres).";
    return e;
  };

  const submit = async () => {
    setFormError(null);
    const e = validate();
    setErrors(e);
    if (Object.keys(e).length) return;
    const base = { name: form.name.trim(), companyId: Number(form.companyId), phone: form.phone.trim(), jobTitle: form.jobTitle.trim() };
    setBusy(true);
    try {
      const saved = profile
        ? await updateProfile(scope, profile.id, base)
        : await createProfile(scope, { ...base, ...(form.withoutEmail ? { username: form.username.trim().toLowerCase() } : { email: form.email.trim() }) });
      await onSaved(saved);
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
    <FormModal title={title ?? (profile ? `Editar perfil ${profile.name}` : "Nuevo perfil")} onClose={onClose} onSubmit={submit}
      submitLabel={submitLabel ?? (profile ? "Guardar cambios" : "Crear perfil")} busy={busy} error={formError}>
      {field("name", "Nombre completo *", <input {...aria("name")} maxLength={120} value={form.name} onChange={(e) => set({ name: e.target.value })} />)}
      {field("companyId", "Empresa *", (
        <select {...aria("companyId")} value={form.companyId} onChange={(e) => set({ companyId: e.target.value })}>
          <option value="" disabled>Elige la empresa</option>
          {options.map((c) => <option key={c.id} value={c.id}>{c.name}{c.active ? "" : " (desactivada)"}</option>)}
        </select>
      ))}
      {field("jobTitle", "Cargo", <input {...aria("jobTitle")} maxLength={80} value={form.jobTitle} onChange={(e) => set({ jobTitle: e.target.value })} />)}
      {field("phone", "Teléfono", <input {...aria("phone")} type="tel" maxLength={30} placeholder="+56 9 1234 5678" value={form.phone} onChange={(e) => set({ phone: e.target.value })} />)}
      {profile ? (
        <p className="form-hint">{profile.hasEmail ? `Correo: ${profile.email}` : `Usuario: ${profile.username}`} · no se edita (es el acceso a NexusWST).</p>
      ) : (
        <>
          <label className="checkbox-row">
            <input type="checkbox" checked={form.withoutEmail} onChange={(e) => set({ withoutEmail: e.target.checked })} /> Usuario sin correo
          </label>
          {form.withoutEmail
            ? field("username", "Nombre de usuario *", <input {...aria("username")} maxLength={40} autoComplete="off" value={form.username} onChange={(e) => set({ username: e.target.value })} />,
              "La contraseña temporal se mostrará una sola vez al crear el perfil.")
            : field("email", "Correo *", <input {...aria("email")} type="email" maxLength={254} value={form.email} onChange={(e) => set({ email: e.target.value })} />,
              "Recibirá un enlace para definir su contraseña.")}
        </>
      )}
      <p className="form-hint">Rol: <strong>Cliente</strong> (sin permisos internos).</p>
    </FormModal>
  );
}
