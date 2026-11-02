"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { listOrders, type Order, type OrderFilters } from "@/services/orders";
import { useAuth } from "@/state/auth";
import { useOrdersSocket } from "./useOrdersSocket";

// Lista de órdenes que se refresca sola cuando llega un aviso por WebSocket.
export function useOrders(filters: OrderFilters) {
  const { companyId } = useAuth();
  const [orders, setOrders] = useState<Order[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const key = JSON.stringify(filters);
  const scope = filters.companyId ?? companyId;
  const latest = useRef(0);

  const reload = useCallback(async () => {
    const request = ++latest.current; // una respuesta vieja no pisa a una más nueva
    try {
      const res = await listOrders(JSON.parse(key), scope);
      if (request !== latest.current) return;
      setOrders(res.data);
      setTotal(res.total);
      setError(null);
    } catch (e) {
      if (request === latest.current) setError((e as Error).message);
    } finally {
      if (request === latest.current) setLoading(false);
    }
  }, [key, scope]);

  useEffect(() => {
    setLoading(true);
    reload();
  }, [reload]);

  const live = useOrdersSocket(() => reload());
  return { orders, total, loading, error, reload, live };
}
