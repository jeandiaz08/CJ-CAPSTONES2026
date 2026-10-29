import { useState, type ReactNode } from "react";
import { Eye, EyeOff, LockKeyhole } from "lucide-react";
import { FieldError } from "./FeedbackMessage";

type PasswordFieldProps = {
  id: string;
  name: string;
  label: string;
  autoComplete: "current-password" | "new-password";
  error?: string;
  invalid?: boolean;
  describedBy?: string;
  autoFocus?: boolean;
  onValueChange?: (value: string) => void;
  children?: ReactNode;
  showToggleText?: boolean;
};

export default function PasswordField({
  id, name, label, autoComplete, error, invalid, describedBy, autoFocus,
  onValueChange, children, showToggleText = false,
}: PasswordFieldProps) {
  const [revealed, setRevealed] = useState(false);
  const errorId = `${id}-error`;
  const description = [describedBy, error ? errorId : undefined].filter(Boolean).join(" ") || undefined;

  return (
    <div className="auth-field">
      <label className="auth-label" htmlFor={id}>{label}</label>
      <div className="auth-input-wrap">
        <LockKeyhole className="auth-input-icon" size={18} aria-hidden="true" />
        <input
          className="auth-input"
          id={id}
          name={name}
          type={revealed ? "text" : "password"}
          autoComplete={autoComplete}
          autoFocus={autoFocus}
          required
          onChange={event => onValueChange?.(event.currentTarget.value)}
          aria-invalid={invalid || !!error}
          aria-describedby={description}
        />
        <button
          className="auth-password-toggle"
          type="button"
          aria-label={revealed ? "Ocultar contraseña" : "Mostrar contraseña"}
          aria-pressed={revealed}
          aria-controls={id}
          onClick={() => setRevealed(!revealed)}
        >
          {revealed ? <EyeOff size={19} aria-hidden="true" /> : <Eye size={19} aria-hidden="true" />}
          {showToggleText && <span>{revealed ? "Ocultar" : "Mostrar"}</span>}
        </button>
      </div>
      {error && <FieldError id={errorId} message={error} />}
      {children}
    </div>
  );
}
