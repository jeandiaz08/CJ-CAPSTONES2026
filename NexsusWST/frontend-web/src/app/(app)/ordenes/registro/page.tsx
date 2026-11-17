import { Suspense } from "react";
import { RegistryView } from "@/features/registry";

export default function Page() {
  return (
    <Suspense fallback={<p>Cargando registro…</p>}>
      <RegistryView />
    </Suspense>
  );
}
