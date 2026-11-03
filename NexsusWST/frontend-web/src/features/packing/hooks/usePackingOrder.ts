"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useOrdersSocket } from "@/hooks/useOrdersSocket";
import { getOrder } from "@/services/orders";

type Detail = Awaited<ReturnType<typeof getOrder>>;

// Detalle de la orden en empaque, en vivo. Conserva lo mostrado mientras refresca.
export function usePackingOrder(id: number, companyId: number | null) {
  const [detail, setDetail] = useState<Detail | null>(null);
  const [error, setError] = useState<string | null>(null);
  const latest = useRef(0);

  const reload = useCallback(async () => {
    const request = ++latest.current; // una respuesta vieja no pisa a una más nueva
    try {
      const res = await getOrder(id, companyId);
      if (request !== latest.current) return;
      setDetail(res);
      setError(null);
    } catch (e) {
      if (request === latest.current) setError((e as Error).message);
    }
  }, [id, companyId]);

  useEffect(() => {
    reload();
  }, [reload]);

  const live = useOrdersSocket((e) => {
    if (e.type === "reconnected" || e.orderId === id) reload();
  });
  return { detail, error, reload, live };
}
