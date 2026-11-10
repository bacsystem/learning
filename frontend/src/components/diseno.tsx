"use client";

import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useState } from "react";

import { CLAVE_DISENO, type Diseno, DISENO_POR_DEFECTO, DISENOS } from "@/components/disenos";

type ContextoDiseno = {
  /** Diseño activo (`valor`). */
  diseno: string;
  setDiseno: (valor: string) => void;
  disenos: Diseno[];
};

// Sin proveedor no hay nada que elegir: `disenos` vacío hace que SelectorDiseno y el menú de usuario no muestren nada.
const SIN_PROVEEDOR: ContextoDiseno = { diseno: DISENO_POR_DEFECTO, setDiseno: () => {}, disenos: [] };
const Contexto = createContext<ContextoDiseno>(SIN_PROVEEDOR);

/** Diseño activo y función para cambiarlo. Fuera de `DisenoProvider` devuelve una lista vacía y no falla. */
export function useDiseno(): ContextoDiseno {
  return useContext(Contexto);
}

/**
 * Guarda el diseño elegido en localStorage y lo aplica como `data-diseno` en `<html>`. Va junto a `ThemeProvider`
 * (next-themes no sirve para esto: solo maneja un atributo). Para que no haya parpadeo, pon `ScriptDiseno` en el `<head>`.
 */
export function DisenoProvider({
  children,
  disenos = DISENOS,
  porDefecto = DISENO_POR_DEFECTO,
}: {
  children: ReactNode;
  disenos?: Diseno[];
  porDefecto?: string;
}) {
  const [diseno, setDisenoEstado] = useState(porDefecto);

  // El diseño real solo se conoce en el cliente; ScriptDiseno ya lo puso en <html> antes de pintar, aquí solo se lee.
  useEffect(() => {
    const guardado = document.documentElement.dataset.diseno;
    if (guardado && disenos.some((d) => d.valor === guardado)) setDisenoEstado(guardado);
  }, [disenos]);

  const setDiseno = useCallback(
    (valor: string) => {
      if (!disenos.some((d) => d.valor === valor)) return;
      setDisenoEstado(valor);
      document.documentElement.dataset.diseno = valor;
      try {
        localStorage.setItem(CLAVE_DISENO, valor);
      } catch {
        // Sin localStorage (modo privado, almacenamiento bloqueado) el diseño vale solo para esta visita.
      }
    },
    [disenos],
  );

  const valor = useMemo(() => ({ diseno, setDiseno, disenos }), [diseno, setDiseno, disenos]);
  return <Contexto.Provider value={valor}>{children}</Contexto.Provider>;
}
