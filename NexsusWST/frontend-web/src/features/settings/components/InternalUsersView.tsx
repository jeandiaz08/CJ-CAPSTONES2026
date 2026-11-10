"use client";

import { useSearchParams } from "next/navigation";
import { NotificationBell } from "@/components/NotificationBell";
import { PageHeader } from "@/components/PageHeader";
import { Tabs } from "@/components/Tabs";
import { ProfilesTab, RequestsTab } from "@/features/clients";

const BASE = "/configuracion/usuarios";
const TABS = [
  { key: "perfiles", label: "Perfiles" },
  { key: "solicitudes", label: "Solicitudes" },
] as const;
type TabKey = (typeof TABS)[number]["key"];

// Configuración › Usuarios WeStorage (Tarea 08, H-02). Perfiles internos de WeStorage y sus
// solicitudes, con los mismos componentes de Clientes en el alcance «internal». Solo la ve el
// Administrador; sin «Gestionar perfiles» queda en lectura y puede solicitar (perfil nuevo,
// activación o desactivación), y quien tiene el permiso aprueba o rechaza.
export function InternalUsersView() {
  const raw = useSearchParams().get("tab");
  const tab: TabKey = TABS.some((t) => t.key === raw) ? (raw as TabKey) : "perfiles";
  return (
    <div className="orders-view clients-view">
      <PageHeader title="Usuarios WeStorage" actions={<NotificationBell />} />
      <Tabs label="Usuarios WeStorage" items={TABS.map((t) => ({ href: `${BASE}?tab=${t.key}`, label: t.label, active: t.key === tab }))} />
      {tab === "solicitudes" ? <RequestsTab mode="admin" scope="internal" /> : <ProfilesTab scope="internal" tab="perfiles" />}
    </div>
  );
}
