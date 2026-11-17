"use client";

import { useCallback, useEffect, useState } from "react";
import { getSummary } from "@/services/orders";
import { useAuth } from "@/state/auth";
import { useOrdersSocket } from "./useOrdersSocket";

// Órdenes Preparadas pendientes de empacar, en vivo, para el contador del sidebar.
export function usePreparedCount(enabled: boolean) {
  const { companyId } = useAuth();
  const [count, setCount] = useState(0);
  const load = useCallback(async () => {
    if (!enabled) return;
    try {
      const res = await getSummary(companyId);
      setCount(res.data.picked ?? 0);
    } catch {
      // El contador es informativo: si falla, conserva el último valor.
    }
  }, [enabled, companyId]);
  useEffect(() => {
    load();
  }, [load]);
  useOrdersSocket(() => load());
  return enabled ? count : 0;
}
