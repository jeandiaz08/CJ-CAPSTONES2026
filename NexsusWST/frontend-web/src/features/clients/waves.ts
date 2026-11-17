// Ola por defecto de una empresa (PV-19). NULL en la base = «General (según reglas)»: sin ola fija,
// el motor usa las reglas de asignación. En el formulario se elige siempre (no puede quedar vacía).
export const GENERAL_WAVE_LABEL = "General (según reglas)";
export const GENERAL_WAVE_VALUE = "general"; // valor del <select> y del filtro ?wave=general

export const waveLabel = (code: string | null | undefined) => (code ? code : GENERAL_WAVE_LABEL);

export type WaveOption = { value: string; label: string };

// Opciones: «General (según reglas)» y los códigos de las olas de Invas, más el código actual si ya
// no viene en la lista (para no perderlo al editar).
export function waveOptions(waves: { code: string; name: string }[], current?: string | null): WaveOption[] {
  const codes = new Map<string, string>();
  for (const w of waves) if (w.code && !codes.has(w.code)) codes.set(w.code, w.name && w.name !== w.code ? `${w.code} · ${w.name}` : w.code);
  if (current && !codes.has(current)) codes.set(current, current);
  return [{ value: GENERAL_WAVE_VALUE, label: GENERAL_WAVE_LABEL }, ...[...codes].map(([value, label]) => ({ value, label }))];
}
