import { Check, Circle } from "lucide-react";
import type { PasswordPolicies } from "keycloakify/login/KcContext";

type Rule = { key: string; label: string; met: boolean | null };

export default function PasswordRequirements({ policy, password, username, email }: {
  policy?: PasswordPolicies;
  password: string;
  username?: string;
  email?: string;
}) {
  if (!policy) return null;
  const rules: Rule[] = [];
  const hasValue = password.length > 0;
  const add = (key: string, label: string, met: boolean | null) => rules.push({ key, label, met: met === null ? null : hasValue && met });

  if (policy.length) add("length", `Al menos ${policy.length} caracteres`, password.length >= policy.length);
  if (policy.maxLength) add("maxLength", `Como máximo ${policy.maxLength} caracteres`, password.length <= policy.maxLength);

  const composition: string[] = [];
  const compositionChecks: boolean[] = [];
  if (policy.upperCase) {
    composition.push(policy.upperCase === 1 ? "Una mayúscula" : `${policy.upperCase} mayúsculas`);
    compositionChecks.push((password.match(/\p{Lu}/gu)?.length ?? 0) >= policy.upperCase);
  }
  if (policy.digits) {
    composition.push(policy.digits === 1 ? "un número" : `${policy.digits} números`);
    compositionChecks.push((password.match(/\p{Nd}/gu)?.length ?? 0) >= policy.digits);
  }
  if (composition.length) add("composition", composition.join(" y ").replace(/^un número/, "Un número"), compositionChecks.every(Boolean));
  if (policy.lowerCase) add("lowerCase", `Al menos ${policy.lowerCase} minúsculas`, (password.match(/\p{Ll}/gu)?.length ?? 0) >= policy.lowerCase);
  if (policy.specialChars) add("specialChars", `Al menos ${policy.specialChars} caracteres especiales`, (password.match(/[^\p{L}\p{N}]/gu)?.length ?? 0) >= policy.specialChars);
  if (policy.notUsername) add("notUsername", "No debe ser igual a tu usuario", username ? password !== username : null);
  if (policy.notEmail) add("notEmail", "No debe ser igual a tu correo", email ? password !== email : null);

  if (rules.length === 0) return null;
  const evaluable = rules.filter(rule => rule.met !== null);
  const passed = evaluable.filter(rule => rule.met).length;
  const progress = evaluable.length ? Math.round(passed / evaluable.length * 100) : 0;

  return (
    <div className="auth-password-requirements" id="password-requirements">
      <div className="auth-strength-track" role="progressbar" aria-label="Fortaleza según los requisitos de contraseña" aria-valuemin={0} aria-valuemax={100} aria-valuenow={progress} aria-valuetext={`${passed} de ${evaluable.length} requisitos comprobables cumplidos`}>
        <span className="auth-strength-fill" style={{ width: `${progress}%` }} />
      </div>
      <ul className="auth-password-rules" aria-live="polite" aria-atomic="true">
        {rules.map(rule => (
          <li key={rule.key} className={rule.met ? "auth-rule-met" : undefined} data-policy={rule.key}>
            {rule.met ? <Check size={16} aria-hidden="true" /> : <Circle size={13} aria-hidden="true" />}
            <span>{rule.label}{rule.met === null && <small>Keycloak lo comprueba al guardar.</small>}</span>
            <span className="auth-sr-only">{rule.met ? ": cumplido" : ": pendiente"}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
