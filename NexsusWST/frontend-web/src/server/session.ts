// Sesión en el servidor: los tokens de Keycloak viven en una cookie httpOnly cifrada por
// Auth.js y nunca llegan al navegador. Este módulo la lee y la renueva.
import { encode, getToken } from "next-auth/jwt";

const SESSION_MAX_AGE_S = 12 * 60 * 60; // igual que la sesión máxima de Keycloak
const REFRESH_MARGIN_MS = 60_000;
const REFRESH_DEDUPE_MS = 5_000;
const COOKIE_CHUNK_SIZE = 3936; // el mismo tamaño con que Auth.js divide la cookie

export type SessionTokens = {
  accessToken: string;
  refreshToken: string;
  idToken: string;
  expiresAt: number; // ms
  refreshExpiresAt: number; // ms
};

// Keycloak rechazó la renovación: la sesión terminó (cierre, inactividad o usuario deshabilitado).
export class SessionEndedError extends Error {}

const secureCookies = () => (process.env.AUTH_URL ?? "").startsWith("https://");
export const sessionCookieName = () => (secureCookies() ? "__Secure-authjs.session-token" : "authjs.session-token");
export const SESSION_MAX_AGE = SESSION_MAX_AGE_S;

export const keycloakClientId = () => process.env.AUTH_KEYCLOAK_ID ?? "nexus-web";
export const keycloakIssuer = () => process.env.AUTH_KEYCLOAK_ISSUER ?? "";

// El servidor habla con Keycloak por la red interna; el navegador, por la URL pública.
export function keycloakInternalEndpoint(name: "token" | "userinfo") {
  const realm = keycloakIssuer().split("/realms/")[1] ?? "nexuswst";
  return `${process.env.KEYCLOAK_INTERNAL_URL}/realms/${realm}/protocol/openid-connect/${name}`;
}

export async function readSession(req: { headers: Headers }): Promise<SessionTokens | null> {
  const name = sessionCookieName();
  const token = await getToken({ req, secret: process.env.AUTH_SECRET!, secureCookie: secureCookies(), cookieName: name, salt: name });
  if (!token?.accessToken || !token.refreshToken) return null;
  const session = token as unknown as SessionTokens;
  return session.refreshExpiresAt > Date.now() ? session : null;
}

const inFlight = new Map<string, Promise<SessionTokens>>();

async function requestRefresh(tokens: SessionTokens): Promise<SessionTokens> {
  const res = await fetch(keycloakInternalEndpoint("token"), {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({ grant_type: "refresh_token", client_id: keycloakClientId(), refresh_token: tokens.refreshToken }),
    cache: "no-store",
  });
  const body = await res.json().catch(() => ({}));
  if (res.status === 400 || res.status === 401) throw new SessionEndedError(body.error ?? "invalid_grant");
  if (!res.ok) throw new Error(`Keycloak respondió ${res.status}`);
  const now = Date.now();
  return {
    accessToken: body.access_token,
    refreshToken: body.refresh_token ?? tokens.refreshToken,
    idToken: body.id_token ?? tokens.idToken,
    expiresAt: now + Number(body.expires_in ?? 0) * 1000,
    refreshExpiresAt: now + Number(body.refresh_expires_in ?? 0) * 1000,
  };
}

// Varias peticiones simultáneas comparten una sola renovación.
export function refreshTokens(tokens: SessionTokens): Promise<SessionTokens> {
  const running = inFlight.get(tokens.refreshToken);
  if (running) return running;
  const request = requestRefresh(tokens).finally(() => setTimeout(() => inFlight.delete(tokens.refreshToken), REFRESH_DEDUPE_MS));
  inFlight.set(tokens.refreshToken, request);
  return request;
}

// Renueva si el token está por vencer (o si force). Con Keycloak caído, el token vigente sirve
// hasta vencer; una sesión terminada lanza SessionEndedError.
export async function freshTokens(tokens: SessionTokens, force = false): Promise<{ tokens: SessionTokens; renewed: boolean }> {
  if (!force && tokens.expiresAt - Date.now() > REFRESH_MARGIN_MS) return { tokens, renewed: false };
  try {
    return { tokens: await refreshTokens(tokens), renewed: true };
  } catch (e) {
    if (e instanceof SessionEndedError || tokens.expiresAt <= Date.now()) throw e;
    return { tokens, renewed: false };
  }
}

const cookieAttributes = (maxAge: number) => `Path=/; HttpOnly; SameSite=Lax; Max-Age=${maxAge}${secureCookies() ? "; Secure" : ""}`;

function existingSessionCookies(cookieHeader: string | null) {
  const name = sessionCookieName();
  return (cookieHeader ?? "").split(";").map((c) => c.split("=")[0].trim()).filter((c) => c === name || c.startsWith(`${name}.`));
}

// Set-Cookie para guardar los tokens renovados, dividida igual que Auth.js y sin dejar trozos viejos.
export async function sessionCookies(req: { headers: Headers }, tokens: SessionTokens): Promise<string[]> {
  const name = sessionCookieName();
  const value = await encode({ token: tokens, secret: process.env.AUTH_SECRET!, salt: name, maxAge: SESSION_MAX_AGE_S });
  const chunks = value.match(new RegExp(`.{1,${COOKIE_CHUNK_SIZE}}`, "g")) ?? [];
  const pairs = chunks.length === 1 ? [[name, chunks[0]]] : chunks.map((chunk, i) => [`${name}.${i}`, chunk]);
  const written = new Set(pairs.map(([n]) => n));
  return [
    ...pairs.map(([n, v]) => `${n}=${v}; ${cookieAttributes(SESSION_MAX_AGE_S)}`),
    ...existingSessionCookies(req.headers.get("cookie")).filter((n) => !written.has(n)).map((n) => `${n}=; ${cookieAttributes(0)}`),
  ];
}

export function clearSessionCookies(req: { headers: Headers }): string[] {
  return existingSessionCookies(req.headers.get("cookie")).map((n) => `${n}=; ${cookieAttributes(0)}`);
}

// Defensa contra otros sitios, además de SameSite: las peticiones que cambian algo llegan desde la app.
export function sameOrigin(req: Request) {
  const origin = req.headers.get("origin");
  if (!origin) return true;
  const app = process.env.APP_PUBLIC_URL;
  return !app || origin === new URL(app).origin;
}
