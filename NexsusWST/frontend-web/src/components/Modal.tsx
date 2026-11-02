"use client";
import { useEffect, useRef, useId } from "react";
import { X } from "lucide-react";
export function Modal({ title, onClose, children }: {
    title: string;
    onClose: () => void;
    children: React.ReactNode;
}) {
    const panel = useRef<HTMLDivElement>(null), close = useRef(onClose), id = useId();
    useEffect(() => { close.current = onClose; }, [onClose]);
    useEffect(() => {
        const previous = document.activeElement as HTMLElement;
        const overflow = document.body.style.overflow;
        document.body.style.overflow = "hidden";
        panel.current?.focus();
        const key = (e: KeyboardEvent) => { if (e.key === "Escape") {
            e.preventDefault();
            close.current();
        } if (e.key === "Tab") {
            const items = Array.from(panel.current?.querySelectorAll<HTMLElement>('button:not(:disabled),a[href],input:not(:disabled),[tabindex="0"]') ?? []);
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
        } };
        document.addEventListener("keydown", key);
        return () => { document.removeEventListener("keydown", key); document.body.style.overflow = overflow; previous?.focus(); };
    }, []);
    return <div className="modal-backdrop" onClick={() => close.current()}>
    <div className="modal" ref={panel} tabIndex={-1} role="dialog" aria-modal="true" aria-labelledby={id} onClick={e => e.stopPropagation()}>
    <div className="modal-head">
    <h2 id={id}>{title}</h2>
    <button className="icon-btn" onClick={onClose} aria-label="Cerrar">
    <X size={18}/>
    </button>
    </div>{children}</div>
    </div>;
}
