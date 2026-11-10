"use client";

import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";

import {
  CLAVE_FUENTE,
  CLAVE_PALETA,
  CLAVE_TONO_OSCURO,
  type Fuente,
  FUENTE_POR_DEFECTO,
  FUENTES,
  type Paleta,
  PALETA_POR_DEFECTO,
  PALETAS,
  type PreferenciasApariencia,
  TONO_OSCURO_POR_DEFECTO,
  type TonoOscuro,
  TONOS_OSCURO,
  valorValido,
} from "@/components/apariencia-opciones";

type ContextoApariencia = {
  paleta: string;
  setPaleta: (valor: string) => void;
  paletas: Paleta[];
  tonoOscuro: string;
  setTonoOscuro: (valor: string) => void;
  tonosOscuro: TonoOscuro[];
  fuente: string;
  setFuente: (valor: string) => void;
  fuentes: Fuente[];
};

// Sin proveedor no hay nada que elegir: las listas vacías hacen que SelectorApariencia y el menú de usuario no muestren nada.
const SIN_PROVEEDOR: ContextoApariencia = {
  paleta: PALETA_POR_DEFECTO,
  setPaleta: () => {},
  paletas: [],
  tonoOscuro: TONO_OSCURO_POR_DEFECTO,
  setTonoOscuro: () => {},
  tonosOscuro: [],
  fuente: FUENTE_POR_DEFECTO,
  setFuente: () => {},
  fuentes: [],
};
const Contexto = createContext<ContextoApariencia>(SIN_PROVEEDOR);

/** Paleta, tono oscuro y tipografía activos, y cómo cambiarlos. Fuera de `AparienciaProvider` devuelve listas vacías y no falla. */
export function useApariencia(): ContextoApariencia {
  return useContext(Contexto);
}

function guardar(clave: string, valor: string) {
  try {
    localStorage.setItem(clave, valor);
  } catch {
    // Sin localStorage (modo privado, almacenamiento bloqueado) la preferencia vale solo para esta visita.
  }
}

/**
 * Guarda la paleta, el tono oscuro y la tipografía elegidos en localStorage y los aplica como `data-paleta`,
 * `data-oscuro` y `data-fuente` en `<html>`. `inicial` (p. ej. el perfil del usuario leído en el servidor) gana sobre lo
 * guardado; `onCambio` recibe las tres preferencias en cada cambio para que la app las guarde en su backend. Para que
 * no haya parpadeo, pon `ScriptDiseno` (con el mismo `inicial`) en el `<head>`.
 */
export function AparienciaProvider({
  children,
  paletas = PALETAS,
  tonosOscuro = TONOS_OSCURO,
  fuentes = FUENTES,
  inicial,
  onCambio,
}: {
  children: ReactNode;
  paletas?: Paleta[];
  tonosOscuro?: TonoOscuro[];
  fuentes?: Fuente[];
  inicial?: Partial<PreferenciasApariencia>;
  onCambio?: (preferencias: PreferenciasApariencia) => void;
}) {
  const [paleta, setPaletaEstado] = useState(() => valorValido(inicial?.paleta, paletas, PALETA_POR_DEFECTO));
  const [tonoOscuro, setTonoEstado] = useState(() =>
    valorValido(inicial?.tonoOscuro, tonosOscuro, TONO_OSCURO_POR_DEFECTO),
  );
  const [fuente, setFuenteEstado] = useState(() => valorValido(inicial?.fuente, fuentes, FUENTE_POR_DEFECTO));
  const actual = useRef<PreferenciasApariencia>({ paleta, tonoOscuro, fuente });
  actual.current = { paleta, tonoOscuro, fuente };
  const avisar = useRef(onCambio);
  avisar.current = onCambio;

  // El valor real solo se conoce en el cliente; ScriptDiseno ya lo puso en <html> antes de pintar, aquí solo se lee.
  useEffect(() => {
    const d = document.documentElement.dataset;
    setPaletaEstado(valorValido(d.paleta, paletas, PALETA_POR_DEFECTO));
    setTonoEstado(valorValido(d.oscuro, tonosOscuro, TONO_OSCURO_POR_DEFECTO));
    setFuenteEstado(valorValido(d.fuente, fuentes, FUENTE_POR_DEFECTO));
  }, [paletas, tonosOscuro, fuentes]);

  const setPaleta = useCallback(
    (valor: string) => {
      if (!paletas.some((p) => p.valor === valor)) return;
      setPaletaEstado(valor);
      document.documentElement.dataset.paleta = valor;
      guardar(CLAVE_PALETA, valor);
      avisar.current?.({ ...actual.current, paleta: valor });
    },
    [paletas],
  );

  const setTonoOscuro = useCallback(
    (valor: string) => {
      if (!tonosOscuro.some((t) => t.valor === valor)) return;
      setTonoEstado(valor);
      document.documentElement.dataset.oscuro = valor;
      guardar(CLAVE_TONO_OSCURO, valor);
      avisar.current?.({ ...actual.current, tonoOscuro: valor });
    },
    [tonosOscuro],
  );

  const setFuente = useCallback(
    (valor: string) => {
      if (!fuentes.some((f) => f.valor === valor)) return;
      setFuenteEstado(valor);
      document.documentElement.dataset.fuente = valor;
      guardar(CLAVE_FUENTE, valor);
      avisar.current?.({ ...actual.current, fuente: valor });
    },
    [fuentes],
  );

  const valor = useMemo(
    () => ({ paleta, setPaleta, paletas, tonoOscuro, setTonoOscuro, tonosOscuro, fuente, setFuente, fuentes }),
    [paleta, setPaleta, paletas, tonoOscuro, setTonoOscuro, tonosOscuro, fuente, setFuente, fuentes],
  );
  return <Contexto.Provider value={valor}>{children}</Contexto.Provider>;
}
