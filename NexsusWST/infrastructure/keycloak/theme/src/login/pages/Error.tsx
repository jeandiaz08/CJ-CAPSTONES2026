import { AlertCircle } from "lucide-react";
import { kcSanitize } from "keycloakify/lib/kcSanitize";
import type { KcContext } from "../KcContext";
import type { I18n } from "../i18n";
import AuthLayout from "../components/AuthLayout";

export default function Error({ kcContext, i18n }: {
  kcContext: Extract<KcContext, { pageId: "error.ftl" }>;
  i18n: I18n;
}) {
  const { message, client, skipLink } = kcContext;

  return (
    <AuthLayout title="No pudimos continuar" centered>
      <div className="auth-status-icon auth-status-icon-error"><AlertCircle size={28} aria-hidden="true" /></div>
      <header className="auth-card-header">
        <h1 id="auth-title" className="auth-title">No pudimos continuar</h1>
        <p className="auth-description" role="alert" aria-live="assertive" dangerouslySetInnerHTML={{ __html: kcSanitize(message.summary) }} />
      </header>
      {!skipLink && client?.baseUrl && (
        <div className="auth-card-actions"><a className="auth-primary-button" href={client.baseUrl}>{i18n.msgStr("backToApplication")}</a></div>
      )}
    </AuthLayout>
  );
}
