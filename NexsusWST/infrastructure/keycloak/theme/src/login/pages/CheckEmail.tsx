import { useEffect, useState } from "react";
import { Mail } from "lucide-react";
import { kcEnvDefaults } from "../../kc.gen";
import type { KcContext } from "../KcContext";
import AuthLayout from "../components/AuthLayout";

export default function CheckEmail({ kcContext }: { kcContext: KcContext }) {
  const entered = kcContext.auth?.attemptedUsername
    || ("login" in kcContext ? kcContext.login?.username : undefined);
  const sender = kcContext.properties.NEXUS_MAIL_FROM || kcEnvDefaults.NEXUS_MAIL_FROM;
  // Preview flags cannot shorten the cooldown in a built theme.
  const initialSeconds = import.meta.env.DEV && kcContext["x-nexus-preview"]?.resendReady ? 0 : 60;
  const [seconds, setSeconds] = useState(initialSeconds);

  useEffect(() => {
    const deadline = Date.now() + initialSeconds * 1000;
    const timer = setInterval(() => {
      const remaining = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
      setSeconds(remaining);
      if (remaining === 0) clearInterval(timer);
    }, 1000);
    return () => clearInterval(timer);
  }, [initialSeconds]);

  const nativeReset = "loginResetCredentialsUrl" in kcContext.url
    && typeof kcContext.url.loginResetCredentialsUrl === "string"
    ? kcContext.url.loginResetCredentialsUrl : kcContext.url.loginUrl;
  const recoveryUrl = new URL(nativeReset, window.location.href);
  if (entered) recoveryUrl.searchParams.set("login_hint", entered);
  const announcement = seconds > 30 ? "Podrás reenviar el correo en un minuto."
    : seconds > 10 ? "Podrás reenviar el correo en treinta segundos o menos."
    : seconds > 0 ? "Podrás reenviar el correo en diez segundos o menos."
    : "Ya puedes reenviar el correo.";

  return (
    <AuthLayout title="Revisa tu correo" centered>
      <div className="auth-status-icon auth-status-icon-mail"><Mail size={28} aria-hidden="true" /></div>
      <header className="auth-card-header">
        <h1 id="auth-title" className="auth-title">Revisa tu correo</h1>
        <p className="auth-description" role="status">
          {entered ? <>Si el correo/usuario <strong>{entered}</strong> tiene una cuenta vinculada, recibirás </>
            : <>Si existe una cuenta vinculada, recibirás </>}
          un correo de <strong>{sender}</strong> con el enlace.
        </p>
      </header>
      <p className="auth-help">¿No llegó? Revisa tu carpeta de spam o</p>
      <div className="auth-card-actions">
        <button type="button" className="auth-secondary-button auth-resend-button"
          disabled={seconds > 0} aria-disabled={seconds > 0}
          aria-label={seconds > 0 ? "Reenviar correo, espera a que termine la cuenta regresiva" : "Reenviar correo"}
          onClick={() => window.location.assign(recoveryUrl.href)}>
          <span aria-hidden="true">Reenviar correo{seconds > 0 ? ` (${seconds} s)` : ""}</span>
        </button>
        <span className="auth-sr-only" role="status" aria-live="polite" aria-atomic="true">{announcement}</span>
        <a className="auth-link" href={kcContext.url.loginUrl}>Volver al inicio de sesión</a>
      </div>
    </AuthLayout>
  );
}
