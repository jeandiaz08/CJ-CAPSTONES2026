"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { Badge } from "@/components/StatusBadge";
import { listIntegrations } from "@/services/integrations";
import { useOrderResource } from "@/features/orders/hooks/useOrderResources";
import { PROVIDERS, SECTIONS, STATUS_BADGE, type Provider } from "../providers";
import { ProviderLogo } from "./ProviderLogo";

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

// Pestaña «Integraciones»: una tarjeta por proveedor de la lista única (providers.ts). Las de
// proveedores soportados resumen sus conexiones (GET /v1/integrations agrupado); las demás son
// fijas. Las tarjetas no llevan interruptor: activar y desactivar es por conexión (pestaña Clientes).
export function ProvidersTab() {
  const [query, setQuery] = useState("");
  const { data, error, loading } = useOrderResource(listIntegrations, "integrations");
  const counts = useMemo(() => {
    const map = new Map<string, { active: number; inactive: number }>();
    for (const i of data?.data ?? []) {
      const key = `${i.kind}:${i.provider}`;
      const c = map.get(key) ?? { active: 0, inactive: 0 };
      if (i.active) c.active++; else c.inactive++;
      map.set(key, c);
    }
    return map;
  }, [data]);
  const term = query.trim().toLowerCase();
  const cards = PROVIDERS.filter((p) => p.card !== false && (!term || p.name.toLowerCase().includes(term)));

  const summary = (p: Provider) => {
    if (error) return <span className="error">No se pudo cargar</span>;
    if (loading && !data) return <span className="muted">Cargando…</span>;
    const c = counts.get(`${p.kind}:${p.code}`);
    if (!c) return <span className="muted">Sin conexiones</span>;
    return (
      <span>
        {c.active > 0 && <span className="integration-ok">{plural(c.active, "conexión activa", "conexiones activas")}</span>}
        {c.active > 0 && c.inactive > 0 && " · "}
        {c.inactive > 0 && <span className="muted">{plural(c.inactive, "inactiva", "inactivas")}</span>}
      </span>
    );
  };

  return (
    <>
      <h2 className="section-title">Canales, couriers e integraciones</h2>
      <div className="orders-toolbar clients-toolbar">
        <label className="orders-search">
          <span className="sr-only">Buscar canal, courier o integración</span>
          <input placeholder="Buscar canal, courier o integración" value={query} onChange={(e) => setQuery(e.target.value)} />
        </label>
      </div>
      {error && <p role="alert" className="error">No se pudieron cargar las conexiones: {error}</p>}
      {cards.length === 0 && <p className="pad">Ningún proveedor coincide con la búsqueda</p>}
      {SECTIONS.map(({ kind, title }) => {
        const list = cards.filter((p) => p.kind === kind);
        if (list.length === 0) return null;
        return (
          <section key={kind} aria-label={title}>
            <h2 className="section-title">{title}</h2>
            <ul className="integration-cards">
              {list.map((p) => {
                const badge = STATUS_BADGE[p.status];
                const live = p.status === "supported" || p.status === "server";
                return (
                  <li key={`${p.kind}:${p.code}`} className={`integration-card${p.status === "soon" ? " is-soon" : ""}`}>
                    <header><ProviderLogo provider={p} /><Badge tone={badge.tone} label={badge.label} /></header>
                    <h3>{p.name}</h3>
                    <p>{p.description}</p>
                    <footer>
                      {p.status === "supported"
                        ? <>{summary(p)}<Link className="link" href={`/integraciones?tab=clientes&tipo=${p.kind}&proveedor=${p.code}`} aria-label={`Ver conexiones de ${p.name}`}>Ver conexiones</Link></>
                        : <span className="muted">{p.note}</span>}
                      {/* TODO(api): el backend no expone la última sincronización; no se inventa el dato. */}
                      {live && <small className="muted">Sincronizado: —</small>}
                    </footer>
                  </li>
                );
              })}
            </ul>
          </section>
        );
      })}
    </>
  );
}
