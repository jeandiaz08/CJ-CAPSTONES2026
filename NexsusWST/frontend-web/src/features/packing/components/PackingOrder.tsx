"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Bell, Timer } from "lucide-react";
import { PageHeader } from "@/components/PageHeader";
import { LiveBadge } from "@/components/StatusBadge";
import { Tabs } from "@/components/Tabs";
import { CourierMark, originLabel, useOrderResource } from "@/features/orders";
import { getCatalog, listCompanies } from "@/services/core";
import { useAuth } from "@/state/auth";
import { usePackingOrder } from "../hooks/usePackingOrder";
import { PACKING_PATH } from "./PackingQueue";
import { PackingWork } from "./PackingWork";

const CAN_PACK = ["admin", "jefe_operaciones", "empaque"];

function formatElapsed(ms: number) {
  const total = Math.max(0, Math.floor(ms / 1000));
  const parts = [Math.floor(total / 3600), Math.floor(total / 60) % 60, total % 60];
  return parts.map((v) => String(v).padStart(2, "0")).join(":");
}

// Cronómetro desde que se tomó la orden.
function Elapsed({ since }: { since: string }) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, []);
  return <span className="packing-elapsed"><Timer size={15} aria-hidden="true" /><span role="timer">{formatElapsed(now - new Date(since).getTime())}</span></span>;
}

// «Mi empaque»: la orden tomada, con su verificación, bultos y etiqueta.
export function PackingOrder({ id }: { id: number }) {
  const { session, companyId } = useAuth();
  const router = useRouter();
  const allowed = !!session && CAN_PACK.includes(session.user.role);
  const { detail, error, reload, live } = usePackingOrder(id, companyId);
  const refs = useOrderResource(() => Promise.all([listCompanies(), getCatalog("couriers")]), "packing-references");
  useEffect(() => { if (session && !allowed) router.replace("/ordenes/todas/listado"); }, [session, allowed, router]);
  if (!allowed) return null;

  const order = detail?.order;
  const courierName = order ? refs.data?.[1].data.find((c) => c.code === order.courier)?.name ?? order.courier : "";
  const companyName = order ? refs.data?.[0].data.find((c) => c.id === order.companyId)?.name ?? `Empresa ${order.companyId}` : "";
  return (
    <div className="orders-view packing-view">
      <PageHeader title={order ? `Orden #${order.externalRef} · ${order.statusLabel}` : "Empaque"} actions={<>
        <LiveBadge live={live} />
        <button className="icon-btn" aria-label="Notificaciones"><Bell size={23} /></button>
      </>} />
      <Tabs items={[{ href: PACKING_PATH, label: "Por empacar", active: false }, { href: `${PACKING_PATH}?tab=mine`, label: "Mi empaque", active: true }]} />
      {error && !order && <div role="alert" className="pad">No se pudo cargar la orden: {error} <button className="btn ghost" onClick={reload}>Reintentar</button></div>}
      {error && order && <p role="alert">No se pudo actualizar: {error}</p>}
      {!order && !error && <div className="detail-skeleton packing-skeleton" aria-label="Cargando orden" />}
      {order && detail && (
        <>
          <div className="packing-order-head">
            <span className="packing-order-courier">Courier: <CourierMark code={order.courier} name={courierName} /></span>
            <span>Comuna: {order.commune || "—"}</span>
          </div>
          <p className="packing-order-sub">
            <span>Cliente: {companyName}</span>
            <span>Origen: {originLabel(order)}</span>
            {order.claimedByName && <span>Tomada por: {order.claimedByName}</span>}
            {order.status === "packing" && order.claimedAt && <Elapsed since={order.claimedAt} />}
          </p>
          <PackingWork key={order.id} order={order} packages={detail.packages} courierName={courierName} onChanged={reload} />
        </>
      )}
    </div>
  );
}
