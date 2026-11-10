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

// El secreto llega UNA sola vez, en la respuesta de crear o de rotar. No se guarda en estado
// global, en localStorage ni en logs: vive solo en el diálogo que lo muestra.
export type IntegrationWithSecret = Integration & { secret: string };
export type NewIntegration = { companyId: number; kind: IntegrationKind; provider: string; defaultCourier?: string };

export const createIntegration = (body: NewIntegration) =>
  api<IntegrationWithSecret>("orders", "/v1/integrations", { method: "POST", body: JSON.stringify(body) });
export const rotateIntegrationSecret = (id: number) =>
  api<IntegrationWithSecret>("orders", `/v1/integrations/${id}/rotate-secret`, { method: "POST" });
