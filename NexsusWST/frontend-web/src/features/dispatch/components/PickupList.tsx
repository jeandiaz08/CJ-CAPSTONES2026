"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { ScanLine } from "lucide-react";
import { scannedCodes, totalPackages, labelBlock, type PickupRow, type ScanNotice } from "../pickupList";

// «Lista de retiro · {Courier}»: lector siempre visible y tabla de órdenes del courier.
export function PickupList({ courierName, rows, marked, selected, notice, canOperate, companyName, onScan, onToggle, onView }: {
  courierName: string;
  rows: PickupRow[];
  marked: string[];
  selected: number[];
  notice: ScanNotice | null;
  canOperate: boolean;
  companyName: (id: number) => string;
  onScan: (code: string) => void;
  onToggle: (orderId: number) => void;
  onView: (row: PickupRow) => void;
}) {
  const [code, setCode] = useState("");
  const reader = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (canOperate) reader.current?.focus();
  }, [canOperate, courierName]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!code.trim()) return;
    onScan(code);
    setCode("");
    reader.current?.focus();
  };
  const expected = rows.reduce((total, row) => total + totalPackages(row), 0);
  const marks = { marked, selected };

  return (
    <section className="dispatch-panel dispatch-list" aria-labelledby="dispatch-list-title" aria-busy={rows.some((r) => !r.packages)}>
      <div className="dispatch-panel-head">
        <h2 id="dispatch-list-title">Lista de retiro <span aria-hidden="true">•</span> {courierName}</h2>
        <span>{rows.length} {rows.length === 1 ? "orden" : "órdenes"} – {expected} {expected === 1 ? "bulto esperado" : "bultos esperados"}</span>
      </div>
      <form className="dispatch-reader" onSubmit={submit}>
        <ScanLine size={22} aria-hidden="true" />
        <label htmlFor="dispatch-reader-input" className="sr-only">Código de la etiqueta</label>
        <input id="dispatch-reader-input" ref={reader} value={code} disabled={!canOperate} autoComplete="off" placeholder="Escanear código de la etiqueta…" onChange={(e) => setCode(e.target.value)} />
        <span className="dispatch-reader-hint">{rows.some((r) => !r.packages) ? "Cargando bultos…" : "Cada lectura marca el bulto"}</span>
      </form>
      <p className={`dispatch-reader-notice ${notice ? `is-${notice.tone}` : ""}`} role="status">{notice?.text}</p>
      {rows.length === 0 ? (
        <p className="pad">No hay órdenes Listas para retiro de {courierName}.</p>
      ) : (
        <div className="dispatch-table">
          <table>
            <thead>
              <tr>
                <th scope="col"><span className="sr-only">Seleccionar</span></th>
                <th scope="col">Código</th>
                <th scope="col">N° orden</th>
                <th scope="col">Cliente</th>
                <th scope="col">Destinatario</th>
                <th scope="col">Comuna</th>
                <th scope="col">Bultos</th>
                <th scope="col"><span className="sr-only">Detalle</span></th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => {
                const { order } = row;
                const block = labelBlock(order);
                const total = totalPackages(row);
                const done = scannedCodes(row, marks).length;
                const tone = done === 0 ? "" : done >= total ? "is-complete" : "is-partial";
                const inManifest = row.manifestId != null;
                return (
                  <tr key={order.id} className={block ? "is-blocked" : ""}>
                    <td>
                      <input type="checkbox" aria-label={`Seleccionar orden ${order.externalRef}`} disabled={!canOperate || !!block || inManifest}
                        checked={inManifest || selected.includes(order.id)} onChange={() => onToggle(order.id)} />
                    </td>
                    <td>{order.label.status === "ready" ? order.label.trackingCode || "—" : "—"}</td>
                    <td>
                      <strong>#{order.externalRef}</strong>
                      {block && <span className="dispatch-flag is-blocked">{block}</span>}
                      {inManifest && <span className="dispatch-flag">En manifiesto #{row.manifestId}</span>}
                    </td>
                    <td>{companyName(order.companyId)}</td>
                    <td>{order.recipientName || "—"}</td>
                    <td>{order.commune || "—"}</td>
                    <td><span className={`dispatch-count ${tone}`} aria-label={`${done} de ${total} bultos marcados`}>{done}/{total}</span></td>
                    <td><button type="button" className="btn dispatch-view" aria-label={`Ver orden ${order.externalRef}`} onClick={() => onView(row)}>VER</button></td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
