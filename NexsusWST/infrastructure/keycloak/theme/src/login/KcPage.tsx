import { lazy, Suspense } from "react";
import Template from "keycloakify/login/Template";
import type { KcContext } from "./KcContext";
import { useI18n } from "./i18n";
import Login from "./pages/Login";
import ResetPassword from "./pages/ResetPassword";
import UpdatePassword from "./pages/UpdatePassword";
import Info from "./pages/Info";
import Error from "./pages/Error";

const DefaultPage = lazy(() => import("keycloakify/login/DefaultPage"));
const UserProfileFormFields = lazy(() => import("keycloakify/login/UserProfileFormFields"));

export default function KcPage({ kcContext }: { kcContext: KcContext }) {
  const { i18n } = useI18n({ kcContext });
  // Avoid rendering fallback-language messages while Spanish loads.
  if (i18n.isFetchingTranslations) return null;

  const defaultPage = (
    <DefaultPage
      kcContext={kcContext}
      i18n={i18n}
      Template={Template}
      doUseDefaultCss
      UserProfileFormFields={UserProfileFormFields}
      doMakeUserConfirmPassword
    />
  );

  return (
    <Suspense fallback={null}>
      {(() => {
        switch (kcContext.pageId) {
          case "login.ftl":
            // Keep optional authentication flows functional until they have a defined design.
            return kcContext.enableWebAuthnConditionalUI || kcContext.auth.showTryAnotherWayLink
              ? defaultPage
              : <Login kcContext={kcContext} i18n={i18n} />;
          case "login-reset-password.ftl":
            return <ResetPassword kcContext={kcContext} />;
          case "login-update-password.ftl":
            return <UpdatePassword kcContext={kcContext} i18n={i18n} />;
          case "info.ftl":
            return <Info kcContext={kcContext} i18n={i18n} />;
          case "error.ftl":
            return <Error kcContext={kcContext} i18n={i18n} />;
          default:
            return defaultPage;
        }
      })()}
    </Suspense>
  );
}
