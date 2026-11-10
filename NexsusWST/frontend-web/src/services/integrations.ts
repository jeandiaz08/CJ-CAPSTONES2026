// Integraciones por cliente (orders-service, RF-05). Solo Administrador: el servidor responde 403 al resto.
import { api } from "./api";

export type IntegrationKind = "channel" | "courier";
export type Integration = {
  id: number;
  companyId: number;
  provider: string;
  kind: IntegrationKind;
  channel: string;
  defaultCourier: string;
  active: boolean;
  createdBy: string; // «user:{id} {nombre}»
  createdAt: string;
  webhookPath: string; // /webhooks/{provider}/{id}
  // TODO(api): el backend aún no entrega último evento, errores de las últimas 24 h ni últimos
  // eventos por conexión (los datos están en webhook_inbox). Ver backend_pendientes.md.
};

// El endpoint no filtra ni pagina: la búsqueda y los filtros se aplican en el navegador.
export const listIntegrations = () => api<{ data: Integration[] }>("orders", "/v1/integrations");
export const setIntegrationActive = (id: number, active: boolean) =>
  api<void>("orders", `/v1/integrations/${id}/${active ? "enable" : "disable"}`, { method: "POST" });
