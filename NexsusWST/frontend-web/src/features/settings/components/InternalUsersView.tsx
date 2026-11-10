"use client";

import { NotificationBell } from "@/components/NotificationBell";
import { PageHeader } from "@/components/PageHeader";
import { Tabs } from "@/components/Tabs";
import { ProfilesTab } from "@/features/clients";

const BASE = "/configuracion/usuarios";

// Configuración › Usuarios WeStorage (Tarea 08, H-02). Perfiles internos de WeStorage con los mismos
// componentes de Clientes en el alcance «internal». Solo la ve el Administrador; sin «Gestionar
// perfiles» queda en lectura. La pestaña Solicitudes llega en la Fase 2.
export function InternalUsersView() {
  return (
    <div className="orders-view clients-view">
      <PageHeader title="Usuarios WeStorage" actions={<NotificationBell />} />
      <Tabs label="Usuarios WeStorage" items={[{ href: `${BASE}?tab=perfiles`, label: "Perfiles", active: true }]} />
      <ProfilesTab scope="internal" tab="perfiles" />
    </div>
  );
}
