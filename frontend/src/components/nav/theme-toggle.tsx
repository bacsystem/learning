"use client";

import { useTheme } from "next-themes";
import { useEffect, useState } from "react";

import { PESO_ICONO_FUNCIONAL } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { MonitorIcon, MoonIcon, SunIcon } from "@phosphor-icons/react";

const OPCIONES = [
  { valor: "light", clave: "claro", Icono: SunIcon },
  { valor: "dark", clave: "oscuro", Icono: MoonIcon },
  { valor: "system", clave: "sistema", Icono: MonitorIcon },
] as const;

/** Textos de `ThemeToggle`: pasa los que quieras traducir, el resto queda en castellano. */
export type TextosThemeToggle = {
  /** `aria-label` del grupo de opciones. */
  temaInterfaz: string;
  claro: string;
  oscuro: string;
  sistema: string;
};

export const TEXTOS_THEME_TOGGLE_ES: TextosThemeToggle = {
  temaInterfaz: "Tema de la interfaz",
  claro: "Claro",
  oscuro: "Oscuro",
  sistema: "Sistema",
};

/**
 * Claro / oscuro / sistema como control segmentado en píldora. Por defecto solo iconos (nombre en `title` y `aria-label`):
 * en el menú de usuario el texto no entraba. `conTexto` muestra además la etiqueta, para espacios anchos.
 */
export function ThemeToggle({
  className,
  conTexto = false,
  textos: textosParciales,
}: {
  className?: string;
  conTexto?: boolean;
  textos?: Partial<TextosThemeToggle>;
}) {
  const textos = { ...TEXTOS_THEME_TOGGLE_ES, ...textosParciales };
  const { theme, setTheme } = useTheme();
  // El tema real solo se conoce en el cliente (localStorage / prefers-color-scheme);
  // hasta montar se pinta sin selección para no desajustar la hidratación.
  const [montado, setMontado] = useState(false);
  useEffect(() => setMontado(true), []);
  const actual = montado ? theme : undefined;

  return (
    <div
      role="radiogroup"
      aria-label={textos.temaInterfaz}
      className={cn(
        "bg-secondary inline-flex items-center gap-0.5 rounded-full p-[3px]",
        conTexto && "w-full",
        className,
      )}
    >
      {OPCIONES.map(({ valor, clave, Icono }) => {
        const activo = actual === valor;
        const etiqueta = textos[clave];
        return (
          <button
            key={valor}
            type="button"
            role="radio"
            aria-checked={activo}
            aria-label={etiqueta}
            title={etiqueta}
            onClick={() => setTheme(valor)}
            className={cn(
              "ease-salida flex h-7 items-center justify-center gap-1.5 rounded-full transition-colors duration-(--duracion-rapida)",
              conTexto ? "text-control min-w-0 flex-1 basis-0 px-2 font-medium" : "w-8",
              activo
                ? "bg-card text-foreground ring-border shadow-xs ring-1"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            <Icono weight={PESO_ICONO_FUNCIONAL} className="size-4 shrink-0" />
            {conTexto ? <span className="truncate">{etiqueta}</span> : null}
          </button>
        );
      })}
    </div>
  );
}
