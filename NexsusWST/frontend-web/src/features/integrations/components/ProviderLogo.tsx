"use client";

import { useState } from "react";
import Image from "next/image";
import { Plug } from "lucide-react";
import type { Provider } from "../providers";

const initials = (name: string) => name.split(/\s+/).map((w) => w[0]).join("").slice(0, 2).toUpperCase();

// Logo oficial del proveedor; si no hay o no carga, un ícono genérico con sus iniciales.
export function ProviderLogo({ provider }: { provider: Provider }) {
  const [failed, setFailed] = useState(false);
  return provider.logo && !failed
    ? <Image className="integration-logo" src={provider.logo} alt="" width={120} height={48} onError={() => setFailed(true)} />
    : <span className="integration-logo-fallback" aria-hidden="true"><Plug size={16} />{initials(provider.name)}</span>;
}
