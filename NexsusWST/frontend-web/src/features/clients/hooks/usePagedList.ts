"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// Lista paginada de Administración clientes (empresas, perfiles, solicitudes). `key` identifica los
// filtros; una respuesta vieja no pisa a una más nueva y la tabla no se vacía al recargar.
export function usePagedList<T>(load: () => Promise<{ data: T[]; total: number }>, key: string) {
  const [rows, setRows] = useState<T[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const latest = useRef(0);

  const reload = useCallback(async () => {
    const request = ++latest.current;
    try {
      const res = await load(); // `load` solo depende de `key`
      if (request !== latest.current) return;
      setRows(res.data);
      setTotal(res.total);
      setError(null);
    } catch (e) {
      if (request === latest.current) setError((e as Error).message);
    } finally {
      if (request === latest.current) setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  useEffect(() => {
    setLoading(true);
    reload();
  }, [reload]);

  return { rows, total, loading, error, reload };
}
