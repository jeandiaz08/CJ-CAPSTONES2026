"use client";

import { useEffect, useState } from "react";

// Devuelve el valor recién cuando deja de cambiar durante `delay` ms.
export function useDebounce<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);
  return debounced;
}
