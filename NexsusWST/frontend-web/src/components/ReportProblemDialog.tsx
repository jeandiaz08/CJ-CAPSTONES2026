"use client";

import { useId, useState } from "react";
import { ConfirmDialog } from "./ConfirmDialog";

export type ProblemOption = { value: string; label: string };

// Informar un problema: tipo, opcionalmente el elemento afectado, y motivo obligatorio.
export function ReportProblemDialog({ title, kinds, targets, defaultTarget = "", onConfirm, onClose }: {
  title: string;
  kinds: ProblemOption[];
  targets?: ProblemOption[]; // por ejemplo, bultos y productos de la orden
  defaultTarget?: string;
  onConfirm: (kind: string, reason: string, target?: ProblemOption) => Promise<void> | void;
  onClose: () => void;
}) {
  const id = useId();
  const [kind, setKind] = useState(kinds[0].value);
  const [target, setTarget] = useState(defaultTarget);
  return (
    <ConfirmDialog title={title} reason="required" reasonLabel="Motivo" confirmLabel="Informar" onClose={onClose}
      onConfirm={(reason) => onConfirm(kind, reason, targets?.find((t) => t.value === target && t.value !== ""))}>
      <div className="modal-field">
        <label htmlFor={`${id}-kind`}>Tipo de problema</label>
        <select id={`${id}-kind`} value={kind} onChange={(e) => setKind(e.target.value)}>
          {kinds.map((k) => <option key={k.value} value={k.value}>{k.label}</option>)}
        </select>
      </div>
      {targets && (
        <div className="modal-field">
          <label htmlFor={`${id}-target`}>Bulto o producto con el problema</label>
          <select id={`${id}-target`} value={target} onChange={(e) => setTarget(e.target.value)}>
            <option value="">Toda la orden</option>
            {targets.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
          </select>
        </div>
      )}
    </ConfirmDialog>
  );
}
