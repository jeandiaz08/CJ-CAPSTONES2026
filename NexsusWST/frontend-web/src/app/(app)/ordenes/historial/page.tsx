import { redirect } from "next/navigation";

// La ruta antigua del submenú «Registro histórico».
export default function Page() {
  redirect("/ordenes/registro");
}
