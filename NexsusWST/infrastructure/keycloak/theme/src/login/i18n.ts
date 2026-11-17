import { i18nBuilder } from "keycloakify/login";

const { useI18n } = i18nBuilder
  .withCustomTranslations({
    es: {
      loginAccountTitle: "Inicia sesión",
      usernameOrEmail: "Correo o usuario",
      password: "Contraseña",
      doLogIn: "Ingresar",
      doForgotPassword: "¿Olvidaste tu contraseña?",
      rememberMe: "Mantener sesión iniciada",
      backToLogin: "Volver al inicio de sesión",
      emailForgotTitle: "¿Olvidaste tu contraseña?",
      updatePasswordTitle: "Crea tu contraseña",
      passwordNew: "Nueva contraseña",
      passwordConfirm: "Repite la contraseña",
      showPassword: "Mostrar contraseña",
      hidePassword: "Ocultar contraseña",
      logoutOtherSessions: "Cerrar sesión en otros dispositivos",
      backToApplication: "Volver a la aplicación",
      proceedWithAction: "Continuar",
      invalidUserMessage: "El correo, usuario o contraseña son incorrectos.",
      invalidPasswordMessage: "El correo, usuario o contraseña son incorrectos.",
      emailSentMessage:
        "Si la cuenta existe y tiene un correo asociado, recibirás instrucciones para recuperar tu contraseña.",
    },
  })
  .build();

export type I18n = ReturnType<typeof useI18n>["i18n"];
export { useI18n };
