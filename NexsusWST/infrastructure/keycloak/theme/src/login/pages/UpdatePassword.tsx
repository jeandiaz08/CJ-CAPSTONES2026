import { useState } from "react";
import { ArrowRight } from "lucide-react";
import type { KcContext } from "../KcContext";
import type { I18n } from "../i18n";
import AuthLayout from "../components/AuthLayout";
import FeedbackMessage from "../components/FeedbackMessage";
import PasswordField from "../components/PasswordField";
import PasswordRequirements from "../components/PasswordRequirements";

export default function UpdatePassword({ kcContext, i18n }: {
  kcContext: Extract<KcContext, { pageId: "login-update-password.ftl" }>;
  i18n: I18n;
}) {
  const [submitting, setSubmitting] = useState(false);
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const { url, messagesPerField, isAppInitiatedAction } = kcContext;
  const hasError = messagesPerField.existsError("password", "password-confirm");
  const username = kcContext.auth?.attemptedUsername || kcContext.username || kcContext.login?.username || kcContext.user?.username;
  const identity = username || kcContext.user?.email;
  const nameParts = identity?.split("@")[0].split(/[\s._-]+/).filter(Boolean) ?? [];
  const initials = (nameParts.length > 1 ? nameParts.slice(0, 2).map(part => part[0]).join("") : nameParts[0]?.slice(0, 2))?.toLocaleUpperCase("es-CL");
  const mismatch = confirmation.length > 0 && password !== confirmation;

  return (
    <AuthLayout title="Crea tu contraseña">
      <header className="auth-card-header">
        <h1 id="auth-title" className="auth-title">Crea tu contraseña</h1>
      </header>
      {!hasError && !(isAppInitiatedAction && kcContext.message?.type === "warning") && <FeedbackMessage message={kcContext.message} />}
      {identity && <div className="auth-identity-chip"><span className="auth-avatar" aria-hidden="true">{initials}</span><span>{identity}</span></div>}
      <form
        id="kc-passwd-update-form"
        className="auth-form"
        action={url.loginAction}
        method="post"
        aria-busy={submitting}
        onSubmit={event => { if (!event.defaultPrevented) setSubmitting(true); }}
      >
        <PasswordField
          id="password-new"
          name="password-new"
          label="Nueva contraseña"
          autoComplete="new-password"
          autoFocus
          invalid={hasError}
          error={messagesPerField.existsError("password") ? messagesPerField.get("password") : undefined}
          onValueChange={setPassword}
          showToggleText
          describedBy={kcContext.passwordPolicies ? "password-requirements" : undefined}
        >
          <PasswordRequirements policy={kcContext.passwordPolicies} password={password} username={username} email={kcContext.user?.email} />
        </PasswordField>
        <PasswordField
          id="password-confirm"
          name="password-confirm"
          label="Repite la contraseña"
          autoComplete="new-password"
          invalid={hasError || mismatch}
          error={mismatch ? "Las contraseñas no coinciden." : messagesPerField.existsError("password-confirm") ? messagesPerField.get("password-confirm") : undefined}
          onValueChange={setConfirmation}
          showToggleText
        />
        <label className="auth-checkbox-label" htmlFor="logout-sessions">
          <input type="checkbox" id="logout-sessions" name="logout-sessions" value="on" />
          {i18n.msgStr("logoutOtherSessions")}
        </label>
        <button className="auth-primary-button" type="submit" disabled={submitting}>
          {submitting ? "Guardando…" : "Guardar e ingresar"}<ArrowRight size={18} aria-hidden="true" />
        </button>
        {isAppInitiatedAction && (
          <button className="auth-secondary-button" type="submit" name="cancel-aia" value="true" formNoValidate disabled={submitting}>
            Cancelar
          </button>
        )}
      </form>
    </AuthLayout>
  );
}
