"use client";

import { useState } from "react";
import { Star } from "lucide-react";
import { applyEvent, setPriority, type Order } from "@/services/orders";
import { useAuth } from "@/state/auth";
import { actionsFor } from "./statuses";

// Botones de acción de una orden según su estado y el rol del usuario.
export function OrderActions({ order, onDone, compact }: { order: Order; onDone?: () => void; compact?: boolean }) {
  const { can } = useAuth();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      onDone?.();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const actions = actionsFor(order.status).filter((a) => can({ roles: a.roles, permission: a.permission }));
  const canPrioritize = can({ roles: ["admin", "jefe_operaciones"] }) && !["handed_over", "cancelled"].includes(order.status);

  return (
    <div className={`actions ${compact ? "compact" : ""}`}>
      {canPrioritize && (
        <button
          className={`btn ghost ${order.priority ? "is-on" : ""}`}
          disabled={busy}
          title={order.priority ? "Quitar prioridad" : "Enviar primero a la cola"}
          onClick={() => {
            const reason = order.priority ? "" : (prompt("Motivo de la prioridad (queda en el historial):") ?? "");
            if (!order.priority && !reason) return;
            run(() => setPriority(order.id, !order.priority, reason));
          }}
        >
          <Star size={15} fill={order.priority ? "currentColor" : "none"} />
          {!compact && (order.priority ? "Prioritaria" : "Priorizar")}
        </button>
      )}
      {actions.map((a) => (
        <button
          key={a.event}
          className={`btn ${a.danger ? "danger" : "primary"}`}
          disabled={busy}
          title={a.simulated ? "SIMULADO: en producción lo confirma Invas" : undefined}
          onClick={() => {
            let reason = "";
            if (a.needsReason) {
              reason = prompt(a.event === "hand_over" ? "Nombre de quien retira (courier)" : `${a.label}: indica el motivo`) ?? "";
              if (!reason) return;
            }
            run(() => applyEvent(order, a.event, reason));
          }}
        >
          {a.label}
          {a.simulated && " (sim.)"}
        </button>
      ))}
      {error && <span className="error small">{error}</span>}
    </div>
  );
}
