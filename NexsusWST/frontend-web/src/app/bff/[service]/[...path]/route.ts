// BFF: el navegador llama aquí con su cookie y el servidor reenvía con el Bearer por la red
// interna. Los cuerpos y las respuestas pasan en streaming (importar, exportar, documentos).
import { clearSessionCookies, freshTokens, readSession, sameOrigin, SessionEndedError, sessionCookies, type SessionTokens } from "@/server/session";

export const dynamic = "force-dynamic";

const UPSTREAMS: Record<string, () => string | undefined> = {
  core: () => process.env.CORE_INTERNAL_URL,
  orders: () => process.env.ORDERS_INTERNAL_URL,
};

const REQUEST_HEADERS = ["accept", "accept-language", "content-type", "content-length", "if-none-match", "x-request-id", "x-forwarded-for"];
const RESPONSE_HEADERS_DROPPED = ["connection", "keep-alive", "transfer-encoding", "content-encoding", "set-cookie"];
const BODYLESS = ["GET", "HEAD"];

type Context = { params: Promise<{ service: string; path: string[] }> };

function jsonError(status: number, error: string, message: string, cookies: string[] = []) {
  const headers = new Headers({ "Content-Type": "application/json", "Cache-Control": "no-store" });
  cookies.forEach((c) => headers.append("Set-Cookie", c));
  return new Response(JSON.stringify({ error, message }), { status, headers });
}

async function forward(req: Request, target: string, accessToken: string, body: BodyInit | null | undefined) {
  const headers = new Headers({ Authorization: `Bearer ${accessToken}`, "Accept-Encoding": "identity" });
  for (const name of REQUEST_HEADERS) {
    const value = req.headers.get(name);
    if (value) headers.set(name, value);
  }
  return fetch(target, { method: req.method, headers, body, redirect: "manual", cache: "no-store", signal: req.signal, duplex: "half" } as RequestInit);
}

async function handle(req: Request, { params }: Context) {
  const { service, path } = await params;
  const base = Object.hasOwn(UPSTREAMS, service) ? UPSTREAMS[service]() : undefined;
  // Solo la API pública: no alcanza /ws, /health ni rutas fuera de /v1.
  if (!base || path[0] !== "v1" || path.some((s) => s === "." || s === ".." || s.includes("/") || s.includes("\\"))) {
    return jsonError(404, "not_found", "Ruta no encontrada.");
  }
  if (!BODYLESS.includes(req.method) && !sameOrigin(req)) return jsonError(403, "forbidden", "Origen no permitido.");

  const session = await readSession(req);
  if (!session) return jsonError(401, "session_expired", "Tu sesión terminó. Vuelve a iniciar sesión.", clearSessionCookies(req));

  let tokens: SessionTokens;
  let renewed: boolean;
  try {
    ({ tokens, renewed } = await freshTokens(session));
  } catch (e) {
    if (e instanceof SessionEndedError) return jsonError(401, "session_expired", "Tu sesión terminó. Vuelve a iniciar sesión.", clearSessionCookies(req));
    return jsonError(503, "unavailable", "No se pudo renovar tu sesión. Intenta nuevamente.");
  }

  // Un cuerpo JSON se puede reenviar tras renovar. Un archivo va en streaming y no se puede
  // repetir: para esos casos cuenta la renovación anticipada de arriba.
  const search = new URL(req.url).search;
  const target = `${base}/${path.map(encodeURIComponent).join("/")}${search}`;
  const replayable = BODYLESS.includes(req.method) || !(req.headers.get("content-type") ?? "").startsWith("multipart/");
  const body = BODYLESS.includes(req.method) ? undefined : replayable ? await req.text() : req.body;

  let upstream: Response;
  try {
    upstream = await forward(req, target, tokens.accessToken, body);
    if (upstream.status === 401 && replayable) {
      try {
        const retried = await freshTokens(tokens, true);
        tokens = retried.tokens;
        renewed = renewed || retried.renewed;
      } catch (e) {
        if (e instanceof SessionEndedError) return jsonError(401, "session_expired", "Tu sesión terminó. Vuelve a iniciar sesión.", clearSessionCookies(req));
        throw e;
      }
      await upstream.body?.cancel();
      upstream = await forward(req, target, tokens.accessToken, body);
    }
  } catch {
    if (req.signal.aborted) return new Response(null, { status: 499 });
    return jsonError(502, "upstream_unavailable", "El servicio no está disponible. Intenta nuevamente.");
  }

  const headers = new Headers(upstream.headers);
  RESPONSE_HEADERS_DROPPED.forEach((name) => headers.delete(name));
  if (!headers.has("Cache-Control")) headers.set("Cache-Control", "no-store");
  if (renewed) (await sessionCookies(req, tokens)).forEach((c) => headers.append("Set-Cookie", c));
  return new Response(upstream.body, { status: upstream.status, statusText: upstream.statusText, headers });
}

export { handle as GET, handle as HEAD, handle as POST, handle as PUT, handle as PATCH, handle as DELETE };
