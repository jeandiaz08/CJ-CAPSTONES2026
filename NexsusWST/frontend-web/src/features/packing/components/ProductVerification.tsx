"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { Check, Minus, Plus, ScanLine } from "lucide-react";
import type { OrderItem } from "@/services/orders";
import { scanSku, type Counts, type ScanNotice } from "../verification";

// «Verifica cada producto»: lector por SKU y una tarjeta por producto.
export function ProductVerification({ reference, items, names, counts, onChange, disabled }: {
  reference: string;
  items: OrderItem[];
  names: Record<string, string>;
  counts: Counts;
  onChange: (counts: Counts) => void;
  disabled: boolean;
}) {
  const [code, setCode] = useState("");
  const [notice, setNotice] = useState<ScanNotice | null>(null);
  const reader = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (!disabled) reader.current?.focus();
  }, [disabled]);

  const scan = (e: FormEvent) => {
    e.preventDefault();
    if (!code.trim()) return;
    const result = scanSku(items, counts, code);
    onChange(result.counts);
    setNotice(result.notice);
    setCode("");
    reader.current?.focus();
  };
  const set = (sku: string, value: number) => {
    setNotice(null);
    onChange({ ...counts, [sku]: value });
  };

  return (
    <section className="packing-panel">
      <div className="packing-panel-head">
        <h2>Verifica cada producto</h2>
        <span>N° orden: #{reference}</span>
      </div>
      <form className="packing-reader" onSubmit={scan}>
        <label htmlFor="packing-reader-input"><ScanLine size={18} aria-hidden="true" />Lector de SKU</label>
        <input id="packing-reader-input" ref={reader} value={code} disabled={disabled} autoComplete="off" placeholder="Lee el código o escribe el SKU y presiona Enter" onChange={(e) => setCode(e.target.value)} />
        <p className={`packing-reader-notice ${notice ? `is-${notice.tone}` : ""}`} role="status">{notice?.text}</p>
      </form>
      <ul className="packing-products">
        {items.map((item) => {
          const count = counts[item.sku] ?? 0;
          const done = count === item.quantity;
          return (
            <li key={item.sku} className={`packing-product ${done ? "is-done" : ""}`}>
              <button type="button" className="packing-check" disabled={disabled} aria-pressed={done} aria-label={`${done ? "Desmarcar" : "Marcar"} ${item.sku}`} onClick={() => set(item.sku, done ? 0 : item.quantity)}>
                {done && <Check size={30} aria-hidden="true" />}
              </button>
              <div className="packing-product-text">
                <strong>{names[item.sku] ?? "Producto sin descripción"}</strong>
                <span>SKU: {item.sku}</span>
              </div>
              <div className="packing-counter">
                <button type="button" className="packing-step" disabled={disabled || count === 0} aria-label={`Quitar una unidad de ${item.sku}`} onClick={() => set(item.sku, count - 1)}><Minus size={20} aria-hidden="true" /></button>
                <span aria-label={`${count} de ${item.quantity} verificadas`}><strong>{count}</strong> / {item.quantity}</span>
                <button type="button" className="packing-step" disabled={disabled || done} aria-label={`Sumar una unidad de ${item.sku}`} onClick={() => set(item.sku, count + 1)}><Plus size={20} aria-hidden="true" /></button>
              </div>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
