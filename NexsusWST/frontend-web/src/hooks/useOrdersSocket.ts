"use client";

import { useEffect, useRef, useState } from "react";
import { api, wsUrl } from "@/services/api";
import { useAuth } from "@/state/auth";

export type OrderChanged = { type: string; orderId: number; companyId: number; status: string; version: number };
type Listener = (e: OrderChanged) => void;

// UNA sola conexión WSS compartida por toda la aplicación (RF-26).
// Tras reconectar se emite "reconnected" para que cada vista vuelva a
// consultar la API: los avisos perdidos no se recuperan (DAS PA-05).
const listeners = new Set<Listener>();
const statusListeners = new Set<(c: boolean) => void>();
let ws: WebSocket | null = null;
let connected = false;
let retry: ReturnType<typeof setTimeout> | undefined;
let currentUser: number | null = null;
const RETRY_MS = 3000;

function setConnected(c: boolean) {
  connected = c;
  statusListeners.forEach((l) => l(c));
}

function scheduleReconnect() {
  clearTimeout(retry);
  if (currentUser != null && listeners.size > 0) retry = setTimeout(() => currentUser != null && connect(currentUser), RETRY_MS);
}

// Cada conexión usa un ticket nuevo: sirve una sola vez y vence en 30 s.
async function connect(userId: number) {
  currentUser = userId;
  let ticket: string;
  try {
    ({ ticket } = await api<{ ticket: string }>("orders", "/v1/ws-ticket", { method: "POST" }));
  } catch {
    if (currentUser === userId) scheduleReconnect();
    return;
  }
  if (currentUser !== userId || ws) return; // se desconectó o ya hay otra conexión mientras se pedía
  const socket = new WebSocket(wsUrl("/ws", ticket));
  ws = socket;
  socket.onopen = () => {
    setConnected(true);
    listeners.forEach((l) => l({ type: "reconnected", orderId: 0, companyId: 0, status: "", version: 0 }));
  };
  socket.onmessage = (m) => {
    const e = JSON.parse(m.data) as OrderChanged;
    listeners.forEach((l) => l(e));
  };
  socket.onclose = () => {
    if (ws !== socket) return;
    setConnected(false);
    ws = null;
    scheduleReconnect();
  };
}

function disconnect() {
  currentUser = null;
  clearTimeout(retry);
  ws?.close();
  ws = null;
  setConnected(false);
}

export function useOrdersSocket(onEvent: Listener) {
  const { session } = useAuth();
  const [isConnected, setIsConnected] = useState(connected);
  const handler = useRef(onEvent);
  handler.current = onEvent;

  const userId = session?.user.id;

  useEffect(() => {
    if (userId == null) return;
    const l: Listener = (e) => handler.current(e);
    listeners.add(l);
    statusListeners.add(setIsConnected);
    if (currentUser !== userId) {
      disconnect();
      void connect(userId);
    }
    return () => {
      listeners.delete(l);
      statusListeners.delete(setIsConnected);
      if (listeners.size === 0) disconnect();
    };
  }, [userId]);

  return isConnected;
}
