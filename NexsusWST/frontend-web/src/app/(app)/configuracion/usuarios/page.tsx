import { Suspense } from "react";
import { InternalUsersView } from "@/features/settings";

export default function Page() {
  return (
    <Suspense fallback={<p className="muted">Cargando…</p>}>
      <InternalUsersView />
    </Suspense>
  );
}
