import { StrictMode, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { KcPage } from "./kc.gen";
import "@fontsource/inter/latin-400.css";
import "@fontsource/inter/latin-500.css";
import "@fontsource/inter/latin-600.css";
import "./login/theme.css";

const root = createRoot(document.getElementById("root")!);

async function render() {
  if (import.meta.env.DEV && !window.kcContext) {
    // Development fixtures never take over a real Keycloak context.
    const { Preview } = await import("./preview/Preview");
    root.render(<StrictMode><Preview /></StrictMode>);
  } else if (window.kcContext) {
    root.render(
      <StrictMode>
        <Suspense fallback={null}>
          <KcPage kcContext={window.kcContext} />
        </Suspense>
      </StrictMode>,
    );
  }
}

void render();
