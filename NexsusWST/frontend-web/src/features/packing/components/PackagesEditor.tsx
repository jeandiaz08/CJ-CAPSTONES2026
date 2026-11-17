"use client";

import { Minus, Plus } from "lucide-react";
import type { Package, PackageInput } from "@/services/orders";

const MAX_PACKAGES = 50; // el servidor rechaza más que esto
const FIELDS: { key: keyof PackageInput; label: string }[] = [
  { key: "weightKg", label: "Peso (kg)" },
  { key: "lengthCm", label: "Largo (cm)" },
  { key: "widthCm", label: "Ancho (cm)" },
  { key: "heightCm", label: "Alto (cm)" },
];

const toInput = (p: Package): PackageInput => ({ weightKg: p.weightKg, lengthCm: p.lengthCm, widthCm: p.widthCm, heightCm: p.heightCm });

// Cantidad de bultos, con peso y medidas opcionales por bulto.
export function PackagesEditor({ packages, busy, disabled, onSave }: {
  packages: Package[];
  busy: boolean;
  disabled: boolean;
  onSave: (packages: PackageInput[]) => void;
}) {
  const current = packages.map(toInput);
  const saveField = (index: number, key: keyof PackageInput, text: string) => {
    const value = text === "" ? undefined : Number(text);
    if (value === current[index][key]) return;
    onSave(current.map((p, i) => (i === index ? { ...p, [key]: value } : p)));
  };
  const locked = disabled || busy;
  return (
    <div className="packing-packages">
      <div className="packing-stepper">
        <span id="packing-packages-label">Cantidad de bultos:</span>
        <button type="button" className="packing-step" disabled={locked || packages.length <= 1} aria-label="Quitar un bulto" onClick={() => onSave(current.slice(0, -1))}><Minus size={20} aria-hidden="true" /></button>
        <output aria-labelledby="packing-packages-label">{packages.length}</output>
        <button type="button" className="packing-step" disabled={locked || packages.length >= MAX_PACKAGES} aria-label="Agregar un bulto" onClick={() => onSave([...current, {}])}><Plus size={20} aria-hidden="true" /></button>
      </div>
      {packages.length > 0 && (
        <details className="packing-package-details">
          <summary>Peso y medidas (opcional)</summary>
          {packages.map((p, index) => (
            <fieldset key={p.code} disabled={locked}>
              <legend>{p.code}</legend>
              {FIELDS.map(({ key, label }) => (
                <label key={key}>{label}
                  <input key={`${p.code}-${p[key] ?? ""}`} type="number" min="0" step="any" inputMode="decimal" defaultValue={p[key] ?? ""} onBlur={(e) => saveField(index, key, e.target.value)} />
                </label>
              ))}
            </fieldset>
          ))}
        </details>
      )}
    </div>
  );
}
