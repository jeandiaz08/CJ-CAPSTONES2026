"use client";

import type { FormEvent, ReactNode } from "react";
import { LoaderCircle } from "lucide-react";
import { Modal } from "./Modal";

export type SubmitTone = "primary" | "danger" | "success" | "highlight";

// Modal de formulario: encabezado, cuerpo con scroll y pie fijo.
// Mientras se envía no se puede cerrar ni volver a enviar.
export function FormModal({ title, status, onClose, onSubmit, submitLabel, submitTone = "primary", submitDisabled = false, busy = false, error, closeLabel = "Cerrar", onSecondary, wide = false, submitFirst = false, children }: {
  title: string;
  status?: ReactNode;
  onClose: () => void;
  onSubmit?: () => void;
  submitLabel?: string;
  submitTone?: SubmitTone;
  submitDisabled?: boolean;
  busy?: boolean;
  error?: string | null;
  closeLabel?: string;
  onSecondary?: () => void;
  wide?: boolean;
  submitFirst?: boolean;
  children: ReactNode;
}) {
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!busy && !submitDisabled) onSubmit?.();
  };
  const secondary = <button type="button" className="btn ghost" disabled={busy} onClick={onSecondary ?? onClose}>{closeLabel}</button>;
  return (
    <Modal title={title} aside={status} onClose={() => { if (!busy) onClose(); }} panelClassName={`form-modal ${wide ? "form-modal-wide" : ""}`}>
      <form className="form-modal-form" onSubmit={submit} noValidate>
        <div className="form-modal-body">{children}</div>
        {error && <p className="form-modal-error" role="alert">{error}</p>}
        <div className="form-modal-foot">
          {!submitFirst && secondary}
          {onSubmit && (
            <button type="submit" className={`btn form-submit-${submitTone}`} disabled={busy || submitDisabled} aria-busy={busy}>
              {busy && <LoaderCircle size={18} className="spin" aria-hidden="true" />}
              {submitLabel}
            </button>
          )}
          {submitFirst && secondary}
        </div>
      </form>
    </Modal>
  );
}
