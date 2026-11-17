"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { Bell, CircleAlert, PackageCheck, PackageX, Truck, UserCheck, UserPlus } from "lucide-react";
import { markAllRead, useNotices, type NoticeKind } from "@/hooks/useNotificationFeed";

const ICONS: Record<NoticeKind, typeof Bell> = { incident: CircleAlert, label_ready: PackageCheck, delivered: Truck, despacho_incompleto: PackageX, profile_request: UserPlus, profile_request_reviewed: UserCheck,
  internal_profile_request: UserPlus, internal_profile_request_reviewed: UserCheck };
const time = (iso: string) => new Date(iso).toLocaleTimeString("es-CL", { hour: "2-digit", minute: "2-digit" });

// Campana: contador de avisos sin leer y lista de los últimos 20. Al abrirla se marcan leídos.
export function NotificationBell() {
  const notices = useNotices();
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const unread = notices.filter((n) => !n.read).length;

  useEffect(() => {
    if (!open) return;
    markAllRead();
    const outside = (e: PointerEvent) => { if (!root.current?.contains(e.target as Node)) setOpen(false); };
    const key = (e: KeyboardEvent) => { if (e.key === "Escape") setOpen(false); };
    document.addEventListener("pointerdown", outside);
    document.addEventListener("keydown", key);
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("keydown", key);
    };
  }, [open, notices.length]);

  return (
    <div className="notification-bell" ref={root}>
      <button type="button" className="icon-btn" aria-label={unread ? `Notificaciones, ${unread} sin leer` : "Notificaciones"} aria-expanded={open} onClick={() => setOpen(!open)}>
        <Bell size={23} aria-hidden="true" />
        {unread > 0 && <span className="notification-count" aria-hidden="true">{unread > 9 ? "9+" : unread}</span>}
      </button>
      {open && (
        <div className="notification-popover" role="region" aria-label="Avisos">
          <h2>Avisos</h2>
          {notices.length === 0 ? (
            <p className="muted">Sin avisos por ahora. Aquí aparecen las incidencias, las etiquetas disponibles, las entregas y las solicitudes de perfiles.</p>
          ) : (
            <ul>
              {notices.map((n) => {
                const Icon = ICONS[n.kind];
                return (
                  <li key={n.key} className={`notice-${n.kind}`}>
                    <Icon size={18} aria-hidden="true" />
                    {n.href
                      ? <Link href={n.href} className="notice-link" onClick={() => setOpen(false)}><strong>{n.title}</strong><span>{n.body}</span></Link>
                      : <div><strong>{n.title}</strong><span>{n.body}</span></div>}
                    <time dateTime={n.at}>{time(n.at)}</time>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
