"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useOrdersSocket } from "@/hooks/useOrdersSocket";
import { getDispatchCouriers, listManifests, type CourierQueue, type Manifest } from "@/services/dispatch";
import { getOrder, listOrders, type Order, type Package } from "@/services/orders";
import type { PickupRow } from "../pickupList";

const DETAIL_CONCURRENCY = 6;
const LIVE_DEBOUNCE_MS = 300;

async function mapLimit<T, R>(items: T[], limit: number, fn: (item: T) => Promise<R>) {
  const out: R[] = new Array(items.length);
  let next = 0;
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, async () => {
    while (next < items.length) {
      const i = next++;
      out[i] = await fn(items[i]);
    }
  }));
  return out;
}

// Couriers con órdenes Listas para retiro y la lista del courier elegido, en vivo y sin
// pestañeo: mientras refresca se conserva lo mostrado.
export function usePickupList(courier: string | null, companyId: number | null) {
  const [couriers, setCouriers] = useState<CourierQueue[] | null>(null);
  const [rows, setRows] = useState<PickupRow[] | null>(null);
  const [openManifest, setOpenManifest] = useState<Manifest | null>(null);
  const [error, setError] = useState<string | null>(null);
  const latest = useRef(0);
  // TODO(api): el listado no trae los códigos de bulto; se piden con el detalle de cada orden.
  const packagesCache = useRef(new Map<string, Package[]>());

  const reload = useCallback(async () => {
    const request = ++latest.current;
    try {
      const [queues, list, manifests] = await Promise.all([
        getDispatchCouriers(companyId),
        courier ? listOrders({ status: "ready_pickup", courier, limit: 200 }, companyId) : null,
        courier ? listManifests("open") : null,
      ]);
      if (request !== latest.current) return;
      setCouriers(queues.data);
      setError(null);
      if (!courier || !list) {
        setRows(null);
        setOpenManifest(null);
        return;
      }
      const open = manifests?.data.find((m) => m.courier === courier) ?? null;
      const lineOf = (order: Order) => open?.lines.find((l) => l.orderId === order.id && l.status === "included");
      const key = (o: Order) => `${o.id}:${o.version}:${o.packagesCount}`;
      const build = (order: Order): PickupRow => {
        const line = lineOf(order);
        return {
          order,
          packages: packagesCache.current.get(key(order)) ?? line?.packages.map((p) => ({ id: p.id, code: p.code, manifestId: open?.id ?? null })),
          manifestId: line ? open?.id : undefined,
          serverScanned: line ? line.packages.filter((p) => p.scanned).map((p) => p.code) : [],
        };
      };
      setOpenManifest(open);
      setRows(list.data.map(build));
      const missing = list.data.filter((o) => !packagesCache.current.has(key(o)) && !lineOf(o));
      if (missing.length === 0) return;
      await mapLimit(missing, DETAIL_CONCURRENCY, async (order) => {
        try {
          packagesCache.current.set(key(order), (await getOrder(order.id, companyId)).packages);
        } catch {
          // Sin detalle, la fila muestra el total y la lectura de sus bultos queda pendiente.
        }
      });
      if (request !== latest.current) return;
      setRows(list.data.map(build));
    } catch (e) {
      if (request === latest.current) setError((e as Error).message);
    }
  }, [courier, companyId]);

  useEffect(() => {
    reload();
  }, [reload]);

  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  const live = useOrdersSocket(() => {
    clearTimeout(timer.current);
    timer.current = setTimeout(reload, LIVE_DEBOUNCE_MS);
  });

  return { couriers, rows, openManifest, error, reload, live };
}
