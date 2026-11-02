"use client";
import { useEffect, useRef, useId } from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";

// Pila de modales abiertos: Esc y Tab solo actúan sobre el de más arriba.
const stack: symbol[] = [];

export function Modal({ title, onClose, children, aside, panelClassName = "", backdropClassName = "" }: {
    title: string;
    onClose: () => void;
    children: React.ReactNode;
    aside?: React.ReactNode;
    panelClassName?: string;
    backdropClassName?: string;
}) {
    const panel = useRef<HTMLDivElement>(null), close = useRef(onClose), id = useId();
    useEffect(() => { close.current = onClose; }, [onClose]);
    useEffect(() => {
        const self = Symbol("modal");
        stack.push(self);
        const previous = document.activeElement as HTMLElement;
        const overflow = document.body.style.overflow;
        document.body.style.overflow = "hidden";
        panel.current?.focus();
        const key = (e: KeyboardEvent) => {
            if (stack.at(-1) !== self) return;
            if (e.key === "Escape") {
                e.preventDefault();
                close.current();
            }
            if (e.key === "Tab") {
                const items = Array.from(panel.current?.querySelectorAll<HTMLElement>('button:not(:disabled),a[href],input:not(:disabled),select:not(:disabled),textarea:not(:disabled),[tabindex="0"]') ?? []);
                const first = items[0], last = items.at(-1);
                if (!first) {
                    e.preventDefault();
                    return;
                }
                if (e.shiftKey && (document.activeElement === first || document.activeElement === panel.current)) {
                    e.preventDefault();
                    last?.focus();
                }
                else if (!e.shiftKey && (document.activeElement === last || document.activeElement === panel.current)) {
                    e.preventDefault();
                    first.focus();
                }
            }
        };
        document.addEventListener("keydown", key);
        return () => {
            stack.splice(stack.indexOf(self), 1);
            document.removeEventListener("keydown", key);
            document.body.style.overflow = overflow;
            previous?.focus();
        };
    }, []);
    if (typeof document === "undefined") return null;
    // En el body: el panel de detalle es sticky y crea su propio contexto de apilamiento.
    return createPortal(<div className={`modal-backdrop ${backdropClassName}`} onClick={e => { e.stopPropagation(); close.current(); }}>
    <div className={`modal ${panelClassName}`} ref={panel} tabIndex={-1} role="dialog" aria-modal="true" aria-labelledby={id} onClick={e => e.stopPropagation()}>
    <div className="modal-head">
    <h2 id={id}>{title}</h2>
    {aside && <div className="modal-aside">{aside}</div>}
    <button type="button" className="icon-btn" onClick={() => close.current()} aria-label="Cerrar">
    <X size={18}/>
    </button>
    </div>{children}</div>
    </div>, document.body);
}
