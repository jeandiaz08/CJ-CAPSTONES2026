"use client";

import { useEffect, useRef, useState } from "react";
import { LoaderCircle, Printer } from "lucide-react";
import { showToast } from "@/components/Toast";
import { apiErrorMessage, downloadFile } from "@/services/api";
import { printLabel, type Order } from "@/services/orders";

export const LABEL_STATUS_TEXT: Record<Order["label"]["status"], string> = {
  none: "Sin solicitar",
  pending: "Etiqueta pendiente",
  ready: "Lista",
  failed: "No se pudo obtener",
  office: "Buscar en oficina",
};

// Botón de la etiqueta según el courier. «Buscar en oficina» no imprime nada.
export function LabelStatusAction({ order, packagesCount, disabled, onChanged }: {
  order: Order;
  packagesCount: number;
  disabled: boolean;
  onChanged: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const { status } = order.label;
  const previous = useRef(status);
  useEffect(() => {
    if (previous.current === "pending" && status === "ready") showToast("La etiqueta ya está lista.", "success");
    if (previous.current === "pending" && status === "failed") showToast("El courier no entregó la etiqueta. Puedes reintentarlo.", "error");
    previous.current = status;
  }, [status]);

  if (status === "office") return null;

  const print = async () => {
    setBusy(true);
    try {
      const res = await printLabel(order.id);
      if (res.document && res.downloadPath) await downloadFile(res.downloadPath, res.document.filename);
      else showToast("Etiqueta pendiente: se reintenta sola y avisa cuando llegue.", "info");
      onChanged();
    } catch (e) {
      showToast(apiErrorMessage(e), "error");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="packing-label">
      <button type="button" className="btn packing-action" disabled={disabled || busy || status === "pending" || packagesCount === 0} aria-busy={busy} onClick={print}>
        {busy ? <LoaderCircle size={18} className="spin" aria-hidden="true" /> : <Printer size={18} aria-hidden="true" />}
        {status === "pending" ? "Etiqueta pendiente" : status === "failed" ? "Reintentar etiqueta" : "Imprimir etiqueta"}
      </button>
      {status === "pending" && <small>Se reintenta sola y avisa cuando llegue.</small>}
      {packagesCount === 0 && <small>Registra los bultos para imprimirla.</small>}
    </div>
  );
}
