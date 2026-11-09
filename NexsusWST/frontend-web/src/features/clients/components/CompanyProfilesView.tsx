"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { NotificationBell } from "@/components/NotificationBell";
import { PageHeader } from "@/components/PageHeader";
import { Tabs } from "@/components/Tabs";
import { listCompanies } from "@/services/core";
import { useAuth } from "@/state/auth";
import { useOrderResource } from "@/features/orders/hooks/useOrderResources";
import { MyProfilesTab } from "./MyProfilesTab";
import { RequestsTab } from "./RequestsTab";

const BASE = "/clientes/perfiles-empresa";

// Clientes › Perfiles empresa (perfil Cliente, Tarea 07 Fase 5): «Mis perfiles» y «Mis solicitudes».
// El Cliente ve solo su empresa; lo filtra el servidor.
export function CompanyProfilesView() {
  const params = useSearchParams();
  const { session } = useAuth();
  const requests = params.get("tab") === "solicitudes";
  const [sent, setSent] = useState(0);
  const companies = useOrderResource(listCompanies, "client-company");
  const companyId = session?.user.companies[0];
  const companyName = companies.data?.data.find((c) => c.id === companyId)?.name ?? companies.data?.data[0]?.name ?? "";
  return (
    <div className="orders-view clients-view">
      <PageHeader title="Clientes" actions={<NotificationBell />} />
      <Tabs label="Perfiles empresa" items={[
        { href: BASE, label: "Mis perfiles", active: !requests },
        { href: `${BASE}?tab=solicitudes`, label: "Mis solicitudes", active: requests },
      ]} />
      {requests
        ? <RequestsTab mode="client" refreshKey={sent} />
        : <MyProfilesTab companyName={companyName} onRequested={() => setSent((v) => v + 1)} />}
    </div>
  );
}
