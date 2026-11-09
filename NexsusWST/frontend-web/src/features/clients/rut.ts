// RUT chileno (módulo 11), igual que el Core (clients/domain/rut.go). El servidor vuelve a validar.
export function normalizeRut(raw: string): string | null {
  const clean = raw.replace(/[.\-\s]/g, "").toUpperCase();
  if (!/^\d{7,8}[\dK]$/.test(clean)) return null;
  const body = clean.slice(0, -1), dv = clean.slice(-1);
  let sum = 0, mul = 2;
  for (let i = body.length - 1; i >= 0; i--) {
    sum += Number(body[i]) * mul;
    mul = mul === 7 ? 2 : mul + 1;
  }
  const r = 11 - (sum % 11);
  const expected = r === 11 ? "0" : r === 10 ? "K" : String(r);
  if (dv !== expected) return null;
  return `${String(Number(body)).replace(/\B(?=(\d{3})+(?!\d))/g, ".")}-${dv}`;
}
