
import {
  Boxes,
  Building2,
  LayoutDashboard,
  ListOrdered,
  PackageCheck,
  PackagePlus,
  Plug,
  Receipt,
  Settings2,
  TriangleAlert,
  Truck,
  Users,
  Warehouse,
  Waves,
  type LucideIcon,
} from "lucide-react";

import type { Role } from "@/state/auth";

export type MenuItem = {
  href: string;
  label: string;
  icon: LucideIcon;
  roles?: Role[];
  permission?: string;
  rf?: string;
  children?: MenuItem[];
};

export type MenuSection = {
  title: string;
  items: MenuItem[];
};

const INTERNOS: Role[] = ["admin", "jefe_operaciones"];

export const MENU: MenuSection[] = [
  {
    title: "Principal",
    items: [
      {
        href: "/",
        label: "Inicio",
        icon: LayoutDashboard,
      },
    ],
  },
  {
    title: "Operación",
    items: [
      {
        href: "/ordenes",
        label: "Órdenes",
        icon: ListOrdered,
        rf: "RF-08 a RF-15",
        children: [
          {
            href: "/ordenes",
            label: "Todas las órdenes",
            icon: ListOrdered,
          },
          {
            href: "/ordenes/pendientes",
            label: "Órdenes pendientes",
            icon: ListOrdered,
          },
          {
            href: "/ordenes/preparacion",
            label: "Órdenes en preparación",
            icon: PackageCheck,
          },
          {
            href: "/ordenes/historial",
            label: "Historial de órdenes",
            icon: Receipt,
          },
        ],
      },
      {
        href: "/olas",
        label: "Asignación a olas",
        icon: Waves,
        roles: INTERNOS,
        rf: "RF-16, RF-17",
        children: [
          {
            href: "olas",
            label: "Olas general",
            icon: ListOrdered,
          }
        ]
      },
      {
        href: "/empaque",
        label: "Empaque",
        icon: PackageCheck,
        roles: [...INTERNOS, "empaque"],
        rf: "RF-18 a RF-22",
      },
      {
        href: "/retiro",
        label: "Retiro courier",
        icon: Truck,
        roles: [...INTERNOS, "empaque"],
        rf: "RF-23, RF-24",
      },
      {
        href: "/incidencias",
        label: "Incidencias",
        icon: TriangleAlert,
        roles: INTERNOS,
        rf: "RF-15, RF-20, RF-27",
      },
    ],
  },
  {
    title: "Maestros",
    items: [
      {
        href: "/clientes",
        label: "Clientes",
        icon: Building2,
        roles: [...INTERNOS, "analista"],
        rf: "RF-04, RF-05",
      },
      {
        href: "/productos",
        label: "Productos",
        icon: Boxes,
        rf: "RF-06, RF-07",
      },
    ],
  },
  {
    title: "Bodega",
    items: [
      {
        href: "/inventario",
        label: "Inventario",
        icon: Warehouse,
        roles: [...INTERNOS, "analista", "cliente"],
      },
      {
        href: "/recepciones",
        label: "Recepciones",
        icon: PackagePlus,
        roles: INTERNOS,
      },
      {
        href: "/facturacion",
        label: "Facturación",
        icon: Receipt,
        roles: ["admin"],
      },
    ],
  },
  {
    title: "Administración",
    items: [
      {
        href: "/usuarios",
        label: "Usuarios y roles",
        icon: Users,
        roles: ["admin"],
        rf: "RF-02",
      },
      {
        href: "/reglas-olas",
        label: "Reglas de olas",
        icon: Settings2,
        roles: INTERNOS,
        rf: "RF-17",
      },
      {
        href: "/integraciones",
        label: "Integraciones",
        icon: Plug,
        roles: ["admin"],
        rf: "RF-05, RF-23",
      },
    ],
  },
];
