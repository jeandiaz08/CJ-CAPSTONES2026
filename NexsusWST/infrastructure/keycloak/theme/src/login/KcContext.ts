import type { ExtendKcContext, PasswordPolicies } from "keycloakify/login/KcContext";

// Keycloak 26.8.1 supplies PasswordPoliciesBean to every login page.
// Keycloakify 11.16.4 does not yet declare it for UPDATE_PASSWORD.
type KcContextExtensionPerPage = {
  "login-update-password.ftl": {
    passwordPolicies?: PasswordPolicies;
    username?: string;
    login?: { username?: string };
    user?: { username?: string; email?: string };
  };
};

export const kcContextExtensionPerPage: KcContextExtensionPerPage = {
  "login-update-password.ftl": {},
};

type KcContextExtension = {
  properties?: Record<string, string | undefined>;
  "x-nexus-preview"?: { resendReady?: boolean };
};
export const kcContextExtension: KcContextExtension = {};
export type KcContext = ExtendKcContext<KcContextExtension, KcContextExtensionPerPage>;
