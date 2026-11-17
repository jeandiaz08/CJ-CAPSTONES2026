"use client";

import { useEffect, useId, useRef, useState, type ReactNode } from "react";

export type DropdownItem = { label: string; icon?: ReactNode; danger?: boolean; disabled?: boolean; onSelect: () => void };

// Menú flotante accesible: flechas para moverse, Esc o clic fuera para cerrar.
export function DropdownMenu({ trigger, triggerClassName = "btn ghost", disabled = false, className = "", menuClassName = "", items }: {
  trigger: ReactNode;
  triggerClassName?: string;
  disabled?: boolean;
  className?: string;
  menuClassName?: string;
  items: DropdownItem[];
}) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const id = useId();
  useEffect(() => {
    if (!open) return;
    const menuItems = () => Array.from(root.current?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]:not(:disabled)') ?? []);
    menuItems()[0]?.focus();
    const outside = (e: PointerEvent) => { if (!root.current?.contains(e.target as Node)) setOpen(false); };
    const key = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
        root.current?.querySelector("button")?.focus();
      }
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        const list = menuItems();
        const index = list.indexOf(document.activeElement as HTMLButtonElement);
        list[(index + (e.key === "ArrowDown" ? 1 : list.length - 1)) % list.length]?.focus();
      }
    };
    document.addEventListener("pointerdown", outside);
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("keydown", key);
    };
  }, [open]);
  return (
    <div className={`dropdown ${className}`} ref={root}>
      <button type="button" className={triggerClassName} disabled={disabled} aria-haspopup="menu" aria-expanded={open} aria-controls={open ? id : undefined} onClick={() => setOpen(!open)}>{trigger}</button>
      {open && (
        <div className={`dropdown-menu ${menuClassName}`} role="menu" id={id}>
          {items.map((item) => (
            <button key={item.label} type="button" role="menuitem" disabled={item.disabled} className={item.danger ? "is-danger" : ""} onClick={() => { setOpen(false); item.onSelect(); }}>
              {item.icon}{item.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
