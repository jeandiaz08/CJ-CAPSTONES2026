"use client";

import { useEffect, useRef, useState } from "react";
import { wsUrl } from "@/services/api";
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
let currentToken: string | null = null;

function setConnected(c: boolean) {
  connected = c;
  statusListeners.forEach((l) => l(c));
}

function connect(token: string) {
  currentToken = token;
  ws = new WebSocket(wsUrl("/ws"));
  ws.onopen = () => {
    setConnected(true);
    listeners.forEach((l) => l({ type: "reconnected", orderId: 0, companyId: 0, status: "", version: 0 }));
  };
  ws.onmessage = (m) => {
    const e = JSON.parse(m.data) as OrderChanged;
    listeners.forEach((l) => l(e));
  };
  ws.onclose = () => {
    setConnected(false);
    ws = null;
    if (currentToken && listeners.size > 0) retry = setTimeout(() => currentToken && connect(currentToken), 3000);
  };
}

function disconnect() {
  currentToken = null;
  clearTimeout(retry);
  ws?.close();
  ws = null;
}

export function useOrdersSocket(onEvent: Listener) {
  const { session } = useAuth();
  const [isConnected, setIsConnected] = useState(connected);
  const handler = useRef(onEvent);
  handler.current = onEvent;

  useEffect(() => {
    if (!session) return;
    const l: Listener = (e) => handler.current(e);
    listeners.add(l);
    statusListeners.add(setIsConnected);
    if (currentToken !== session.accessToken) {
      disconnect();
      connect(session.accessToken);
    }
    return () => {
      listeners.delete(l);
      statusListeners.delete(setIsConnected);
      if (listeners.size === 0) disconnect();
    };
  }, [session]);

  return isConnected;
}
