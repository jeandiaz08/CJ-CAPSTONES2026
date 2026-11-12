"use client";

import { useCallback, useState } from "react";
import Link from "next/link";
import { NotificationBell } from "@/components/NotificationBell";
import { PageHeader } from "@/components/PageHeader";
import { LiveBadge } from "@/components/StatusBadge";
import { useOrdersSocket } from "@/hooks/useOrdersSocket";
import { useAuth } from "@/state/auth";
import { homeCards, quickLinks } from "../homeConfig";
import { HomeCards } from "./HomeCards";
import { Reminders } from "./Reminders";

// Home por perfil (Tarea 11): saludo, tarjetas, accesos rápidos y recordatorios.
export function HomeView() {
  const { session, companyId } = useAuth();
  const [refresh, setRefresh] = useState(0);
  const live = useOrdersSocket(useCallback(() => setRefresh((n) => n + 1), []));
  const user = session?.user;
  if (!user) return null;
  const cards = homeCards(user);
  const links = quickLinks(user);

  return (
    <div className="home-view">
      <PageHeader title="Home" actions={<><LiveBadge live={live} /><NotificationBell /></>} />
      <div className="home-greeting">
        <p>Bienvenido a NexusWST</p>
        <h2>Hola, {user.name.split(" ")[0]} <span aria-hidden="true">👋</span></h2>
        <p>Aquí tienes un resumen de tu operación de hoy</p>
      </div>

      <HomeCards cards={cards} companyId={companyId} refresh={refresh} />

      {links.length > 0 && (
        <section aria-labelledby="quick-title">
          <h2 id="quick-title" className="home-section-title">Acceso rápido</h2>
          <p className="muted">Llega más rápido donde lo necesites</p>
          <nav className="home-quick" aria-label="Acceso rápido">
            {links.map(({ label, href, icon: Icon }) => (
              <Link key={label} href={href}><Icon size={44} strokeWidth={1.5} aria-hidden="true" />{label}</Link>
            ))}
          </nav>
        </section>
      )}

      <Reminders />
    </div>
  );
}
