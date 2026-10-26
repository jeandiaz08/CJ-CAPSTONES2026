
"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  ChevronDown,
  ChevronRight,
  X,
} from "lucide-react";

import { MENU } from "@/features/navigation/menu";
import { useAuth } from "@/state/auth";

export function Sidebar({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const pathname = usePathname();
  const { can } = useAuth();

  const [expanded, setExpanded] = useState<string[]>([]);

  // Comprueba si una ruta está activa.
  const isActive = (href: string) => {
    if (href === "/") return pathname === "/";
    return pathname === href || pathname.startsWith(`${href}/`);
  };

  // Abre automáticamente el módulo que contiene la ruta actual.
  useEffect(() => {
    const activeParents = MENU.flatMap((section) =>
      section.items
        .filter(
          (item) =>
            item.children?.some((child) => isActive(child.href)) &&
            can({
              roles: item.roles,
              permission: item.permission,
            })
        )
        .map((item) => item.href)
    );

    setExpanded((previous) =>
      Array.from(new Set([...previous, ...activeParents]))
    );
  }, [pathname, can]);

  const toggleSubmenu = (href: string) => {
    setExpanded((previous) =>
      previous.includes(href)
        ? previous.filter((item) => item !== href)
        : [...previous, href]
    );
  };

  return (
    <>
      <div
        className={`sidebar-backdrop ${open ? "show" : ""}`}
        onClick={onClose}
      />

      <aside
        className={`sidebar ${open ? "open" : ""}`}
        aria-label="Menú principal"
      >
        <div className="sidebar-brand">
          <span className="brand-mark">N</span>

          <span className="brand-name">
            <strong>WeStorage</strong>
            <small>NexusWST</small>
          </span>

          <button
            className="icon-btn only-mobile"
            onClick={onClose}
            aria-label="Cerrar menú"
          >
            <X size={18} />
          </button>
        </div>

        <nav className="sidebar-nav">
          {MENU.map((section, sectionIndex) => {
            const items = section.items
              .map((item) => {
                if (
                  !can({
                    roles: item.roles,
                    permission: item.permission,
                  })
                ) {
                  return null;
                }

                const children = (item.children ?? []).filter((child) =>
                  can({
                    roles: child.roles,
                    permission: child.permission,
                  })
                );

                return { ...item, children };
              })
              .filter((item) => item !== null);

            if (items.length === 0) return null;

            return (
              <div
                key={section.title || `section-${sectionIndex}`}
                className="nav-section"
              >
                {section.title && (
                  <p className="nav-title">{section.title}</p>
                )}

                {items.map((item) => {
                  const Icon = item.icon;
                  const hasChildren = item.children.length > 0;
                  const isExpanded = expanded.includes(item.href);

                  const active =
                    isActive(item.href) ||
                    item.children.some((child) =>
                      isActive(child.href)
                    );

                  return (
                    <div key={item.href} className="nav-item">
                      <div
                        className={`nav-link-wrapper ${
                          active ? "active" : ""
                        }`}
                      >
                        <Link
                          href={item.href}
                          className="nav-link"
                          onClick={onClose}
                          aria-current={
                            pathname === item.href ? "page" : undefined
                          }
                        >
                          <Icon size={18} strokeWidth={1.8} />
                          <span>{item.label}</span>
                        </Link>

                        {hasChildren && (
                          <button
                            type="button"
                            className="submenu-toggle"
                            aria-label={`${
                              isExpanded ? "Cerrar" : "Abrir"
                            } ${item.label}`}
                            aria-expanded={isExpanded}
                            onClick={() => toggleSubmenu(item.href)}
                          >
                            {isExpanded ? (
                              <ChevronDown size={17} />
                            ) : (
                              <ChevronRight size={17} />
                            )}
                          </button>
                        )}
                      </div>

                      {hasChildren && isExpanded && (
                        <div className="nav-submenu">
                          {item.children.map((child) => {
                            const ChildIcon = child.icon;
                            const childActive =
                              pathname === child.href;

                            return (
                              <Link
                                key={child.href}
                                href={child.href}
                                className={`nav-sublink ${
                                  childActive ? "active" : ""
                                }`}
                                onClick={onClose}
                              >
                                <span className="submenu-dot">•</span>
                                <span>{child.label}</span>
                              </Link>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            );
          })}
        </nav>
      </aside>
    </>
  );
}
