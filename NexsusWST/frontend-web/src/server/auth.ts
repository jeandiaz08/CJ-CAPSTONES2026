// Auth.js con Keycloak: código + PKCE, cliente público nexus-web. Los tokens quedan solo en
// la cookie cifrada; la respuesta de /api/auth/session no los incluye.
import NextAuth from "next-auth";
import Keycloak from "next-auth/providers/keycloak";
import { keycloakClientId, keycloakInternalEndpoint, keycloakIssuer, SESSION_MAX_AGE } from "./session";

const DEFAULT_REFRESH_LIFETIME_S = 2 * 60 * 60; // inactividad de Keycloak

export const { handlers } = NextAuth(() => ({
  trustHost: true,
  session: { strategy: "jwt", maxAge: SESSION_MAX_AGE },
  pages: { signIn: "/login", error: "/login" },
  providers: [
    Keycloak({
      clientId: keycloakClientId(),
      issuer: keycloakIssuer(),
      authorization: { url: `${keycloakIssuer()}/protocol/openid-connect/auth`, params: { scope: "openid profile email", ui_locales: "es" } },
      token: keycloakInternalEndpoint("token"),
      userinfo: keycloakInternalEndpoint("userinfo"),
      client: { token_endpoint_auth_method: "none" },
      checks: ["pkce", "state"],
    }),
  ],
  callbacks: {
    jwt({ token, account }) {
      if (!account) return token;
      const now = Date.now();
      const refreshLifetime = Number((account as { refresh_expires_in?: number }).refresh_expires_in ?? DEFAULT_REFRESH_LIFETIME_S);
      return {
        ...token,
        accessToken: account.access_token,
        refreshToken: account.refresh_token,
        idToken: account.id_token,
        expiresAt: account.expires_at ? account.expires_at * 1000 : now,
        refreshExpiresAt: now + refreshLifetime * 1000,
      };
    },
  },
}));
