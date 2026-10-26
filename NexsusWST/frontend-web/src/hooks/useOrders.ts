"use client";

import { useCallback, useEffect, useState } from "react";
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

  const reload = useCallback(async () => {
    try {
      const res = await listOrders(JSON.parse(key), companyId);
      setOrders(res.data);
      setTotal(res.total);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [key, companyId]);

  useEffect(() => {
    setLoading(true);
    reload();
  }, [reload]);

  const live = useOrdersSocket(() => reload());
  return { orders, total, loading, error, reload, live };
}
