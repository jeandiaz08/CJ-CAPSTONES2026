"use client";

import { useEffect, useRef, useSyncExternalStore } from "react";
import { CircleAlert, CircleCheck, Info, X } from "lucide-react";

export type ToastTone = "success" | "error" | "warning" | "info";
type Toast = { id: number; message: string; tone: ToastTone };

const ICONS = { success: CircleCheck, error: CircleAlert, warning: Info, info: Info };
const DURATION_MS = 5000;
const ERROR_DURATION_MS = 8000;

let toasts: Toast[] = [];
let nextId = 1;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((listener) => listener());
const dismiss = (id: number) => {
  toasts = toasts.filter((toast) => toast.id !== id);
  emit();
};

export function showToast(message: string, tone: ToastTone = "success") {
  toasts = [...toasts, { id: nextId++, message, tone }];
  emit();
}

const subscribe = (listener: () => void) => {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
};
const empty: Toast[] = [];

// Se cierra solo; el tiempo se detiene mientras el mouse está encima.
function ToastCard({ toast }: { toast: Toast }) {
  const Icon = ICONS[toast.tone];
  const remaining = useRef(toast.tone === "error" ? ERROR_DURATION_MS : DURATION_MS);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const startedAt = useRef(0);
  const resume = () => {
    startedAt.current = Date.now();
    timer.current = setTimeout(() => dismiss(toast.id), remaining.current);
  };
  const pause = () => {
    clearTimeout(timer.current);
    remaining.current -= Date.now() - startedAt.current;
  };
  useEffect(() => {
    resume();
    return () => clearTimeout(timer.current);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return (
    <div className={`toast toast-${toast.tone}`} onMouseEnter={pause} onMouseLeave={resume}>
      <Icon size={20} aria-hidden="true" />
      <span>{toast.message}</span>
      <button type="button" className="toast-close" aria-label="Cerrar aviso" onClick={() => dismiss(toast.id)}>
        <X size={18} />
      </button>
    </div>
  );
}

export function Toaster() {
  const items = useSyncExternalStore(subscribe, () => toasts, () => empty);
  const errors = items.filter((toast) => toast.tone === "error");
  const others = items.filter((toast) => toast.tone !== "error");
  return (
    <div className="toaster">
      <div className="toaster-region" aria-live="assertive">{errors.map((toast) => <ToastCard key={toast.id} toast={toast} />)}</div>
      <div className="toaster-region" aria-live="polite">{others.map((toast) => <ToastCard key={toast.id} toast={toast} />)}</div>
    </div>
  );
}
