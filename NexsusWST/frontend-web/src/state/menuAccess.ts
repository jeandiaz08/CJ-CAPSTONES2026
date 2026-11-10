// Mapa único perfil → ítems del menú y pantallas (Matriz de perfiles, hoja «Menú por perfil»).
// Ocultar un ítem no reemplaza la validación del servidor (regla 7 de la matriz).
// TODO(PV-21): cuando exista user_warehouses, filtrar alertas, incidencias, anulaciones y bandejas por las bodegas del usuario (Admin sin restricción; Cliente por empresa)
import { Boxes, ChartLine, ChartNoAxesCombined, CircleHelp, Home, Inbox, Link as LinkIcon, Package, ScanBarcode, Settings, Truck, Users, type LucideIcon } from "lucide-react";

export type Role = "admin" | "jefe_operaciones" | "analista" | "empaque" | "cliente";
export type AccessUser = { role: Role; permissions: string[] };

// yes: navegable · soon: «Próximamente», visible sin función · dispatch: solo con el permiso Despacho.
// Un perfil que no aparece no ve el ítem. Analista es un perfil futuro: por ahora solo Home.
type Level = "yes" | "soon" | "dispatch";
type Access = Partial<Record<Role, Level>>;

// Permiso «Gestionar perfiles» (Matriz v1.2). Administración clientes la ve el Administrador
// (en lectura si no lo tiene) y cualquiera con el permiso (regla 11: se valida por permiso).
export const PERM_MANAGE_PROFILES = "gestionar_perfiles";

const ALL: Access = { admin: "yes", jefe_operaciones: "yes", empaque: "yes", cliente: "yes", analista: "yes" };
const INTERNAL_AND_CLIENT: Access = { admin: "yes", jefe_operaciones: "yes", cliente: "yes" };
const INTERNAL: Access = { admin: "yes", jefe_operaciones: "yes" };
const PACKING: Access = { admin: "yes", jefe_operaciones: "yes", empaque: "yes" };
const ALL_BUT_ANALYST: Access = { admin: "yes", jefe_operaciones: "yes", empaque: "yes", cliente: "yes" };
const DISPATCH: Access = { admin: "yes", jefe_operaciones: "dispatch", empaque: "dispatch" };

// orPermission: además de los perfiles de `access`, lo ve quien tenga ese permiso.
export type MenuChild = { name: string; href: string; access: Access; orPermission?: string };
export type MenuEntry = { name: string; icon: LucideIcon; href?: string; access: Access; orPermission?: string; children?: MenuChild[] };

const CLIENT_ADMIN: Access = { admin: "yes" };

export const MENU: MenuEntry[] = [
  { name: "Home", href: "/", icon: Home, access: ALL },
  { name: "Dashboard", icon: ChartLine, access: { admin: "soon", jefe_operaciones: "soon", cliente: "soon" } },
  {
    name: "Órdenes", href: "/ordenes", icon: Inbox, access: ALL_BUT_ANALYST,
    children: [
      { name: "Todas las órdenes", href: "/ordenes/todas/listado", access: INTERNAL_AND_CLIENT },
      { name: "Empaque", href: "/ordenes/empaque", access: PACKING },
      { name: "Retiro y entrega", href: "/ordenes/retiro", access: DISPATCH },
      { name: "Registro histórico", href: "/ordenes/registro", access: INTERNAL_AND_CLIENT },
    ],
  },
  {
    name: "Inventario", href: "/inventario", icon: Boxes, access: INTERNAL_AND_CLIENT,
    children: [
      { name: "Stock", href: "/inventario/stock", access: INTERNAL_AND_CLIENT },
      { name: "Diferencias de inventario", href: "/inventario/diferencias", access: INTERNAL_AND_CLIENT },
      { name: "Inventario LPN", href: "/inventario/lpn", access: INTERNAL_AND_CLIENT },
    ],
  },
  { name: "Recepciones", icon: Package, access: { admin: "soon", jefe_operaciones: "soon" } },
  { name: "Productos", href: "/productos", icon: ScanBarcode, access: INTERNAL_AND_CLIENT },
  { name: "Facturación", icon: ChartNoAxesCombined, access: { admin: "soon", cliente: "soon" } },
  { name: "Logística", icon: Truck, access: { admin: "soon", jefe_operaciones: "soon" } },
  {
    name: "Clientes", href: "/clientes", icon: Users, access: { admin: "yes", cliente: "yes" }, orPermission: PERM_MANAGE_PROFILES,
    children: [
      { name: "Perfiles empresa", href: "/clientes/perfiles-empresa", access: { cliente: "yes" } },
      { name: "Administración clientes", href: "/clientes/administracion", access: CLIENT_ADMIN, orPermission: PERM_MANAGE_PROFILES },
    ],
  },
  // Tarea 10: solo Administrador (RF-05). También es el destino de «Ver integraciones» de una empresa.
  { name: "Integraciones", href: "/integraciones", icon: LinkIcon, access: { admin: "yes" } },
  { name: "Ayuda", icon: CircleHelp, access: { admin: "soon", jefe_operaciones: "soon", cliente: "soon" } },
  // Tarea 08: Usuarios WeStorage solo para el Administrador (en lectura sin «Gestionar perfiles»).
  {
    name: "Configuración", href: "/configuracion", icon: Settings, access: { admin: "yes" },
    children: [
      { name: "Usuarios WeStorage", href: "/configuracion/usuarios", access: { admin: "yes" } },
      { name: "Ajustes generales", href: "/configuracion/ajustes", access: { admin: "soon" } },
    ],
  },
];

// Pantallas a las que se llega por URL. Manda la ruta más larga que coincida. Lo «Próximamente»
// no tiene función, así que nadie entra. Una ruta sin regla no existe: la responde el 404 de Next.
const ROUTES: { path: string; access: Access; orPermission?: string }[] = [
  { path: "/", access: ALL },
  { path: "/ordenes", access: INTERNAL_AND_CLIENT },
  { path: "/ordenes/todas", access: INTERNAL_AND_CLIENT },
  { path: "/ordenes/todas/asignacion", access: INTERNAL },
  { path: "/ordenes/empaque", access: PACKING },
  { path: "/empaque", access: PACKING },
  { path: "/ordenes/retiro", access: DISPATCH },
  { path: "/ordenes/registro", access: INTERNAL_AND_CLIENT },
  { path: "/ordenes/historial", access: INTERNAL_AND_CLIENT },
  { path: "/olas", access: INTERNAL },
  { path: "/reglas-olas", access: INTERNAL },
  { path: "/incidencias", access: INTERNAL },
  { path: "/inventario", access: INTERNAL_AND_CLIENT },
  { path: "/productos", access: INTERNAL_AND_CLIENT },
  { path: "/clientes", access: { admin: "yes", cliente: "yes" }, orPermission: PERM_MANAGE_PROFILES },
  { path: "/clientes/perfiles-empresa", access: { cliente: "yes" } },
  { path: "/clientes/administracion", access: CLIENT_ADMIN, orPermission: PERM_MANAGE_PROFILES },
  { path: "/integraciones", access: { admin: "yes" } },
  { path: "/usuarios", access: INTERNAL },
  { path: "/configuracion", access: { admin: "yes" } },
  { path: "/configuracion/usuarios", access: { admin: "yes" } },
  { path: "/configuracion/ajustes", access: {} },
  ...["/dashboard", "/recepciones", "/facturacion", "/logistica", "/ayuda"].map((path) => ({ path, access: {} })),
];

function allowed(access: Access, user: AccessUser, orPermission?: string) {
  const level = access[user.role];
  if (orPermission && user.permissions.includes(orPermission)) return true;
  return level === "yes" || level === "soon" || (level === "dispatch" && user.permissions.includes("despacho"));
}

export type VisibleChild = MenuChild & { disabled: boolean };
export type VisibleEntry = Omit<MenuEntry, "children"> & { disabled: boolean; children?: VisibleChild[] };

export function visibleMenu(user: AccessUser): VisibleEntry[] {
  return MENU.filter((entry) => allowed(entry.access, user, entry.orPermission)).map((entry) => ({
    ...entry,
    disabled: entry.access[user.role] === "soon",
    children: entry.children?.filter((child) => allowed(child.access, user, child.orPermission))
      .map((child) => ({ ...child, disabled: child.access[user.role] === "soon" && !(child.orPermission && user.permissions.includes(child.orPermission)) })),
  }));
}

export function canAccessPath(user: AccessUser, pathname: string) {
  const rule = ROUTES.filter(({ path }) => pathname === path || (path !== "/" && pathname.startsWith(`${path}/`))).sort((a, b) => b.path.length - a.path.length)[0];
  return !rule || (rule.access[user.role] !== "soon" && allowed(rule.access, user, rule.orPermission));
}
