"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useOrdersSocket } from "@/hooks/useOrdersSocket";
import { listRegistry, type RegistryEntry, type RegistryFilters } from "@/services/registry";

// Registro de órdenes que se refresca solo con los avisos del WebSocket, sin vaciar la tabla.
export function useRegistry(filters: RegistryFilters, companyId: number | null) {
  const [rows, setRows] = useState<RegistryEntry[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const key = JSON.stringify(filters);
  const latest = useRef(0);

  const reload = useCallback(async () => {
    const request = ++latest.current; // una respuesta vieja no pisa a una más nueva
    try {
      const res = await listRegistry(JSON.parse(key), companyId);
      if (request !== latest.current) return;
      setRows(res.data);
      setTotal(res.total);
      setError(null);
    } catch (e) {
      if (request === latest.current) setError((e as Error).message);
    } finally {
      if (request === latest.current) setLoading(false);
    }
  }, [key, companyId]);

  useEffect(() => {
    setLoading(true);
    reload();
  }, [reload]);

  const live = useOrdersSocket(() => reload());
  return { rows, total, loading, error, reload, live };
}
