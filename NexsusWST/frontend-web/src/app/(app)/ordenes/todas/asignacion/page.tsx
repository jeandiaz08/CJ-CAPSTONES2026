import {Suspense} from "react";
import {OrdersAssignment} from "@/features/orders";
export default function Page(){return <Suspense fallback={<p>Cargando asignación…</p>}><OrdersAssignment/></Suspense>}
