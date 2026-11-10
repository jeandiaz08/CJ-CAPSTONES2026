import { Suspense } from "react";
import { IntegrationsView } from "@/features/integrations";

export default function Page() {
  return (
    <Suspense fallback={<p className="muted">Cargando…</p>}>
      <IntegrationsView />
    </Suspense>
  );
}
