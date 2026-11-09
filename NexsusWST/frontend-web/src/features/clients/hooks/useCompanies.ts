"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { listAdminCompanies, type AdminCompany, type CompanyFilters } from "@/services/clients";

// Lista de empresas de Administración clientes. Una respuesta vieja no pisa a una más nueva.
export function useCompanies(filters: CompanyFilters) {
  const [rows, setRows] = useState<AdminCompany[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const key = JSON.stringify(filters);
  const latest = useRef(0);

  const reload = useCallback(async () => {
    const request = ++latest.current;
    try {
      const res = await listAdminCompanies(JSON.parse(key));
      if (request !== latest.current) return;
      setRows(res.data);
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

  return { rows, total, loading, error, reload };
}
