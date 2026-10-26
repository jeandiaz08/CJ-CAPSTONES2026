"use client";

// Vuelta desde Keycloak: cambia el código por los tokens (PKCE) y lee el
// perfil de Nexus en el Core (que lo crea en el primer ingreso).
import { Suspense, useEffect, useRef, useState } from "react";
import { useSearchParams } from "next/navigation";
import { completeLogin, logoutUrl } from "@/services/oidc";
import { useAuth } from "@/state/auth";

function CallbackInner() {
  const params = useSearchParams();
  const { establish } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const [idToken, setIdToken] = useState("");
  const done = useRef(false);

  useEffect(() => {
    if (done.current) return; // React (modo estricto) monta dos veces en desarrollo
    done.current = true;
    (async () => {
      try {
        const { tokens, returnTo } = await completeLogin(new URLSearchParams(params.toString()));
        setIdToken(tokens.idToken);
        await establish(tokens);
        // replace: el código de un solo uso no queda en el historial del navegador
        window.location.replace(returnTo.startsWith("/") && !returnTo.startsWith("//") ? returnTo : "/");
      } catch (e) {
        setError((e as Error).message);
      }
    })();
  }, [params, establish]);

  return (
    <main className="login">
      <div className="login-card">
        <h1>{error ? "No se pudo iniciar sesión" : "Ingresando…"}</h1>
        {error && (
          <>
            <p className="error">{error}</p>
            <button
              type="button"
              className="btn primary block"
              onClick={() => window.location.assign(idToken ? logoutUrl(idToken) : "/login")}
            >
              Volver al inicio de sesión
            </button>
          </>
        )}
      </div>
    </main>
  );
}

export default function CallbackPage() {
  return (
    <Suspense>
      <CallbackInner />
    </Suspense>
  );
}
