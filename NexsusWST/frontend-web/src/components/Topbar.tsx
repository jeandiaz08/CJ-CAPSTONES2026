"use client";

import { useEffect, useState } from "react";
import { LogOut, Menu } from "lucide-react";
import { listCompanies, type Company } from "@/services/core";
import { ROLE_LABELS, useAuth } from "@/state/auth";

export function Topbar({ onMenu }: { onMenu: () => void }) {
  const { session, logout, companyId, setCompanyId } = useAuth();
  const [companies, setCompanies] = useState<Company[]>([]);

  useEffect(() => {
    listCompanies()
      .then((r) => setCompanies(r.data))
      .catch(() => setCompanies([])); // Core caído: Órdenes sigue operando (RNF-DIS-03)
  }, []);

  if (!session) return null;
  const u = session.user;

  return (
    <header className="topbar">
      <button className="icon-btn only-mobile" onClick={onMenu} aria-label="Abrir menú">
        <Menu size={20} />
      </button>
      <label className="company-select">
        <span className="muted">Empresa</span>
        <select
          value={companyId ?? ""}
          onChange={(e) => setCompanyId(e.target.value ? Number(e.target.value) : null)}
        >
          {u.allCompanies && <option value="">Todas</option>}
          {companies.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
      </label>
      <div className="topbar-user">
        <div className="user-text">
          <strong>{u.name}</strong>
          <span className="muted">{ROLE_LABELS[u.role]}</span>
        </div>
        <button className="icon-btn" onClick={logout} title="Cerrar sesión" aria-label="Cerrar sesión">
          <LogOut size={18} />
        </button>
      </div>
    </header>
  );
}
