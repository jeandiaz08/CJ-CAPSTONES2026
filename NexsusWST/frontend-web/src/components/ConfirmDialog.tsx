"use client";

import { useId, useState, type ReactNode } from "react";
import { ApiError, apiErrorMessage } from "@/services/api";
import { FormModal } from "./FormModal";

const MAX_REASON = 140;

// Confirmación con motivo opcional u obligatorio. Con `secondStep` pide una segunda
// confirmación antes de ejecutar. Si `onConfirm` falla, el error se muestra aquí.
export function ConfirmDialog({ title, status, children, reason = "none", reasonLabel = `Observaciones (${MAX_REASON} máx.)`, reasonRequiredMessage = "Indica el motivo.", confirmLabel, destructive = false, confirmDisabled = false, secondStep, onConfirm, onClose }: {
  title: string;
  status?: ReactNode;
  children?: ReactNode;
  reason?: "none" | "optional" | "required";
  reasonLabel?: string;
  reasonRequiredMessage?: string;
  confirmLabel: string;
  destructive?: boolean;
  confirmDisabled?: boolean;
  secondStep?: { message: ReactNode; confirmLabel: string };
  onConfirm: (reason: string) => Promise<void> | void;
  onClose: () => void;
}) {
  const fieldId = useId();
  const [text, setText] = useState("");
  const [step, setStep] = useState<1 | 2>(1);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const confirming = step === 2 && !!secondStep;

  const submit = async () => {
    setError(null);
    setFieldError(null);
    const value = text.trim();
    if (reason === "required" && !value) {
      setFieldError(reasonRequiredMessage);
      return;
    }
    if (secondStep && step === 1) {
      setStep(2);
      return;
    }
    setBusy(true);
    try {
      await onConfirm(value);
    } catch (e) {
      const reasonField = e instanceof ApiError ? e.fields.find((f) => f.field === "reason" || f.field === "note") : undefined;
      if (reasonField) {
        setFieldError(reasonField.message);
        setStep(1);
      } else {
        setError(apiErrorMessage(e));
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <FormModal
      title={title}
      status={status}
      onClose={onClose}
      onSubmit={submit}
      submitLabel={confirming ? secondStep.confirmLabel : confirmLabel}
      submitTone={destructive ? "danger" : "primary"}
      submitDisabled={confirmDisabled}
      busy={busy}
      error={error}
      closeLabel={confirming ? "Volver" : "Cerrar"}
      onSecondary={confirming ? () => { setStep(1); setError(null); } : onClose}
    >
      {confirming ? (
        <p className="confirm-warning">{secondStep.message}</p>
      ) : (
        <>
          {children}
          {reason !== "none" && (
            <div className="modal-field">
              <label htmlFor={fieldId}>{reasonLabel}{reason === "optional" ? " · opcional" : ""}</label>
              <textarea id={fieldId} rows={3} maxLength={MAX_REASON} value={text} aria-invalid={!!fieldError} aria-describedby={fieldError ? `${fieldId}-error` : undefined} onChange={(e) => setText(e.target.value)} />
              <div className="modal-field-meta">
                {fieldError ? <span id={`${fieldId}-error`} className="modal-field-error" role="alert">{fieldError}</span> : <span />}
                <span>{text.length}/{MAX_REASON}</span>
              </div>
            </div>
          )}
        </>
      )}
    </FormModal>
  );
}
