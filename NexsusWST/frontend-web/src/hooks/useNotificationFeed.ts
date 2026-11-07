"use client";

import { useSyncExternalStore } from "react";
import { getOrder } from "@/services/orders";
import { useOrdersSocket, type OrderChanged } from "./useOrdersSocket";

// Avisos de la campana: los que llegan por WebSocket mientras la app está abierta. Se guardan
// solo en memoria (los últimos 20); recargar la página los borra.
// TODO(api): GET /v1/notifications ya guarda los avisos; usarlo para no perderlos al recargar.
export type NoticeKind = "incident" | "label_ready" | "delivered";
export type Notice = { key: string; kind: NoticeKind; title: string; body: string; orderId: number | null; at: string; read: boolean };

const MAX_NOTICES = 20;
let notices: Notice[] = [];
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((l) => l());

function push(notice: Omit<Notice, "read">) {
  if (notices.some((n) => n.key === notice.key)) return;
  notices = [{ ...notice, read: false }, ...notices].slice(0, MAX_NOTICES);
  emit();
}

export function markAllRead() {
  if (!notices.some((n) => !n.read)) return;
  notices = notices.map((n) => ({ ...n, read: true }));
  emit();
}

export function clearNotices() {
  notices = [];
  emit();
}

type NotificationMessage = OrderChanged & {
  notification?: { id: number; kind: string; title: string; body: string; orderId: number | null; createdAt: string };
};

async function handle(message: NotificationMessage) {
  const n = message.notification;
  if (message.type === "notification" && n && (n.kind === "incident" || n.kind === "label_ready")) {
    push({ key: `notification:${n.id}`, kind: n.kind, title: n.title, body: n.body, orderId: n.orderId, at: n.createdAt });
    return;
  }
  // TODO(api): la entrega no genera un aviso propio; se arma con order.changed y se pide la referencia.
  if (message.type === "order.changed" && message.status === "handed_over") {
    const key = `delivered:${message.orderId}`;
    if (notices.some((x) => x.key === key)) return;
    const ref = await getOrder(message.orderId, null).then((d) => d.order.externalRef).catch(() => String(message.orderId));
    push({ key, kind: "delivered", title: `Orden #${ref} entregada al courier`, body: "Salió en el manifiesto del courier.", orderId: message.orderId, at: new Date().toISOString() });
  }
}

// Se monta una vez, en el layout de la app, para no perder avisos al cambiar de pantalla.
export function useNotificationFeed() {
  useOrdersSocket((message) => void handle(message as NotificationMessage));
}

const subscribe = (listener: () => void) => {
  listeners.add(listener);
  return () => listeners.delete(listener);
};
const snapshot = () => notices;
const empty: Notice[] = [];

export function useNotices() {
  return useSyncExternalStore(subscribe, snapshot, () => empty);
}
