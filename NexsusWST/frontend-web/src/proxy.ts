// Protege las rutas de la app: sin sesión, al login. El detalle por perfil lo resuelve
// (app)/layout.tsx y la regla definitiva, el servidor de cada API.
import { NextResponse, type NextRequest } from "next/server";
import { readSession } from "@/server/session";

export async function proxy(req: NextRequest) {
  const session = await readSession(req).catch(() => null);
  if (!session) {
    const { pathname, search } = req.nextUrl;
    // Tras Traefik el host de la petición es el del contenedor: se usa la URL pública.
    const login = new URL("/login", process.env.APP_PUBLIC_URL ?? req.nextUrl.origin);
    if (pathname !== "/") login.searchParams.set("next", pathname + search);
    return NextResponse.redirect(login);
  }
  const res = NextResponse.next();
  res.headers.set("Cache-Control", "no-store, private"); // «atrás» no muestra datos tras cerrar sesión
  return res;
}

export const config = {
  matcher: ["/((?!api/auth|bff|login|_next/static|_next/image|brand|couriers|integrations/|fonts|favicon.ico).*)"],
};
