import { Suspense } from "react";
import { ClientAdminView } from "@/features/clients";

export default function Page() {
  return (
    <Suspense fallback={<p className="muted">Cargando…</p>}>
      <ClientAdminView />
    </Suspense>
  );
}
