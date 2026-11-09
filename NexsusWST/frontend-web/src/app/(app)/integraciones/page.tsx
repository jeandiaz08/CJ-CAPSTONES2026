import { Suspense } from "react";
import { IntegrationsSoon } from "@/features/clients";

export default function Page() {
  return (
    <Suspense fallback={<p className="muted">Cargando…</p>}>
      <IntegrationsSoon />
    </Suspense>
  );
}
