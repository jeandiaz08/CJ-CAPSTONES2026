"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useOrdersSocket } from "@/hooks/useOrdersSocket";
import { listActivity, type ActivityFilters, type ActivityItem } from "@/services/activity";

// Acciones de un apartado; se refrescan con los avisos del WebSocket sin vaciar la tabla.
export function useActivity(filters: ActivityFilters) {
  const [items, setItems] = useState<ActivityItem[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const key = JSON.stringify(filters);
  const latest = useRef(0);

  const reload = useCallback(async () => {
    const request = ++latest.current; // una respuesta vieja no pisa a una más nueva
    try {
      const res = await listActivity(JSON.parse(key));
      if (request !== latest.current) return;
      setItems(res.items);
      setTotal(res.total);
      setError(null);
    } catch (e) {
      if (request === latest.current) setError((e as Error).message);
    } finally {
      if (request === latest.current) setLoading(false);
    }
  }, [key]);

  useEffect(() => {
    setLoading(true);
    reload();
  }, [reload]);

  const live = useOrdersSocket(() => reload());
  return { items, total, loading, error, reload, live };
}
