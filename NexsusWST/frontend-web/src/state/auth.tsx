"use client";

// Sesión del usuario (RF-01, DAS v1.1 §4.5).
//  - Auth.js inicia la sesión con Keycloak y guarda los tokens en una cookie httpOnly cifrada:
//    el navegador nunca los ve. El BFF los renueva antes de que venzan.
//  - El PERFIL (rol, permisos, empresas, módulos) sale del Core: /v1/auth/me.
import { signIn } from "next-auth/react";
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { clearProductCache } from "@/services/orderReference";
import type { Role } from "./menuAccess";

export type { Role } from "./menuAccess";

export type ModuleAccess = { module: string; actions: string[] };

export type SessionUser = {
  id: number;
  email: string;
  name: string;
  role: Role;
  permissions: string[];
  companies: number[];
  allCompanies: boolean;
  modules: ModuleAccess[];
};

export type Session = { user: SessionUser };

type AuthState = {
  session: Session | null;
  ready: boolean;
  /** Hay sesión de Keycloak, pero el perfil de Nexus no se pudo usar (deshabilitado, sin perfil, Core caído). */
  profileError: string | null;
  /** Empresa elegida en la barra superior (null = todas, solo personal interno) */
  companyId: number | null;
  setCompanyId: (id: number | null) => void;
  /** Redirige al inicio de sesión de Keycloak. */
  login: (returnTo?: string) => Promise<void>;
  logout: () => void;
  can: (opts: { roles?: Role[]; permission?: string; module?: string; action?: string }) => boolean;
};

const PROFILE_REFRESH_MS = 9 * 60 * 1000; // recoge cambios de rol o empresas, y mantiene viva la sesión
const NO_SESSION_CODES = ["session_expired", "unauthorized", "token_expired"];
const AuthContext = createContext<AuthState | null>(null);

class ProfileError extends Error {
  constructor(message: string, public status: number, public code?: string) {
    super(message);
  }
}

async function fetchProfile(): Promise<SessionUser> {
  const res = await fetch("/bff/core/v1/auth/me", { cache: "no-store" });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new ProfileError(body.message ?? `No se pudo leer tu perfil (HTTP ${res.status})`, res.status, body.error);
  return {
    id: body.id,
    email: body.email,
    name: body.name,
    role: body.role,
    permissions: body.permissions ?? [],
    companies: body.companies ?? [],
    allCompanies: !!body.allCompanies,
    modules: body.modules ?? [],
  };
}

const sessionEnded = (e: unknown) => e instanceof ProfileError && e.status === 401 && NO_SESSION_CODES.includes(e.code ?? "");

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<Session | null>(null);
  const [ready, setReady] = useState(false);
  const [profileError, setProfileError] = useState<string | null>(null);
  const [companyId, setCompanyId] = useState<number | null>(null);

  const apply = useCallback((user: SessionUser | null) => {
    if (!user) clearProductCache();
    setSession(user ? { user } : null);
    if (user && !user.allCompanies && user.companies.length > 0) {
      setCompanyId((prev) => (prev != null && user.companies.includes(prev) ? prev : user.companies[0]));
    }
  }, []);

  useEffect(() => {
    let current = true;
    fetchProfile()
      .then((user) => { if (current) apply(user); })
      .catch((e) => {
        if (!current) return;
        apply(null);
        if (!sessionEnded(e)) setProfileError((e as Error).message);
      })
      .finally(() => { if (current) setReady(true); });
    return () => { current = false; };
  }, [apply]);

  useEffect(() => {
    if (!session) return;
    const timer = setInterval(() => {
      fetchProfile().then(apply).catch((e) => {
        if (sessionEnded(e)) window.location.replace("/login");
        else if (e instanceof ProfileError && (e.code === "user_disabled" || e.code === "company_disabled")) {
          apply(null);
          setProfileError(e.message);
        }
        // Core caído: se mantiene el perfil anterior (continuidad de Órdenes).
      });
    }, PROFILE_REFRESH_MS);
    return () => clearInterval(timer);
  }, [session, apply]);

  const login = useCallback(async (returnTo = "/") => {
    await signIn("keycloak", { redirectTo: returnTo });
  }, []);

  const logout = useCallback(() => {
    clearProductCache();
    try {
      Object.keys(sessionStorage).filter((k) => k.startsWith("nexus:")).forEach((k) => sessionStorage.removeItem(k));
    } catch {
      // Sin almacenamiento no hay nada que limpiar.
    }
    fetch("/bff/logout", { method: "POST", cache: "no-store" })
      .then((res) => res.json())
      .then((body: { url?: string }) => window.location.replace(body.url ?? "/login")) // cierra también en Keycloak
      .catch(() => window.location.replace("/login"));
  }, []);

  // TODO(PV-21): cuando exista user_warehouses, filtrar alertas, incidencias, anulaciones y bandejas por las bodegas del usuario (Admin sin restricción; Cliente por empresa)
  const can = useCallback(
    ({ roles, permission, module, action }: { roles?: Role[]; permission?: string; module?: string; action?: string }) => {
      if (!session) return false;
      const u = session.user;
      const okRole = !roles || roles.includes(u.role);
      const okPerm = !permission || u.permissions.includes(permission);
      const mod = module ? u.modules.find((m) => m.module === module) : undefined;
      const okModule = !module || (!!mod && (!action || mod.actions.includes(action)));
      return okRole && okPerm && okModule;
    },
    [session],
  );

  const value = useMemo(
    () => ({ session, ready, profileError, companyId, setCompanyId, login, logout, can }),
    [session, ready, profileError, companyId, login, logout, can],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth debe usarse dentro de <AuthProvider>");
  return ctx;
}

export const ROLE_LABELS: Record<Role, string> = {
  admin: "Administrador",
  jefe_operaciones: "Jefe de Operaciones",
  analista: "Analista",
  empaque: "Empaque",
  cliente: "Cliente",
};
