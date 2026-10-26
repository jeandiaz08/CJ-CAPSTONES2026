// Inicio de sesión con Keycloak: OpenID Connect, código de autorización + PKCE
// (DAS v1.1 §4.5). Puente mínimo sin dependencias: Jean lo reemplaza por
// Auth.js (H-01 T05/T06) manteniendo la misma interfaz de state/auth.tsx.
//
// Keycloak se publica por Traefik en el MISMO origen (/auth), así que no hay
// CORS y el navegador nunca habla con nombres internos de Docker.

const REALM = process.env.NEXT_PUBLIC_KEYCLOAK_REALM ?? "nexuswst";
const CLIENT_ID = process.env.NEXT_PUBLIC_KEYCLOAK_CLIENT_ID ?? "nexus-web";

function base(): string {
  const configured = process.env.NEXT_PUBLIC_KEYCLOAK_URL; // p. ej. https://auth.westorage.cl/auth
  return `${configured ?? `${window.location.origin}/auth`}/realms/${REALM}/protocol/openid-connect`;
}

const redirectUri = () => `${window.location.origin}/login/callback`;

export type TokenSet = {
  accessToken: string;
  refreshToken: string;
  idToken: string;
  expiresAt: number; // ms epoch
  refreshExpiresAt: number; // ms epoch (sesión: 2 h sin uso / 12 h máx.)
};

const PKCE_KEY = "nexus.pkce";

function randomString(bytes = 32): string {
  const a = new Uint8Array(bytes);
  crypto.getRandomValues(a);
  return b64url(a);
}

function b64url(a: Uint8Array): string {
  let s = "";
  a.forEach((b) => (s += String.fromCharCode(b)));
  return btoa(s).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

async function challenge(verifier: string): Promise<string> {
  const hash = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(verifier));
  return b64url(new Uint8Array(hash));
}

/** Redirige a la pantalla de inicio de sesión de Keycloak. */
export async function startLogin(returnTo = "/"): Promise<void> {
  const verifier = randomString(48);
  const state = randomString(16);
  sessionStorage.setItem(PKCE_KEY, JSON.stringify({ verifier, state, returnTo }));
  const q = new URLSearchParams({
    client_id: CLIENT_ID,
    redirect_uri: redirectUri(),
    response_type: "code",
    scope: "openid profile email",
    state,
    code_challenge: await challenge(verifier),
    code_challenge_method: "S256",
    ui_locales: "es",
  });
  window.location.assign(`${base()}/auth?${q}`);
}

/** Recuperar contraseña: Keycloak muestra el formulario y envía el correo. */
export async function startForgotPassword(): Promise<void> {
  await startLogin("/");
}

function toTokenSet(b: Record<string, unknown>): TokenSet {
  const now = Date.now();
  return {
    accessToken: String(b.access_token),
    refreshToken: String(b.refresh_token ?? ""),
    idToken: String(b.id_token ?? ""),
    expiresAt: now + Number(b.expires_in ?? 0) * 1000,
    refreshExpiresAt: now + Number(b.refresh_expires_in ?? 0) * 1000,
  };
}

async function tokenRequest(form: Record<string, string>): Promise<TokenSet> {
  const res = await fetch(`${base()}/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({ client_id: CLIENT_ID, ...form }),
  });
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    const err = new Error(body.error_description ?? body.error ?? `HTTP ${res.status}`) as Error & { status?: number };
    err.status = res.status;
    throw err;
  }
  return toTokenSet(body);
}

/** Paso de vuelta: cambia el código por los tokens. */
export async function completeLogin(params: URLSearchParams): Promise<{ tokens: TokenSet; returnTo: string }> {
  const raw = sessionStorage.getItem(PKCE_KEY);
  sessionStorage.removeItem(PKCE_KEY);
  if (params.get("error")) throw new Error(params.get("error_description") ?? params.get("error") ?? "Inicio de sesión cancelado");
  if (!raw) throw new Error("La solicitud de inicio de sesión expiró. Vuelve a intentarlo.");
  const { verifier, state, returnTo } = JSON.parse(raw) as { verifier: string; state: string; returnTo: string };
  if (params.get("state") !== state) throw new Error("Respuesta de inicio de sesión no válida (state).");
  const tokens = await tokenRequest({
    grant_type: "authorization_code",
    code: params.get("code") ?? "",
    redirect_uri: redirectUri(),
    code_verifier: verifier,
  });
  return { tokens, returnTo: returnTo || "/" };
}

/** Renovación silenciosa. Lanza error con status 400 si la sesión terminó. */
export function refreshTokens(refreshToken: string): Promise<TokenSet> {
  return tokenRequest({ grant_type: "refresh_token", refresh_token: refreshToken });
}

/** Cierra la sesión en Keycloak y vuelve a /login. */
export function logoutUrl(idToken: string): string {
  const q = new URLSearchParams({ client_id: CLIENT_ID, post_logout_redirect_uri: `${window.location.origin}/login` });
  if (idToken) q.set("id_token_hint", idToken);
  return `${base()}/logout?${q}`;
}
