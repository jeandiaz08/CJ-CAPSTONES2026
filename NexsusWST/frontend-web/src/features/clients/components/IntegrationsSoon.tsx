"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { NotificationBell } from "@/components/NotificationBell";
import { PageHeader } from "@/components/PageHeader";
import { listCompanies } from "@/services/core";

// Integraciones (RF-05): página de relleno («Próximamente»). Es el destino de «Ver integraciones»
// en Clientes › Empresas, que llega con ?empresa={id}; aquí solo se muestra qué empresa se filtró.
export function IntegrationsSoon() {
  const params = useSearchParams();
  const raw = params.get("empresa");
  const companyId = raw && /^\d+$/.test(raw) ? Number(raw) : null;
  const [company, setCompany] = useState<{ id: number; name: string | null } | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (companyId == null) return;
    let alive = true;
    listCompanies()
      .then((r) => {
        if (alive) setCompany({ id: companyId, name: r.data.find((c) => c.id === companyId)?.name ?? null });
      })
      .catch((e: Error) => {
        if (alive) setError(e.message);
      });
    return () => {
      alive = false;
    };
  }, [companyId]);

  const loaded = company?.id === companyId ? company : null;
  return (
    <>
      <PageHeader title="Integraciones" actions={<NotificationBell />} />
      <section className="card placeholder" aria-live="polite">
        {companyId != null && (
          <p>
            <span className="muted">Empresa: </span>
            {error ? <span className="error">{error}</span> : !loaded ? "Cargando…" : (loaded.name ?? "No existe o no tienes acceso a esta empresa.")}
          </p>
        )}
        <p><strong>Próximamente.</strong> Aquí se configurarán las integraciones por empresa: canales, transporte y notificaciones.</p>
      </section>
    </>
  );
}
