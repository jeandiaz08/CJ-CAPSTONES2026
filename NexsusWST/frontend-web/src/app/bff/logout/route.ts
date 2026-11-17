// Cierra la sesión de Nexus y entrega la URL para cerrarla también en Keycloak.
import { clearSessionCookies, keycloakClientId, keycloakIssuer, readSession, sameOrigin } from "@/server/session";

export const dynamic = "force-dynamic";

export async function POST(req: Request) {
  if (!sameOrigin(req)) return Response.json({ error: "forbidden" }, { status: 403 });
  const session = await readSession(req).catch(() => null);
  const app = process.env.APP_PUBLIC_URL ?? new URL(req.url).origin;
  let url = "/login";
  if (session?.idToken) {
    const q = new URLSearchParams({ client_id: keycloakClientId(), post_logout_redirect_uri: `${app}/login`, id_token_hint: session.idToken });
    url = `${keycloakIssuer()}/protocol/openid-connect/logout?${q}`;
  }
  const headers = new Headers({ "Content-Type": "application/json", "Cache-Control": "no-store" });
  clearSessionCookies(req).forEach((c) => headers.append("Set-Cookie", c));
  return new Response(JSON.stringify({ url }), { status: 200, headers });
}
