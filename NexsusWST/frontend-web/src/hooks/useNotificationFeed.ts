"use client";

import { useSyncExternalStore } from "react";
import { getOrder } from "@/services/orders";
import { useAuth } from "@/state/auth";
import type { AccessUser } from "@/state/menuAccess";
import { useOrdersSocket, type OrderChanged } from "./useOrdersSocket";

// Avisos de la campana: los que llegan por WebSocket mientras la app está abierta. Se guardan
// solo en memoria (los últimos 20); recargar la página los borra.
// TODO(api): GET /v1/notifications ya guarda los avisos; usarlo para no perderlos al recargar.
export type NoticeKind = "incident" | "label_ready" | "delivered" | "despacho_incompleto" | "profile_request" | "profile_request_reviewed"
  | "internal_profile_request" | "internal_profile_request_reviewed";
// href: a dónde lleva el aviso (las solicitudes de perfiles abren su pantalla).
export type Notice = { key: string; kind: NoticeKind; title: string; body: string; orderId: number | null; at: string; read: boolean; href?: string };

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
  notification?: { id: number | string; kind: string; title: string; body: string; orderId?: number | null; requestId?: number; userId?: number; createdAt: string };
};

// Solicitudes de perfiles (Tarea 07): la nueva solicitud solo la ve quien tiene «Gestionar perfiles»;
// la respuesta, solo el Cliente (el servidor ya no le envía al Cliente los avisos internos).
// WeStorage (Tarea 08): la nueva, solo el Administrador con el permiso; la respuesta va por el canal
// interno y la muestra solo quien la pidió (userId).
function requestNotice(n: NonNullable<NotificationMessage["notification"]>, user: (AccessUser & { id?: number }) | null) {
  const internalHref = `/configuracion/usuarios?tab=solicitudes&solicitud=${n.requestId}`;
  if (n.kind === "internal_profile_request" && user?.role === "admin" && user.permissions.includes("gestionar_perfiles")) {
    return { kind: n.kind, href: internalHref } as const;
  }
  if (n.kind === "internal_profile_request_reviewed" && user?.id !== undefined && user.id === n.userId) {
    return { kind: n.kind, href: internalHref } as const;
  }
  if (n.kind === "profile_request" && user?.permissions.includes("gestionar_perfiles")) {
    return { kind: n.kind, href: `/clientes/administracion?tab=solicitudes&solicitud=${n.requestId}` } as const;
  }
  if (n.kind === "profile_request_reviewed" && user?.role === "cliente") {
    return { kind: n.kind, href: `/clientes/perfiles-empresa?tab=solicitudes&solicitud=${n.requestId}` } as const;
  }
  return null;
}

async function handle(message: NotificationMessage, user: (AccessUser & { id?: number }) | null) {
  const n = message.notification;
  const request = message.type === "notification" && n ? requestNotice(n, user) : null;
  if (n && request) {
    push({ key: `notification:${n.id}`, kind: request.kind, title: n.title, body: n.body, orderId: null, at: n.createdAt, href: request.href });
    return;
  }
  if (message.type === "notification" && n && (n.kind === "incident" || n.kind === "label_ready" || n.kind === "despacho_incompleto")) {
    push({ key: `notification:${n.id}`, kind: n.kind, title: n.title, body: n.body, orderId: n.orderId ?? null, at: n.createdAt });
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
  const { session } = useAuth();
  const user = session?.user ?? null;
  useOrdersSocket((message) => void handle(message as NotificationMessage, user));
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
