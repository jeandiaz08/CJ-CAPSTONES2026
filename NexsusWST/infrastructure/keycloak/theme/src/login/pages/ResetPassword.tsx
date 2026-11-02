import { useState } from "react";
import { ArrowLeft, ArrowRight, UserRound } from "lucide-react";
import type { KcContext } from "../KcContext";
import AuthLayout from "../components/AuthLayout";
import FeedbackMessage from "../components/FeedbackMessage";

export default function ResetPassword({ kcContext }: { kcContext: Extract<KcContext, { pageId: "login-reset-password.ftl" }> }) {
  const [submitting, setSubmitting] = useState(false);
  const { url, auth, messagesPerField } = kcContext;
  const hasError = messagesPerField.existsError("username");
  const prefilledUsername = auth.attemptedUsername || new URLSearchParams(window.location.search).get("login_hint") || "";

  return (
    <AuthLayout title="Recupera tu contraseña" notification={hasError
      ? <FeedbackMessage id="username-error" message={{ type: "error", summary: messagesPerField.get("username") }} />
      : undefined}>
      <a className="auth-back-link" href={url.loginUrl}><ArrowLeft size={16} aria-hidden="true" />Volver al inicio de sesión</a>
      <header className="auth-card-header">
        <h1 id="auth-title" className="auth-title">¿Olvidaste tu contraseña?</h1>
        <p className="auth-description">Ingresa tu correo o usuario. Si tu cuenta tiene un correo asociado, recibirás un enlace para crear una nueva contraseña.</p>
      </header>
      {!hasError && <FeedbackMessage message={kcContext.message} />}
      <form
        id="kc-reset-password-form"
        className="auth-form"
        action={url.loginAction}
        method="post"
        aria-busy={submitting}
        onSubmit={event => { if (!event.defaultPrevented) setSubmitting(true); }}
      >
        <div className="auth-field">
          <label className="auth-label" htmlFor="username">Correo o usuario</label>
          <div className="auth-input-wrap">
            <UserRound className="auth-input-icon" size={18} aria-hidden="true" />
            <input
              className="auth-input"
              type="text"
              id="username"
              name="username"
              defaultValue={prefilledUsername}
              autoComplete="username"
              autoCapitalize="none"
              spellCheck={false}
              autoFocus
              required
              aria-invalid={hasError}
              aria-describedby={hasError ? "username-error" : undefined}
            />
          </div>
        </div>
        <button className="auth-primary-button" type="submit" disabled={submitting}>
          {submitting ? "Enviando…" : "Enviar enlace"}<ArrowRight size={18} aria-hidden="true" />
        </button>
      </form>
      <p className="auth-help">Si usas Microsoft para ingresar, recupera tu acceso desde tu cuenta de Microsoft.</p>
    </AuthLayout>
  );
}
