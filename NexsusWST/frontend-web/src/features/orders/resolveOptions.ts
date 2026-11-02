// Cómo se resuelve cada tipo de incidencia. Es la única tabla tipo → opciones:
// un tipo nuevo se agrega en el backend (domain/status.go) y con una fila aquí.
// Un tipo sin fila usa las opciones de «otro».
export type ResolveOption = "address" | "courier" | "split" | "retry" | "resume" | "cancel";

type ResolveConfig = { options: ResolveOption[]; resumeTitle?: string };

const RESOLVE_CONFIG: Record<string, ResolveConfig> = {
  datos: { options: ["address", "courier", "resume", "cancel"] },
  // Sin «resume»: reanudar sin cambios volvería a chocar con la cobertura o los bultos del courier.
  courier_cobertura: { options: ["address", "courier", "split", "cancel"] },
  courier_bultos: { options: ["courier", "split", "cancel"] },
  retiro: { options: ["address", "courier", "resume", "cancel"] },
  diferencia_empaque: { options: ["resume", "cancel"] },
  invas: { options: ["retry", "cancel"] },
  integracion: { options: ["retry", "resume", "cancel"] },
  cancelacion: { options: ["resume", "cancel"], resumeTitle: "Mantener la orden" },
  otro: { options: ["address", "courier", "resume", "cancel"] },
};

export function resolveConfig(kind?: string): ResolveConfig {
  return kind && Object.hasOwn(RESOLVE_CONFIG, kind) ? RESOLVE_CONFIG[kind] : RESOLVE_CONFIG.otro;
}
