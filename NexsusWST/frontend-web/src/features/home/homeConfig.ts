// Home por perfil (Tarea 11): qué tarjetas y qué accesos rápidos ve cada perfil.
// Cada número sale de un endpoint que ya existe y el servidor lo filtra por perfil y empresa;
// aquí solo se decide qué mostrar. Los accesos pasan además por canAccessPath().
import { CircleAlert, ClipboardList, History, Inbox, Link as LinkIcon, PackageCheck, PenLine, ScanBarcode, Truck, UserPlus, Users, type LucideIcon } from "lucide-react";
import type { OrderStatus } from "@/services/orders";
import { canAccessPath, PERM_MANAGE_PROFILES, type AccessUser } from "@/state/menuAccess";

// De dónde sale el número: suma de estados del resumen de órdenes, integraciones activas/total
// o solicitudes de perfiles pendientes.
export type CardSource = { kind: "orders"; statuses: OrderStatus[] } | { kind: "integrations" } | { kind: "requests" };
export type HomeCard = { key: string; title: string; caption: string; linkLabel: string; href: string; icon: LucideIcon; source: CardSource };
export type QuickLink = { label: string; href: string; icon: LucideIcon };

const LISTING = "/ordenes/todas/listado";
const byStatus = (...statuses: OrderStatus[]) => `${LISTING}?status=${statuses.join(",")}`;

// «En curso» del Cliente: todo menos Entregada al courier, Anulada y Cancelada.
export const IN_PROGRESS: OrderStatus[] = ["pending", "validating", "validated", "picking", "picked", "packing", "ready_pickup", "incident"];

const orders = (key: string, title: string, caption: string, statuses: OrderStatus[], icon: LucideIcon, href = byStatus(...statuses), linkLabel = "Ver órdenes"): HomeCard =>
  ({ key, title, caption, linkLabel, href, icon, source: { kind: "orders", statuses } });

const INCIDENTS = (title: string) => orders("incident", title, "Incidencias abiertas", ["incident"], CircleAlert);

/** Tarjetas del perfil, máximo 3. El color lo da la posición (naranjo, azul, verde). */
export function homeCards(user: AccessUser): HomeCard[] {
  const dispatch = user.permissions.includes("despacho");
  switch (user.role) {
    case "admin":
      return [
        orders("pending", "Órdenes pendientes", "En estado Pendiente", ["pending"], Inbox),
        { key: "integrations", title: "Integraciones activas", caption: "Conexiones activas", linkLabel: "Ver integraciones", href: "/integraciones", icon: LinkIcon, source: { kind: "integrations" } },
        user.permissions.includes(PERM_MANAGE_PROFILES)
          ? { key: "requests", title: "Solicitudes de perfiles pendientes", caption: "Solicitudes por responder", linkLabel: "Ver solicitudes", href: "/clientes/administracion?tab=solicitudes", icon: UserPlus, source: { kind: "requests" } }
          : INCIDENTS("Incidencias abiertas"),
      ];
    case "jefe_operaciones":
      return [
        orders("to-validate", "Órdenes por validar", "Pendientes y En validación", ["pending", "validating"], Inbox),
        INCIDENTS("Órdenes con incidencia"),
        orders("ready", "Listas para retiro", "Esperan al courier", ["ready_pickup"], Truck),
      ];
    case "empaque":
      return [
        orders("to-pack", "Por empacar", "Órdenes preparadas", ["picked"], PackageCheck, "/ordenes/empaque", "Ver empaque"),
        ...(dispatch ? [orders("ready", "Listas para retiro", "Esperan al courier", ["ready_pickup"], Truck, "/ordenes/retiro", "Ver retiro y entrega")] : []),
      ];
    case "cliente":
      return [
        orders("in-progress", "Mis órdenes en curso", "Órdenes en curso", IN_PROGRESS, Inbox),
        orders("incident", "Con incidencia", "Órdenes con incidencia", ["incident"], CircleAlert),
        { key: "requests", title: "Solicitudes pendientes de tu empresa", caption: "Solicitudes por responder", linkLabel: "Ver solicitudes", href: "/clientes/perfiles-empresa?tab=solicitudes", icon: UserPlus, source: { kind: "requests" } },
      ];
    default:
      return []; // Analista (perfil futuro): solo el saludo y sus recordatorios.
  }
}

const QUICK: Partial<Record<AccessUser["role"], QuickLink[]>> = {
  admin: [
    { label: "Todas las órdenes", href: LISTING, icon: Inbox },
    { label: "Órdenes manuales", href: `${LISTING}?origin=manual`, icon: PenLine },
    { label: "Solicitudes", href: "/clientes/administracion?tab=solicitudes", icon: UserPlus },
    { label: "Registro de acciones", href: "/ordenes/registro?tab=acciones", icon: ClipboardList },
    { label: "Integraciones", href: "/integraciones", icon: LinkIcon },
  ],
  jefe_operaciones: [
    { label: "Todas las órdenes", href: LISTING, icon: Inbox },
    { label: "Órdenes manuales", href: `${LISTING}?origin=manual`, icon: PenLine },
    { label: "Retiro y entrega", href: "/ordenes/retiro", icon: Truck },
    { label: "Registro histórico", href: "/ordenes/registro", icon: History },
  ],
  empaque: [
    { label: "Empaque", href: "/ordenes/empaque", icon: PackageCheck },
    { label: "Retiro y entrega", href: "/ordenes/retiro", icon: Truck },
  ],
  cliente: [
    { label: "Mis órdenes", href: LISTING, icon: Inbox },
    { label: "Perfiles empresa", href: "/clientes/perfiles-empresa", icon: Users },
    { label: "Mis solicitudes", href: "/clientes/perfiles-empresa?tab=solicitudes", icon: UserPlus },
    { label: "Productos", href: "/productos", icon: ScanBarcode },
  ],
};

/** Accesos rápidos: solo las pantallas que ese perfil puede abrir (menuAccess). */
export function quickLinks(user: AccessUser): QuickLink[] {
  return (QUICK[user.role] ?? []).filter((link) => canAccessPath(user, link.href.split("?")[0]));
}

// --- Fechas de los recordatorios (día local del navegador) -----------------------------------
const pad = (n: number) => String(n).padStart(2, "0");
const toIso = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
const dayNumber = (iso: string) => {
  const [y, m, d] = iso.split("-").map(Number);
  return Math.round(Date.UTC(y, m - 1, d) / 86_400_000);
};

/** Fecha AAAA-MM-DD a `days` días de hoy. */
export function dateInDays(days: number, today = new Date()): string {
  return toIso(new Date(today.getFullYear(), today.getMonth(), today.getDate() + days));
}

/** «Hoy», «Mañana», «Ayer», «En 3 días», «Hace 2 días». Sin fecha, vacío. */
export function dueLabel(dueDate: string | null, today = new Date()): string {
  if (!dueDate) return "";
  const diff = dayNumber(dueDate.slice(0, 10)) - dayNumber(toIso(today));
  if (diff === 0) return "Hoy";
  if (diff === 1) return "Mañana";
  if (diff === -1) return "Ayer";
  return diff > 0 ? `En ${diff} días` : `Hace ${-diff} días`;
}
