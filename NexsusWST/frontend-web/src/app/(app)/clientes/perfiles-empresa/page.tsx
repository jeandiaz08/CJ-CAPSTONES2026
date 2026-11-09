import { Suspense } from "react";
import { CompanyProfilesView } from "@/features/clients";

export default function Page() {
  return (
    <Suspense fallback={<p className="muted">Cargando…</p>}>
      <CompanyProfilesView />
    </Suspense>
  );
}
