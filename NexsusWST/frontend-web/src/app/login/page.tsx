"use client";

// La pantalla de usuario y contraseña es la de Keycloak (con la marca
// WeStorage, H-01 T07). Aquí solo se redirige; "¿Olvidaste tu contraseña?"
// también está en esa pantalla y envía el correo desde westorage.cl.
import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/state/auth";

const DEMO = [
  ["admin@nexus.local", "Administrador"],
  ["jefe@nexus.local", "Jefe de Operaciones"],
  ["empaque@nexus.local", "Empaque (permiso Despacho)"],
  ["analista@nexus.local", "Analista"],
  ["cliente1@nexus.local", "Cliente (Empresa Demo Uno)"],
  ["cliente2@nexus.local", "Cliente (Empresa Demo Dos)"],
];

function LoginInner() {
  const { login, session, ready } = useAuth();
  const router = useRouter();
  const params = useSearchParams();
  const [busy, setBusy] = useState(false);
  const message = params.get("error");

  useEffect(() => {
    if (ready && session) router.replace("/");
  }, [ready, session, router]);

  const go = async () => {
    setBusy(true);
    await login(params.get("next") ?? "/");
  };

  return (
    <main className="login">
      <div className="login-card">
        <div className="sidebar-brand dark">
          <span className="brand-mark">N</span>
          <span>NexusWST</span>
        </div>
        <h1>Iniciar sesión</h1>
        <p className="muted small">Ingresarás con tu cuenta de WeStorage. Si olvidaste tu contraseña, puedes recuperarla en la siguiente pantalla.</p>
        {message && <p className="error">{message}</p>}
        <button type="button" className="btn primary block" disabled={busy} onClick={go}>
          {busy ? "Redirigiendo…" : "Ingresar"}
        </button>
        {process.env.NODE_ENV === "development" && (
          <div className="demo-users">
            <p className="muted small">
              Usuarios de prueba en Keycloak (contraseña <code>Nexus2026!</code>):
            </p>
            <ul className="muted small">
              {DEMO.map(([mail, label]) => (
                <li key={mail}>
                  <code>{mail}</code> · {label}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </main>
  );
}

export default function LoginPage() {
  return (
    <Suspense>
      <LoginInner />
    </Suspense>
  );
}
