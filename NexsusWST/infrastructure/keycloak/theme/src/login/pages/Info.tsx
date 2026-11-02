import { CheckCircle2, Info as InfoIcon, Mail } from "lucide-react";
import { kcSanitize } from "keycloakify/lib/kcSanitize";
import type { KcContext } from "../KcContext";
import type { I18n } from "../i18n";
import AuthLayout from "../components/AuthLayout";
import { isEmailSentMessage } from "./Login";
import CheckEmail from "./CheckEmail";

export default function Info({ kcContext, i18n }: {
  kcContext: Extract<KcContext, { pageId: "info.ftl" }>;
  i18n: I18n;
}) {
  const { messageHeader, message, requiredActions, skipLink, pageRedirectUri, actionUri, client } = kcContext;
  const emailSent = isEmailSentMessage(kcContext, i18n);
  if (emailSent) return <CheckEmail kcContext={kcContext} />;
  const title = emailSent ? "Revisa tu correo" : messageHeader ? i18n.advancedMsgStr(messageHeader) : "Información de tu cuenta";
  const Icon = emailSent ? Mail : message.type === "success" ? CheckCircle2 : InfoIcon;
  const href = skipLink ? undefined : pageRedirectUri || actionUri || client.baseUrl;
  const linkText = !pageRedirectUri && actionUri ? i18n.msgStr("proceedWithAction") : i18n.msgStr("backToApplication");

  return (
    <AuthLayout title={title} centered>
      <div className={`auth-status-icon auth-status-icon-${emailSent ? "mail" : message.type === "error" ? "error" : message.type === "success" ? "success" : "info"}`}>
        <Icon size={28} aria-hidden="true" />
      </div>
      <header className="auth-card-header">
        <h1 id="auth-title" className="auth-title" dangerouslySetInnerHTML={{ __html: kcSanitize(title) }} />
        <p
          className="auth-description"
          role={message.type === "error" ? "alert" : "status"}
          aria-live={message.type === "error" ? "assertive" : "polite"}
          dangerouslySetInnerHTML={{ __html: kcSanitize(message.summary) }}
        />
      </header>
      {requiredActions && requiredActions.length > 0 && (
        <ul className="auth-required-actions">
          {requiredActions.map(action => <li key={action}>{i18n.advancedMsg(`requiredAction.${action}`)}</li>)}
        </ul>
      )}
      {href && <div className="auth-card-actions"><a className="auth-primary-button" href={href}>{linkText}</a></div>}
    </AuthLayout>
  );
}
