"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { NotificationBell } from "@/components/NotificationBell";
import { PageHeader } from "@/components/PageHeader";
import { LiveBadge } from "@/components/StatusBadge";
import { Tabs } from "@/components/Tabs";
import { useAuth } from "@/state/auth";
import { OrdersRegistry } from "./OrdersRegistry";

// Órdenes → Registro histórico (Tarea 06). El Cliente ve solo «Registro órdenes» de su empresa
// (lo filtra el servidor); Administrador y Jefe de Operaciones ven también «Registro de acciones».
export function RegistryView() {
  const { session } = useAuth();
  const params = useSearchParams();
  const [live, setLive] = useState(false);
  const internal = session?.user.role === "admin" || session?.user.role === "jefe_operaciones";
  const actions = internal && params.get("tab") === "acciones";
  return (
    <div className="orders-view registry-view">
      <PageHeader title="Órdenes" actions={<><LiveBadge live={live} /><NotificationBell /></>} />
      <Tabs items={[
        { href: "/ordenes/registro", label: "Registro órdenes", active: !actions },
        ...(internal ? [{ href: "/ordenes/registro?tab=acciones", label: "Registro de acciones", active: actions }] : []),
      ]} />
      {/* TODO(tarea-06-fase-3): pestaña «Registro de acciones» con GET /v1/activity. */}
      {actions ? <p className="detail-empty">Registro de acciones · Próximamente</p> : <OrdersRegistry onLive={setLive} />}
    </div>
  );
}
