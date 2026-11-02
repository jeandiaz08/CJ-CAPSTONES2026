import { Suspense, type FormEvent } from "react";
import KcPage from "../login/KcPage";
import { getPreviewContext, scenarios, type Scenario } from "./kcContextMocks";
import "./preview.css";

export function Preview() {
  const params = new URLSearchParams(window.location.search);
  const requested = params.get("screen") ?? "login";
  if (requested === "gallery") return <Gallery />;
  const scenario: Scenario = requested in scenarios ? requested as Scenario : "login";
  const context = getPreviewContext(scenario);

  const onSubmit = (event: FormEvent) => {
    // Fixtures cannot send credentials or mutate the running realm.
    event.preventDefault();
    const next = scenario.startsWith("reset") ? "check-email"
      : scenario.startsWith("new-password") ? "info"
      : scenario.startsWith("login-microsoft") ? "login-microsoft-error" : "login-error";
    window.location.assign(`?screen=${next}`);
  };

  return (
    <>
      <div onSubmitCapture={onSubmit}>
        <Suspense fallback={<p>Cargando vista previa…</p>}><KcPage kcContext={context} /></Suspense>
      </div>
      {!params.has("capture") && (
        <a className="preview-switcher" href="?screen=gallery">Ver pantallas y estados</a>
      )}
    </>
  );
}

function Gallery() {
  return (
    <main className="preview-gallery">
      <p className="preview-eyebrow">NexusWST · revisión visual</p>
      <h1>Login de WeStorage</h1>
      <p>Vistas de desarrollo con datos de ejemplo. Los formularios de esta galería no acceden al realm.</p>
      <p>Revisar en 360, 768 y 1280 px. En Keycloak real, los mensajes y las reglas los entrega el servidor.</p>
      <nav aria-label="Pantallas del tema">
        {Object.entries(scenarios).map(([key, label]) => (
          <a key={key} href={`?screen=${key}`}>{label}<span aria-hidden="true">→</span></a>
        ))}
      </nav>
    </main>
  );
}
