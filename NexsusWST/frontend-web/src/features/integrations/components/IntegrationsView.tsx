"use client";

import { useSearchParams } from "next/navigation";
import { NotificationBell } from "@/components/NotificationBell";
import { PageHeader } from "@/components/PageHeader";
import { Tabs } from "@/components/Tabs";
import { ConnectionsTab } from "./ConnectionsTab";
import { ProvidersTab } from "./ProvidersTab";

const BASE = "/integraciones";

// Integraciones (RF-05, H-06; Tarea 10). Solo Administrador. Dos pestañas: «Integraciones»
// (tarjetas por proveedor) y «Clientes» (conexiones por empresa). Un enlace con ?empresa= o
// ?proveedor= y sin ?tab= abre «Clientes» ya filtrada («Ver integraciones» de una empresa).
export function IntegrationsView() {
  const params = useSearchParams();
  const raw = params.get("tab");
  const tab = raw === "clientes" || raw === "integraciones" ? raw : params.get("empresa") || params.get("proveedor") ? "clientes" : "integraciones";
  return (
    <div className="orders-view clients-view integrations-view">
      <PageHeader title="Integraciones" actions={<NotificationBell />} />
      <Tabs label="Integraciones" items={[
        { href: `${BASE}?tab=integraciones`, label: "Integraciones", active: tab === "integraciones" },
        { href: `${BASE}?tab=clientes`, label: "Clientes", active: tab === "clientes" },
      ]} />
      {tab === "clientes" ? <ConnectionsTab /> : <ProvidersTab />}
    </div>
  );
}
