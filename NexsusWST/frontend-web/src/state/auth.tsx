"use client";

// Sesión del usuario (RF-01, DAS v1.1 §4.5).
//  - Keycloak inicia la sesión y emite el token (10 min); aquí se renueva en
//    silencio 1 minuto antes de vencer. La sesión dura hasta 12 h y se cierra
//    tras 2 h sin uso (lo decide Keycloak).
//  - El PERFIL (rol, permisos, empresas, módulos) sale del Core: /v1/auth/me.
//  - Se guarda en sessionStorage (se borra al cerrar la pestaña).
// Puente mínimo: Jean lo reemplaza por Auth.js manteniendo esta interfaz.
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { logoutUrl, refreshTokens, startLogin, type TokenSet } from "@/services/oidc";

export type Role = "admin" | "jefe_operaciones" | "analista" | "empaque" | "cliente";

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

export type Session = TokenSet & { user: SessionUser };

type AuthState = {
  session: Session | null;
  ready: boolean;
  /** Empresa elegida en la barra superior (null = todas, solo personal interno) */
  companyId: number | null;
  setCompanyId: (id: number | null) => void;
  /** Redirige al inicio de sesión de Keycloak. */
  login: (returnTo?: string) => Promise<void>;
  /** Guarda la sesión obtenida en /login/callback. */
  establish: (tokens: TokenSet) => Promise<SessionUser>;
  logout: () => void;
  can: (opts: { roles?: Role[]; permission?: string; module?: string; action?: string }) => boolean;
};

const KEY = "nexus.session";
const AuthContext = createContext<AuthState | null>(null);

function readStored(): Session | null {
  try {
    const raw = sessionStorage.getItem(KEY);
    if (!raw) return null;
    const s = JSON.parse(raw) as Session;
    // Sirve mientras el token o la sesión de Keycloak sigan vigentes.
    return s.expiresAt > Date.now() || s.refreshExpiresAt > Date.now() ? s : null;
  } catch {
    return null;
  }
}

function store(s: Session | null) {
  try {
    if (s) sessionStorage.setItem(KEY, JSON.stringify(s));
    else sessionStorage.removeItem(KEY);
  } catch {
    /* almacenamiento no disponible: la sesión vive solo en memoria */
  }
}

// Token accesible para services/api.ts (fuera de React)
let currentToken: string | null = null;
export const getToken = () => currentToken;

async function fetchProfile(accessToken: string): Promise<SessionUser> {
  const res = await fetch("/api/core/v1/auth/me", { headers: { Authorization: `Bearer ${accessToken}` }, cache: "no-store" });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    const err = new Error(body.message ?? `No se pudo leer tu perfil (HTTP ${res.status})`) as Error & { code?: string; status?: number };
    err.code = body.error;
    err.status = res.status;
    throw err;
  }
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

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<Session | null>(null);
  const [ready, setReady] = useState(false);
  const [companyId, setCompanyId] = useState<number | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const apply = useCallback((s: Session | null) => {
    currentToken = s && s.expiresAt > Date.now() ? s.accessToken : null;
    setSession(s);
    store(s);
    if (s && !s.user.allCompanies && s.user.companies.length > 0) {
      setCompanyId((prev) => (prev != null && s.user.companies.includes(prev) ? prev : s.user.companies[0]));
    }
  }, []);

  const clear = useCallback(() => {
    apply(null);
    setCompanyId(null);
  }, [apply]);

  const doRefresh = useCallback(
    async (s: Session) => {
      try {
        const tokens = await refreshTokens(s.refreshToken);
        let user = s.user;
        try {
          user = await fetchProfile(tokens.accessToken); // recoge cambios de rol/empresas
        } catch (e) {
          const code = (e as { code?: string }).code;
          if (code === "user_disabled" || code === "company_disabled") {
            clear();
            return;
          }
          /* Core caído: se mantiene el perfil anterior (continuidad de Órdenes) */
        }
        apply({ ...tokens, user });
      } catch (e) {
        // 400 = sesión terminada en Keycloak (cierre, 2 h sin uso, 12 h, usuario deshabilitado)
        if ((e as { status?: number }).status === 400) clear();
        /* Keycloak caído: el token vigente sirve hasta vencer (DAS §4.5) */
      }
    },
    [apply, clear],
  );

  useEffect(() => {
    const s = readStored();
    if (s && s.expiresAt <= Date.now() + 5_000) {
      void doRefresh(s).finally(() => setReady(true));
    } else {
      apply(s);
      setReady(true);
    }
  }, [apply, doRefresh]);

  // Renovación silenciosa: 1 minuto antes de que venza el token (10 min).
  useEffect(() => {
    if (timer.current) clearTimeout(timer.current);
    if (!session) return;
    const ms = session.expiresAt - Date.now() - 60_000;
    timer.current = setTimeout(() => void doRefresh(session), Math.max(ms, 5_000));
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
  }, [session, doRefresh]);

  const login = useCallback((returnTo?: string) => startLogin(returnTo), []);

  const establish = useCallback(
    async (tokens: TokenSet) => {
      const user = await fetchProfile(tokens.accessToken);
      apply({ ...tokens, user });
      return user;
    },
    [apply],
  );

  const logout = useCallback(() => {
    const idToken = session?.idToken ?? "";
    clear();
    window.location.assign(logoutUrl(idToken)); // cierra la sesión también en Keycloak
  }, [session, clear]);

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
    () => ({ session, ready, companyId, setCompanyId, login, establish, logout, can }),
    [session, ready, companyId, login, establish, logout, can],
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
