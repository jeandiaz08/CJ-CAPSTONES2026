"use client";

import { useSearchParams } from "next/navigation";
import { NotificationBell } from "@/components/NotificationBell";
import { PageHeader } from "@/components/PageHeader";
import { Tabs } from "@/components/Tabs";
import { CompaniesTab } from "./CompaniesTab";

const BASE = "/clientes/administracion";
const TABS = [
  { key: "perfiles", label: "Perfiles" },
  { key: "empresas", label: "Empresas" },
  { key: "solicitudes", label: "Solicitudes" },
] as const;
type TabKey = (typeof TABS)[number]["key"];

// Clientes › Administración clientes (Tarea 07). Solo lo de las empresas cliente:
// perfiles de clientes, empresas y sus solicitudes (regla 10 de la matriz).
// Sin «Gestionar perfiles» queda en lectura. Empresas: Fase 2; Perfiles y Solicitudes: fases 3 y 4.
export function ClientAdminView() {
  const params = useSearchParams();
  const raw = params.get("tab");
  const tab: TabKey = TABS.some((t) => t.key === raw) ? (raw as TabKey) : "perfiles";
  return (
    <div className="orders-view clients-view">
      <PageHeader title="Administración clientes" actions={<NotificationBell />} />
      <Tabs label="Administración clientes" items={TABS.map((t) => ({ href: `${BASE}?tab=${t.key}`, label: t.label, active: t.key === tab }))} />
      {tab === "empresas" ? <CompaniesTab /> : (
        <section className="card placeholder" aria-live="polite">
          <p className="muted">En construcción.</p>
        </section>
      )}
    </div>
  );
}
