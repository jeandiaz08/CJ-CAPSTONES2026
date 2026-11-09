"use client";

import { useSearchParams } from "next/navigation";
import { NotificationBell } from "@/components/NotificationBell";
import { PageHeader } from "@/components/PageHeader";
import { Tabs } from "@/components/Tabs";

const BASE = "/clientes/perfiles-empresa";

// Clientes › Perfiles empresa (perfil Cliente, Tarea 07 Fase 5): «Mis perfiles» y «Mis solicitudes».
// El Cliente ve solo su empresa; lo filtra el servidor.
export function CompanyProfilesView() {
  const params = useSearchParams();
  const requests = params.get("tab") === "solicitudes";
  return (
    <div className="orders-view clients-view">
      <PageHeader title="Clientes" actions={<NotificationBell />} />
      <Tabs label="Perfiles empresa" items={[
        { href: BASE, label: "Mis perfiles", active: !requests },
        { href: `${BASE}?tab=solicitudes`, label: "Mis solicitudes", active: requests },
      ]} />
      <section className="card placeholder" aria-live="polite">
        <p className="muted">En construcción.</p>
      </section>
    </div>
  );
}
