// ÚNICA lista de proveedores de la pantalla Integraciones: de aquí salen las tarjetas, los filtros
// y los nombres. `status`:
//   · supported: el backend lo acepta en POST /v1/integrations (ChannelProviders y CourierProviders
//     de orders-service/internal/application/integrations.go);
//   · soon: «Próximamente», sin adaptador;
//   · none: sin integración propia;
//   · server: se configura una sola vez en el servidor (variables de entorno), no por cliente.
// TODO(api): el backend no publica esta lista. Cuando Cristopher agregue un proveedor (por ejemplo
// shopify, que el API Mock de Ismael incluye), basta cambiar su `status` a "supported" aquí.
import type { BadgeTone } from "@/components/StatusBadge";
import type { IntegrationKind } from "@/services/integrations";

export type ProviderStatus = "supported" | "soon" | "none" | "server";
export type ProviderKind = IntegrationKind | "wms";
export type Provider = {
  code: string;
  kind: ProviderKind;
  name: string;
  description: string;
  status: ProviderStatus;
  logo?: string;
  note?: string; // pie de las tarjetas fijas
  card?: false; // sin tarjeta: solo aparece en la pestaña Clientes
};

export const PROVIDERS: Provider[] = [
  { code: "shopify", kind: "channel", name: "Shopify", description: "Sincroniza tus órdenes de Shopify en tiempo real.", status: "soon", logo: "/integrations/shopify.svg", note: "Sin adaptador" },
  { code: "woocommerce", kind: "channel", name: "WooCommerce", description: "Sincroniza tus órdenes de WooCommerce en tiempo real.", status: "soon", logo: "/integrations/woocommerce.svg", note: "Sin adaptador" },
  { code: "mercadolibre", kind: "channel", name: "Mercado Libre", description: "Sincroniza tus órdenes de Mercado Libre en tiempo real.", status: "supported", logo: "/integrations/mercadolibre.webp" },
  { code: "generic", kind: "channel", name: "API Nexus", description: "Formato propio de NexusWST para cualquier tienda o sistema.", status: "supported", logo: "/brand/nexuswst.png" },
  { code: "chilexpress", kind: "courier", name: "Chilexpress", description: "Recibe el seguimiento de tus entregas de Chilexpress.", status: "supported", logo: "/couriers/chilexpress.svg" },
  { code: "blueexpress", kind: "courier", name: "BlueExpress", description: "Recibe el seguimiento de tus entregas de BlueExpress.", status: "supported", logo: "/couriers/blueexpress.svg" },
  { code: "starken", kind: "courier", name: "Starken", description: "Recibe el seguimiento de tus entregas de Starken.", status: "supported", logo: "/couriers/starken.webp" },
  { code: "mercadoenvios", kind: "courier", name: "Mercado Envíos", description: "Entregas de las órdenes de Mercado Libre.", status: "none", logo: "/couriers/mercadoenvios.webp", note: "Llega por el canal Mercado Libre" },
  { code: "recibelo", kind: "courier", name: "Recíbelo", description: "Se retira en oficina; no tiene sincronización.", status: "none", logo: "/couriers/recibelo.webp", note: "Buscar en oficina" },
  { code: "generic", kind: "courier", name: "Courier genérico", description: "Seguimiento en el formato propio de NexusWST.", status: "supported", card: false },
  { code: "invas", kind: "wms", name: "Invas WMS", description: "Stock, validación y estados de preparación en tiempo real. Cuenta única WeStorage.", status: "server", logo: "/integrations/invas.svg", note: "Configurado en el servidor" },
];

// Un solo badge por estado y siempre el mismo texto.
export const STATUS_BADGE: Record<ProviderStatus, { label: string; tone: BadgeTone }> = {
  supported: { label: "Disponible", tone: "info" },
  server: { label: "Disponible", tone: "info" },
  soon: { label: "Próximamente", tone: "neutral" },
  none: { label: "Sin integración", tone: "neutral" },
};

export const KIND_LABELS: Record<IntegrationKind, string> = { channel: "Canal", courier: "Courier" };
export const SECTIONS: { kind: ProviderKind; title: string }[] = [
  { kind: "channel", title: "Canales" },
  { kind: "courier", title: "Couriers" },
  { kind: "wms", title: "Integraciones" },
];

export const findProvider = (code: string, kind: ProviderKind) => PROVIDERS.find((p) => p.code === code && p.kind === kind);
export const providerName = (code: string, kind: ProviderKind) => findProvider(code, kind)?.name ?? code;
// Proveedores que acepta el backend, para filtros y para «Conectar cliente».
export const supportedProviders = (kind?: IntegrationKind | "") =>
  PROVIDERS.filter((p) => p.status === "supported" && p.kind !== "wms" && (!kind || p.kind === kind));
