"use client";

import { useId, type ReactNode } from "react";

export type CardOption = {
  value: string;
  title: string;
  description?: string;
  danger?: boolean;
  disabled?: boolean;
  badge?: string;
  children?: ReactNode; // se muestra debajo de la tarjeta cuando está elegida
};

// Opciones excluyentes como tarjetas seleccionables. Son radios nativos: funcionan con teclado.
export function OptionCards({ legend, options, value, onChange, error }: {
  legend: string;
  options: CardOption[];
  value: string;
  onChange: (value: string) => void;
  error?: string;
}) {
  const name = useId();
  return (
    <fieldset className="option-cards">
      <legend>{legend}</legend>
      {options.map((option) => (
        <div key={option.value} className="option-card-wrap">
          <label className={`option-card ${value === option.value ? "is-selected" : ""} ${option.danger ? "is-danger" : ""} ${option.disabled ? "is-disabled" : ""}`}>
            <input type="radio" name={name} value={option.value} checked={value === option.value} disabled={option.disabled} onChange={() => onChange(option.value)} />
            <span className="option-card-text">
              <strong>{option.title}{option.badge && <small className="option-card-badge">{option.badge}</small>}</strong>
              {option.description && <span>{option.description}</span>}
            </span>
          </label>
          {value === option.value && option.children && <div className="option-card-extra">{option.children}</div>}
        </div>
      ))}
      {error && <span className="modal-field-error" role="alert">{error}</span>}
    </fieldset>
  );
}
