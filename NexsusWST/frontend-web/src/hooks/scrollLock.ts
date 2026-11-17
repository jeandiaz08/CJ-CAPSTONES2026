// Bloqueo de scroll del body compartido por modales y el menú móvil.
// Un contador: el primero que bloquea guarda el overflow original y el último que libera
// lo restaura, aunque se cierren en otro orden (Escape, clic afuera, navegación o dos
// modales apilados). Antes cada uno guardaba y restauraba el suyo y podía quedar "hidden".
let locks = 0;
let original = "";

export function lockScroll(): () => void {
  if (locks++ === 0) {
    original = document.body.style.overflow;
    document.body.style.overflow = "hidden";
  }
  let released = false;
  return () => {
    if (released) return;
    released = true;
    if (--locks === 0) document.body.style.overflow = original;
  };
}
