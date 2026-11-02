import { useState } from "react";
import { ArrowRight, UserRound } from "lucide-react";
import type { KcContext } from "../KcContext";
import type { I18n } from "../i18n";
import AuthLayout from "../components/AuthLayout";
import FeedbackMessage from "../components/FeedbackMessage";
import PasswordField from "../components/PasswordField";
import CheckEmail from "./CheckEmail";

type LoginContext = Extract<KcContext, { pageId: "login.ftl" }>;

export function isEmailSentMessage(kcContext: KcContext, i18n: I18n) {
  return kcContext.message?.type === "success"
    && kcContext.message.summary.trim() === i18n.msgStr("emailSentMessage").trim();
}

export default function Login({ kcContext, i18n }: { kcContext: LoginContext; i18n: I18n }) {
  const [submitting, setSubmitting] = useState(false);
  const { url, realm, login, auth, messagesPerField, usernameHidden } = kcContext;
  // Keycloak only includes enabled, visible providers in this list.
  const microsoftProvider = kcContext.social?.providers?.find(provider => provider.alias === "microsoft" && provider.loginUrl);
  const hasCredentialError = messagesPerField.existsError("username", "password");
  const credentialError = hasCredentialError ? messagesPerField.getFirstError("username", "password") : "";

  if (isEmailSentMessage(kcContext, i18n)) {
    return <CheckEmail kcContext={kcContext} />;
  }

  return (
    <AuthLayout title="Inicia sesión" notification={credentialError
      ? <FeedbackMessage id="credentials-error" message={{ type: "error", summary: credentialError }} />
      : undefined}>
      <header className="auth-card-header">
        <h1 id="auth-title" className="auth-title">Inicia sesión</h1>
      </header>
      {!hasCredentialError && !(kcContext.isAppInitiatedAction && kcContext.message?.type === "warning") && (
        <FeedbackMessage message={kcContext.message} />
      )}
      <>
        {microsoftProvider ? (
          <a className="auth-microsoft-button" href={microsoftProvider.loginUrl} id={`social-${microsoftProvider.alias}`}>
            <img className="auth-microsoft-logo" src={`${import.meta.env.BASE_URL}brand/Microsoft_logo.png`} alt="" aria-hidden="true" />
            Continuar con Microsoft
          </a>
        ) : (
          <button className="auth-microsoft-button" type="button" aria-disabled="true" disabled aria-describedby="microsoft-help">
            <img className="auth-microsoft-logo" src={`${import.meta.env.BASE_URL}brand/Microsoft_logo.png`} alt="" aria-hidden="true" />
            Continuar con Microsoft
          </button>
        )}
          <p className="auth-help" id="microsoft-help">Solo personal WeStorage: usa tu cuenta @westorage.cl</p>
          {realm.password && <div className="auth-divider"><span>O con tu correo o usuario</span></div>}
      </>
      {realm.password && (
        <form
          id="kc-form-login"
          className="auth-form"
          action={url.loginAction}
          method="post"
          aria-busy={submitting}
          onSubmit={event => { if (!event.defaultPrevented) setSubmitting(true); }}
        >
          {!usernameHidden && (
            <div className="auth-field">
              <label className="auth-label" htmlFor="username">Correo o usuario</label>
              <div className="auth-input-wrap">
                <UserRound className="auth-input-icon" size={18} aria-hidden="true" />
                <input
                  className="auth-input"
                  id="username"
                  name="username"
                  type="text"
                  defaultValue={login.username ?? ""}
                  autoComplete="username"
                  autoCapitalize="none"
                  spellCheck={false}
                  autoFocus
                  required
                  aria-invalid={hasCredentialError}
                  aria-describedby={hasCredentialError ? "credentials-error" : undefined}
                />
              </div>
            </div>
          )}
          <PasswordField
            id="password"
            name="password"
            label="Contraseña"
            autoComplete="current-password"
            autoFocus={usernameHidden}
            invalid={hasCredentialError}
            describedBy={hasCredentialError ? "credentials-error" : undefined}
          />
          <div className="auth-options">
            {realm.rememberMe && !usernameHidden && (
              <label className="auth-checkbox-label" htmlFor="rememberMe">
                <input id="rememberMe" name="rememberMe" type="checkbox" defaultChecked={!!login.rememberMe} />
                Mantener sesión iniciada
              </label>
            )}
            {realm.resetPasswordAllowed && <a className="auth-link" href={url.loginResetCredentialsUrl}>¿Olvidaste tu contraseña?</a>}
          </div>
          <input type="hidden" name="credentialId" value={auth.selectedCredential ?? ""} />
          <button className="auth-primary-button" id="kc-login" name="login" type="submit" disabled={submitting}>
            {submitting ? "Ingresando…" : "Ingresar"}<ArrowRight size={18} aria-hidden="true" />
          </button>
        </form>
      )}
      <p className="auth-help">¿No tienes acceso a NexusWST? Contacta a un administrador para activar tu cuenta.</p>
    </AuthLayout>
  );
}
