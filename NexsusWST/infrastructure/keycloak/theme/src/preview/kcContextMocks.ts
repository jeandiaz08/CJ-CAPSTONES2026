import { createGetKcContextMock } from "keycloakify/login/KcContext";
import type { KcContext } from "../login/KcContext";
import { kcContextExtension, kcContextExtensionPerPage } from "../login/KcContext";

export const scenarios = {
  login: "Login normal · Microsoft no configurado",
  "login-microsoft": "Login normal · Microsoft configurado",
  "login-error": "Error de credenciales · Microsoft no configurado",
  "login-microsoft-error": "Error de credenciales · Microsoft configurado",
  reset: "Recuperar contraseña",
  "reset-error": "Recuperación · error de validación",
  "check-email": "Revisa tu correo · confirmación genérica",
  "check-email-ready": "Revisa tu correo · reenviar habilitado",
  "check-email-no-user": "Revisa tu correo · sin usuario ingresado",
  "new-password": "Crear contraseña",
  "new-password-error": "Crear contraseña · error del servidor",
  error: "Error general · enlace vencido",
  info: "Mensaje informativo de Keycloak",
} as const;
export type Scenario = keyof typeof scenarios;

const { getKcContextMock } = createGetKcContextMock({
  kcContextExtension,
  kcContextExtensionPerPage,
  overrides: {
    themeName: "nexuswst",
    realm: {
      name: "nexuswst",
      displayName: "NexusWST · WeStorage",
      displayNameHtml: "NexusWST · WeStorage",
      internationalizationEnabled: false,
      registrationEmailAsUsername: false,
    },
    locale: { currentLanguageTag: "es" },
    url: {
      loginUrl: "?screen=login",
      loginAction: "#preview-submit",
      loginRestartFlowUrl: "?screen=login",
    },
  },
  overridesPerPage: {
    "login.ftl": {
      realm: { rememberMe: true, registrationAllowed: false, resetPasswordAllowed: true },
      login: { username: "" },
      social: { displayInfo: false, providers: [] },
      url: { loginResetCredentialsUrl: "?screen=reset" },
    },
    "login-reset-password.ftl": {
      auth: { attemptedUsername: "" },
      url: { loginResetCredentialsUrl: "?screen=reset" },
    },
    "error.ftl": { client: { baseUrl: "?screen=login" } },
    "info.ftl": { client: { baseUrl: "?screen=login" } },
    "login-update-password.ftl": {
      auth: { attemptedUsername: "ana.demo@westorage.cl" },
      passwordPolicies: { length: 8, upperCase: 1, digits: 1, notUsername: true },
    },
  },
});

function fieldErrors(errors: Record<string, string>): KcContext["messagesPerField"] {
  return {
    get: (field) => errors[field] ?? "",
    exists: (field) => Boolean(errors[field]),
    existsError: (...fields) => fields.some((field) => Boolean(errors[field])),
    getFirstError: (...fields) => fields.map((field) => errors[field]).find(Boolean) ?? "",
    printIfExists: (field, text) => errors[field] ? text : undefined,
  };
}

export function getPreviewContext(scenario: Scenario): KcContext {
  switch (scenario) {
    case "login-microsoft":
      return getKcContextMock({ pageId: "login.ftl", overrides: {
        social: { displayInfo: true, providers: [{ alias: "microsoft", providerId: "oidc", displayName: "Microsoft", loginUrl: "?screen=login" }] },
      } });
    case "login-error":
      return getKcContextMock({ pageId: "login.ftl", overrides: {
        login: { username: "operador.demo" },
        messagesPerField: fieldErrors({ username: "Usuario o contraseña incorrectos.", password: "Usuario o contraseña incorrectos." }),
      } });
    case "login-microsoft-error":
      return getKcContextMock({ pageId: "login.ftl", overrides: {
        login: { username: "operador.demo" },
        messagesPerField: fieldErrors({ username: "Usuario o contraseña incorrectos.", password: "Usuario o contraseña incorrectos." }),
        social: { displayInfo: true, providers: [{ alias: "microsoft", providerId: "oidc", displayName: "Microsoft", loginUrl: "?screen=login" }] },
      } });
    case "reset":
      return getKcContextMock({ pageId: "login-reset-password.ftl" });
    case "reset-error":
      return getKcContextMock({ pageId: "login-reset-password.ftl", overrides: {
        messagesPerField: fieldErrors({ username: "Ingresa tu correo o nombre de usuario." }),
      } });
    case "check-email":
    case "check-email-ready":
    case "check-email-no-user":
      return getKcContextMock({ pageId: "login.ftl", overrides: { message: {
        type: "success",
        summary: "Si la cuenta existe y tiene un correo asociado, recibirás instrucciones para recuperar tu contraseña.",
      }, auth: { attemptedUsername: scenario === "check-email-no-user" ? undefined : "ana.demo@westorage.cl" },
      "x-nexus-preview": { resendReady: scenario === "check-email-ready" } } });
    case "new-password":
      return getKcContextMock({ pageId: "login-update-password.ftl" });
    case "new-password-error":
      return getKcContextMock({ pageId: "login-update-password.ftl", overrides: {
        messagesPerField: fieldErrors({ password: "La contraseña no cumple la política configurada.", "password-confirm": "Las contraseñas no coinciden." }),
      } });
    case "error":
      return getKcContextMock({ pageId: "error.ftl", overrides: {
        message: { type: "error", summary: "La solicitud ha expirado. Vuelve a iniciar el proceso." },
      } });
    case "info":
      return getKcContextMock({ pageId: "info.ftl", overrides: {
        message: { type: "success", summary: "Tu contraseña se ha actualizado." },
        messageHeader: "Contraseña actualizada",
      } });
    default:
      return getKcContextMock({ pageId: "login.ftl" });
  }
}
