"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useOrdersSocket } from "@/hooks/useOrdersSocket";
import { getPackingQueue, type Order } from "@/services/orders";

// Cola de empaque en vivo. Conserva las tarjetas mientras refresca: las que ya no
// corresponden desaparecen y las nuevas aparecen, sin pestañeo.
export function usePackingQueue(companyId: number | null) {
  const [orders, setOrders] = useState<Order[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const latest = useRef(0);

  const reload = useCallback(async () => {
    const request = ++latest.current; // una respuesta vieja no pisa a una más nueva
    try {
      const res = await getPackingQueue(companyId);
      if (request !== latest.current) return;
      setOrders(res.data);
      setError(null);
    } catch (e) {
      if (request === latest.current) setError((e as Error).message);
    } finally {
      if (request === latest.current) setLoading(false);
    }
  }, [companyId]);

  useEffect(() => {
    reload();
  }, [reload]);

  const live = useOrdersSocket(() => reload());
  return { orders, loading, error, reload, live };
}
