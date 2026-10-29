import { AlertCircle, CheckCircle2, Info } from "lucide-react";
import { kcSanitize } from "keycloakify/lib/kcSanitize";
import type { KcContext } from "../KcContext";

export default function FeedbackMessage({ message, id }: { message: KcContext["message"]; id?: string }) {
  if (!message?.summary) return null;

  const Icon = message.type === "success" ? CheckCircle2 : message.type === "error" ? AlertCircle : Info;

  return (
    <div
      id={id}
      className={`auth-feedback auth-feedback-${message.type}`}
      role={message.type === "error" ? "alert" : "status"}
      aria-live={message.type === "error" ? "assertive" : "polite"}
    >
      <Icon size={19} aria-hidden="true" />
      <span dangerouslySetInnerHTML={{ __html: kcSanitize(message.summary) }} />
    </div>
  );
}

export function FieldError({ id, message }: { id: string; message: string }) {
  if (!message) return null;

  return (
    <span
      id={id}
      className="auth-field-error"
      aria-live="polite"
    >
      <AlertCircle size={15} aria-hidden="true" />
      <span dangerouslySetInnerHTML={{ __html: kcSanitize(message) }} />
    </span>
  );
}
