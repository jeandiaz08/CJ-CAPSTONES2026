"use client";

import { NotificationBell } from "@/components/NotificationBell";
import { useEffect, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Archive, Check, Package, PackageOpen } from "lucide-react";
import { KpiCard } from "@/components/KpiCard";
import { PageHeader } from "@/components/PageHeader";
import { LiveBadge } from "@/components/StatusBadge";
import { Tabs } from "@/components/Tabs";
import { CourierMark, PrioritySlot, useOrderResource } from "@/features/orders";
import { getCatalog, listCompanies } from "@/services/core";
import { getSummary, type Order } from "@/services/orders";
import { useAuth } from "@/state/auth";
import { ageLabel, unitsOf } from "../format";
import { usePackingQueue } from "../hooks/usePackingQueue";
import { TakeOrderModal } from "./TakeOrderModal";

export const PACKING_PATH = "/ordenes/empaque";
const CAN_PACK = ["admin", "jefe_operaciones", "empaque"];
const REFRESH_AGE_MS = 60000;

function PackingCard({ order, companyName, courierName, mine, now, onOpen }: {
  order: Order;
  companyName?: string;
  courierName: string;
  mine: boolean;
  now: number;
  onOpen: () => void;
}) {
  const items = order.items ?? [];
  return (
    <button type="button" className="packing-card" onClick={onOpen} aria-label={`Orden ${order.externalRef}${order.priority ? ", prioritaria" : ""}`}>
      <span className="packing-card-head">
        <PrioritySlot priority={order.priority} />
        <strong>#{order.externalRef}</strong>
      </span>
      <span className="packing-card-client" title={companyName}>{companyName ?? `Empresa ${order.companyId}`}</span>
      <CourierMark code={order.courier} name={courierName} />
      <span className="packing-card-counts">
        <span>Productos: <strong>{items.length}</strong></span>
        <span>Unidades: <strong>{unitsOf(items)}</strong></span>
        {mine && <span>Bultos: <strong>{order.packagesCount}</strong></span>}
      </span>
      {mine && !order.claimedByName && <span className="packing-card-orphan">Tomada por un usuario deshabilitado</span>}
      <small>{ageLabel(order.createdAt, now)}</small>
    </button>
  );
}

// Cola de empaque: «Por empacar» (las Preparadas) y «Mi empaque» (las que tomé).
export function PackingQueue() {
  const { session, companyId } = useAuth();
  const params = useSearchParams(), router = useRouter(), pathname = usePathname();
  const mine = params.get("tab") === "mine";
  const courierFilter = params.get("courier") ?? "";
  const allowed = !!session && CAN_PACK.includes(session.user.role);
  const { orders, loading, error, reload, live } = usePackingQueue(companyId);
  const [revision, setRevision] = useState(0);
  const [selected, setSelected] = useState<Order | null>(null);
  const [now, setNow] = useState(() => Date.now());
  const summary = useOrderResource(() => getSummary(companyId), `packing-summary:${companyId}:${revision}`, String(companyId));
  const refs = useOrderResource(() => Promise.all([listCompanies(), getCatalog("couriers")]), "packing-references");

  useEffect(() => { if (session && !allowed) router.replace("/ordenes/todas/listado"); }, [session, allowed, router]);
  useEffect(() => { setRevision((v) => v + 1); }, [orders]);
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), REFRESH_AGE_MS);
    return () => clearInterval(timer);
  }, []);

  if (!allowed) return null;

  const companyName = (id: number) => refs.data?.[0].data.find((company) => company.id === id)?.name;
  const courierName = (code: string) => refs.data?.[1].data.find((courier) => courier.code === code)?.name ?? code;
  const inTab = orders.filter((order) => (mine ? order.status === "packing" : order.status === "picked"));
  const couriers = [...new Set(inTab.map((order) => order.courier))];
  const visible = courierFilter ? inTab.filter((order) => order.courier === courierFilter) : inTab;
  const setCourier = (code: string) => {
    const next = new URLSearchParams(params.toString());
    if (code) next.set("courier", code);
    else next.delete("courier");
    const search = next.toString();
    router.replace(search ? `${pathname}?${search}` : pathname);
  };
  const open = (order: Order) => {
    if (mine) router.push(`${PACKING_PATH}/${order.id}`);
    else setSelected(order);
  };
  const count = (status: "picked" | "packing" | "ready_pickup") => (summary.loading ? undefined : summary.data?.data[status] ?? 0);

  return (
    <div className="orders-view packing-view">
      <PageHeader title="Empaque" actions={<>
        <LiveBadge live={live} />
        <NotificationBell />
      </>} />
      <Tabs items={[{ href: PACKING_PATH, label: "Por empacar", active: !mine }, { href: `${PACKING_PATH}?tab=mine`, label: "Mi empaque", active: mine }]} />
      <div className="orders-kpis">
        <KpiCard label="Preparadas" count={count("picked")} tone="processing" icon={<Package size={30} />} />
        <KpiCard label="En empaque" count={count("packing")} tone="warn" icon={<PackageOpen size={30} />} />
        <KpiCard label="Lista para retiro" count={count("ready_pickup")} tone="ok" icon={<Archive size={30} />} />
      </div>
      {summary.error && <p role="alert">No se pudieron cargar los indicadores.</p>}
      {couriers.length > 0 && (
        <div className="packing-chips" role="group" aria-label="Filtrar por courier">
          <button type="button" className={`packing-chip ${courierFilter ? "" : "is-active"}`} aria-pressed={!courierFilter} onClick={() => setCourier("")}>{!courierFilter && <Check size={14} aria-hidden="true" />}Todos</button>
          {couriers.map((code) => (
            <button key={code} type="button" className={`packing-chip ${courierFilter === code ? "is-active" : ""}`} aria-pressed={courierFilter === code} onClick={() => setCourier(courierFilter === code ? "" : code)}>
              {courierFilter === code && <Check size={14} aria-hidden="true" />}{courierName(code)}
            </button>
          ))}
        </div>
      )}
      {error && orders.length > 0 && <p role="alert">No se pudo actualizar: {error}</p>}
      {error && orders.length === 0 ? (
        <div role="alert" className="pad">{error} <button className="btn ghost" onClick={reload}>Reintentar</button></div>
      ) : loading && orders.length === 0 ? (
        <div className="packing-grid" aria-label="Cargando órdenes">{Array.from({ length: 6 }, (_, i) => <div key={i} className="detail-skeleton packing-skeleton" />)}</div>
      ) : visible.length === 0 ? (
        <p className="pad">{courierFilter ? "Ninguna orden de este courier" : mine ? "No tienes órdenes en empaque" : "No hay órdenes por empacar"}</p>
      ) : (
        <div className="packing-grid">
          {visible.map((order) => <PackingCard key={order.id} order={order} companyName={companyName(order.companyId)} courierName={courierName(order.courier)} mine={mine} now={now} onOpen={() => open(order)} />)}
        </div>
      )}
      {selected && <TakeOrderModal order={selected} companyName={companyName(selected.companyId)} courierName={courierName(selected.courier)} now={now} onClose={() => setSelected(null)} onLost={() => { setSelected(null); reload(); }} onTaken={(order) => { setSelected(null); reload(); router.push(`${PACKING_PATH}/${order.id}`); }} />}
    </div>
  );
}
