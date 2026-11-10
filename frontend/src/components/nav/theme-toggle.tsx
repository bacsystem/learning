"use client";

import { useTheme } from "next-themes";
import { useEffect, useState } from "react";

import { PESO_ICONO_FUNCIONAL } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { MonitorIcon, MoonIcon, SunIcon } from "@phosphor-icons/react";

const OPCIONES = [
  { valor: "light", etiqueta: "Claro", Icono: SunIcon },
  { valor: "dark", etiqueta: "Oscuro", Icono: MoonIcon },
  { valor: "system", etiqueta: "Sistema", Icono: MonitorIcon },
] as const;

/**
 * Claro / oscuro / sistema como control segmentado en píldora. Por defecto solo iconos (nombre en `title` y `aria-label`):
 * en el menú de usuario el texto no entraba. `conTexto` muestra además la etiqueta, para espacios anchos.
 */
export function ThemeToggle({ className, conTexto = false }: { className?: string; conTexto?: boolean }) {
  const { theme, setTheme } = useTheme();
  // El tema real solo se conoce en el cliente (localStorage / prefers-color-scheme);
  // hasta montar se pinta sin selección para no desajustar la hidratación.
  const [montado, setMontado] = useState(false);
  useEffect(() => setMontado(true), []);
  const actual = montado ? theme : undefined;

  return (
    <div
      role="radiogroup"
      aria-label="Tema de la interfaz"
      className={cn(
        "bg-secondary inline-flex items-center gap-0.5 rounded-full p-[3px]",
        conTexto && "w-full",
        className,
      )}
    >
      {OPCIONES.map(({ valor, etiqueta, Icono }) => {
        const activo = actual === valor;
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
