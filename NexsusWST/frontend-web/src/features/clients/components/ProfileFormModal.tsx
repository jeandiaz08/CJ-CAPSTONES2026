"use client";

import { useId, useState, type ReactNode } from "react";
import { FormModal } from "@/components/FormModal";
import { ApiError, apiErrorMessage } from "@/services/api";
import { createProfile, updateProfile, type Profile, type ProfileInput, type ProfileScope } from "@/services/clients";
import { INTERNAL_ROLES, PERMISSION_LABELS, PROFILE_ROLE_LABELS, ROLE_PERMISSIONS } from "../access";

type Field = "name" | "companyId" | "email" | "username" | "phone" | "jobTitle" | "role" | "permissions";
const EMAIL = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;
const USERNAME = /^[a-z0-9][a-z0-9._-]{2,39}$/;
export type CompanyChoice = { id: number; name: string; active: boolean };

// «Nuevo perfil» y «Editar perfil».
// - Cliente (scope «client»): obligatorios nombre, empresa y correo (recibe ahí el enlace para definir
//   su contraseña). El rol queda Cliente fijo, sin permisos internos.
// - WeStorage (scope «internal», Tarea 08): sin empresa; rol (Administrador, Jefe de Operaciones o
//   Empaque) y los permisos que ese rol admite. «Usuario sin correo» (SCRUM-231): el nombre de usuario
//   es obligatorio y al crear se muestra una sola vez la contraseña temporal.
// Al editar, el correo y el usuario no cambian (son el acceso a Keycloak). `initial` completa el
// formulario (por ejemplo, «Editar y aceptar» una solicitud) y `onSaved` recibe el perfil creado o
// editado. Con `submitData` el formulario no crea el perfil: entrega los datos y `lockCompany` deja
// la empresa fija. `self`: el perfil es el de quien edita (no puede cambiar su rol ni sus permisos).
export function ProfileFormModal({ scope, profile, initial, companies, title, submitLabel, lockCompany = false, self = false, submitData, onClose, onSaved }: {
  scope: ProfileScope;
  profile?: Profile;
  initial?: Partial<ProfileInput>;
  companies: CompanyChoice[];
  title?: string;
  submitLabel?: string;
  lockCompany?: boolean;
  self?: boolean;
  submitData?: (body: ProfileInput) => Promise<void>;
  onClose: () => void;
  onSaved?: (profile: Profile) => void | Promise<void>;
}) {
  const id = useId();
  const internal = scope === "internal";
  const [form, setForm] = useState({
    name: profile?.name ?? initial?.name ?? "",
    companyId: String(profile?.companyId ?? initial?.companyId ?? ""),
    jobTitle: profile?.jobTitle ?? initial?.jobTitle ?? "",
    phone: profile?.phone ?? initial?.phone ?? "",
    email: initial?.email ?? "",
    username: initial?.username ?? "",
    withoutEmail: false,
    role: profile?.role ?? initial?.role ?? "",
    permissions: profile?.permissions ?? initial?.permissions ?? [],
  });
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({});
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const set = (change: Partial<typeof form>) => setForm((f) => ({ ...f, ...change }));
  // Solo empresas activas; al editar se conserva la actual aunque esté desactivada.
  const options = companies.filter((c) => c.active || c.id === profile?.companyId || (lockCompany && c.id === initial?.companyId));
  const allowedPerms = ROLE_PERMISSIONS[form.role] ?? [];
  // Al cambiar de rol se quitan los permisos que el nuevo rol no admite.
  const setRole = (role: string) => set({ role, permissions: form.permissions.filter((p) => (ROLE_PERMISSIONS[role] ?? []).includes(p)) });
  const togglePerm = (perm: string) => set({ permissions: form.permissions.includes(perm) ? form.permissions.filter((p) => p !== perm) : [...form.permissions, perm] });

  const validate = () => {
    const e: Partial<Record<Field, string>> = {};
    if (!form.name.trim()) e.name = "El nombre es obligatorio.";
    if (!internal && !form.companyId) e.companyId = "Elige la empresa.";
    if (internal && !form.role) e.role = "Elige el rol.";
    if (!profile) {
      if (internal && form.withoutEmail) {
        if (!form.username.trim()) e.username = "Sin correo, el nombre de usuario es obligatorio.";
        else if (!USERNAME.test(form.username.trim().toLowerCase())) e.username = "De 3 a 40 caracteres: minúsculas, números, punto, guion o guion bajo; empieza con letra o número.";
      } else if (!form.email.trim()) e.email = internal ? "Indica el correo o marca «Usuario sin correo»." : "El correo es obligatorio: el perfil recibe ahí el enlace para definir su contraseña.";
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
    const common = { name: form.name.trim(), phone: form.phone.trim(), jobTitle: form.jobTitle.trim() };
    const base = internal
      ? { ...common, role: form.role, permissions: form.permissions.filter((p) => allowedPerms.includes(p)) }
      : { ...common, companyId: Number(form.companyId) };
    const login = internal && form.withoutEmail ? { username: form.username.trim().toLowerCase() } : { email: form.email.trim() };
    setBusy(true);
    try {
      if (submitData) {
        await submitData({ ...base, ...login });
        return;
      }
      const saved = profile ? await updateProfile(scope, profile.id, base) : await createProfile(scope, { ...base, ...login });
      await onSaved?.(saved);
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
  const loginText = profile ? (profile.hasEmail ? `Correo: ${profile.email || "—"}` : `Usuario: ${profile.username || "—"}`) : "";

  return (
    <FormModal title={title ?? (profile ? `Editar perfil ${profile.name}` : "Nuevo perfil")} onClose={onClose} onSubmit={submit}
      submitLabel={submitLabel ?? (profile ? "Guardar cambios" : "Crear perfil")} busy={busy} error={formError}>
      {field("name", "Nombre completo *", <input {...aria("name")} maxLength={120} value={form.name} onChange={(e) => set({ name: e.target.value })} />)}
      {!internal && field("companyId", "Empresa *", (
        <select {...aria("companyId")} value={form.companyId} disabled={lockCompany} onChange={(e) => set({ companyId: e.target.value })}>
          <option value="" disabled>Elige la empresa</option>
          {options.map((c) => <option key={c.id} value={c.id}>{c.name}{c.active ? "" : " (desactivada)"}</option>)}
        </select>
      ))}
      {field("jobTitle", "Cargo", <input {...aria("jobTitle")} maxLength={80} value={form.jobTitle} onChange={(e) => set({ jobTitle: e.target.value })} />)}
      {field("phone", "Teléfono", <input {...aria("phone")} type="tel" maxLength={30} placeholder="+56 9 1234 5678" value={form.phone} onChange={(e) => set({ phone: e.target.value })} />)}
      {profile ? (
        <p className="form-hint">{loginText} · no se edita (es el acceso a NexusWST).</p>
      ) : (
        <>
          {internal && (
            <label className="checkbox-row">
              <input type="checkbox" checked={form.withoutEmail} onChange={(e) => set({ withoutEmail: e.target.checked })} /> Usuario sin correo
            </label>
          )}
          {internal && form.withoutEmail
            ? field("username", "Nombre de usuario *", <input {...aria("username")} maxLength={40} autoComplete="off" value={form.username} onChange={(e) => set({ username: e.target.value })} />,
              "La contraseña temporal se mostrará una sola vez al crear el perfil.")
            : field("email", "Correo *", <input {...aria("email")} type="email" maxLength={254} value={form.email} onChange={(e) => set({ email: e.target.value })} />,
              "Recibirá un enlace para definir su contraseña.")}
        </>
      )}
      {internal ? (
        <>
          {field("role", "Rol *", (
            <select {...aria("role")} value={form.role} disabled={self} onChange={(e) => setRole(e.target.value)}>
              <option value="" disabled>Elige el rol</option>
              {INTERNAL_ROLES.map((r) => <option key={r} value={r}>{PROFILE_ROLE_LABELS[r]}</option>)}
            </select>
          ), self ? "No puedes cambiar tu propio rol ni tus permisos." : undefined)}
          <fieldset className="modal-field" aria-describedby={errors.permissions ? `${id}-permissions-error` : undefined}>
            <legend>Permisos</legend>
            {allowedPerms.length === 0
              ? <span className="form-hint">{form.role ? "Este rol no tiene permisos adicionales." : "Elige primero el rol."}</span>
              : allowedPerms.map((p) => (
                <label key={p} className="checkbox-row">
                  <input type="checkbox" checked={form.permissions.includes(p)} disabled={self} onChange={() => togglePerm(p)} /> {PERMISSION_LABELS[p] ?? p}
                </label>
              ))}
            {errors.permissions && <span id={`${id}-permissions-error`} className="modal-field-error" role="alert">{errors.permissions}</span>}
          </fieldset>
          {/* TODO(PV-21): campo «Bodegas» cuando exista la tabla de bodegas del usuario. */}
        </>
      ) : (
        <p className="form-hint">Rol: <strong>Cliente</strong> (sin permisos internos).</p>
      )}
    </FormModal>
  );
}
