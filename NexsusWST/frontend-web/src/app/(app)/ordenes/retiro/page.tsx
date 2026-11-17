import { Suspense } from "react";
import { DispatchView } from "@/features/dispatch";

export default function Page() {
  return <Suspense fallback={<p>Cargando retiro…</p>}><DispatchView /></Suspense>;
}
