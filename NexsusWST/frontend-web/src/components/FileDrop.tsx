"use client";

import { useId, useRef, useState, type DragEvent } from "react";
import { FileUp, X } from "lucide-react";

// Arrastrar o elegir un archivo. Valida extensión y tamaño antes de aceptarlo.
export function FileDrop({ label, accept, maxBytes, value, onChange }: {
  label: string;
  accept: string[]; // extensiones en minúscula: [".xlsx", ".csv"]
  maxBytes: number;
  value: File | null;
  onChange: (file: File | null) => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const hintId = useId();
  const [dragging, setDragging] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const maxLabel = `${Math.round(maxBytes / (1 << 20))} MB`;

  const pick = (file?: File) => {
    if (!file) return;
    if (!accept.some((ext) => file.name.toLowerCase().endsWith(ext))) {
      setError(`Formato no permitido. Usa ${accept.join(", ")}.`);
      return;
    }
    if (file.size > maxBytes) {
      setError(`El archivo supera el máximo de ${maxLabel}.`);
      return;
    }
    setError(null);
    onChange(file);
  };
  const drop = (e: DragEvent) => {
    e.preventDefault();
    setDragging(false);
    pick(e.dataTransfer.files[0]);
  };

  return (
    <div className="modal-field">
      <span>{label}</span>
      <div className={`file-drop ${dragging ? "is-dragging" : ""}`} onDragOver={(e) => { e.preventDefault(); setDragging(true); }} onDragLeave={() => setDragging(false)} onDrop={drop}>
        <FileUp size={24} aria-hidden="true" />
        {value ? (
          <span className="file-drop-name">
            {value.name}
            <button type="button" className="toast-close" aria-label="Quitar archivo" onClick={() => { onChange(null); if (input.current) input.current.value = ""; }}><X size={16} /></button>
          </span>
        ) : (
          <span>
            Arrastra el archivo aquí o{" "}
            <button type="button" className="text-button" aria-describedby={hintId} onClick={() => input.current?.click()}>elige uno</button>
          </span>
        )}
        <small id={hintId}>{accept.join(", ")} · máximo {maxLabel}</small>
        <input ref={input} type="file" hidden accept={accept.join(",")} onChange={(e) => pick(e.target.files?.[0])} />
      </div>
      {error && <span className="modal-field-error" role="alert">{error}</span>}
    </div>
  );
}
