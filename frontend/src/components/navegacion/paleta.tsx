"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState } from "react";

import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Kbd } from "@/components/ui/kbd";
import type { IconoComponente } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";

export type ComandoPaleta = {
  id: string;
  label: string;
  /** Texto secundario (ruta, atajo, categoría). */
  detalle?: string;
  grupo?: string;
  icon?: IconoComponente;
  /** Navega a `href` o ejecuta `onSelect`. */
  href?: string;
  onSelect?: () => void;
  /** Palabras extra para la búsqueda. */
  claves?: string[];
};

function normalizar(s: string) {
  return s.normalize("NFD").replace(/[̀-ͯ]/g, "").toLowerCase();
}

/**
 * Paleta de comandos (⌘K / Ctrl+K): búsqueda difusa simple por etiqueta, detalle y claves, navegación con flechas y Enter.
 * Monta `<Paleta comandos={…} />` una vez en el layout; se abre con el atajo o con `open`/`onOpenChange` desde el buscador de la top bar.
 */
export function Paleta({
  comandos,
  open,
  onOpenChange,
  placeholder = "Buscar o ejecutar…",
}: {
  comandos: ComandoPaleta[];
  open?: boolean;
  onOpenChange?: (v: boolean) => void;
  placeholder?: string;
}) {
  const router = useRouter();
  const [interno, setInterno] = useState(false);
  const abierto = open ?? interno;
  const setAbierto = onOpenChange ?? setInterno;
  const [q, setQ] = useState("");
  const [idx, setIdx] = useState(0);
  const lista = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function atajo(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setAbierto(!abierto);
      }
    }
    window.addEventListener("keydown", atajo);
    return () => window.removeEventListener("keydown", atajo);
  }, [abierto, setAbierto]);

  const filtrados = useMemo(() => {
    const n = normalizar(q.trim());
    if (!n) return comandos;
    return comandos.filter((c) =>
      normalizar([c.label, c.detalle ?? "", c.grupo ?? "", ...(c.claves ?? [])].join(" ")).includes(n),
    );
  }, [comandos, q]);

  useEffect(() => setIdx(0), [q, abierto]);

  function ejecutar(c: ComandoPaleta) {
    setAbierto(false);
    setQ("");
    if (c.href) router.push(c.href);
    c.onSelect?.();
  }

  function teclas(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setIdx((i) => Math.min(i + 1, filtrados.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setIdx((i) => Math.max(i - 1, 0));
    } else if (e.key === "Enter" && filtrados[idx]) {
      e.preventDefault();
      ejecutar(filtrados[idx]);
    }
  }

  let grupoActual: string | undefined;

  return (
    <Dialog open={abierto} onOpenChange={setAbierto}>
      <DialogContent className="mt-[12vh] gap-0 self-start overflow-hidden p-0 sm:max-w-lg" showCloseButton={false}>
        <DialogTitle className="sr-only">Paleta de comandos</DialogTitle>
        <div className="border-border/60 flex items-center gap-2 border-b px-3">
          <MagnifyingGlassIcon weight="bold" className="text-muted-foreground/70 size-4 shrink-0" />
          <input
            autoFocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={teclas}
            placeholder={placeholder}
            aria-label={placeholder}
            className="text-campo md:text-control text-foreground placeholder:text-muted-foreground/70 h-11 w-full bg-transparent outline-none"
          />
          <Kbd className="bg-muted shadow-none">Esc</Kbd>
        </div>
        <div ref={lista} role="listbox" className="max-h-80 overflow-y-auto p-1">
          {filtrados.length === 0 ? (
            <p className="text-control text-muted-foreground px-3 py-6 text-center">Sin resultados para «{q}»</p>
          ) : null}
          {filtrados.map((c, i) => {
            const Icono = c.icon;
            const cabecera = c.grupo && c.grupo !== grupoActual ? c.grupo : null;
            grupoActual = c.grupo;
            return (
              <div key={c.id} className="contents">
                {cabecera ? (
                  <div className="text-etiqueta text-muted-foreground px-2 pt-2 pb-1 font-medium">{cabecera}</div>
                ) : null}
                <button
                  type="button"
                  role="option"
                  aria-selected={i === idx}
                  onMouseEnter={() => setIdx(i)}
                  onClick={() => ejecutar(c)}
                  className={cn(
                    "text-control flex w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left font-medium outline-none",
                    i === idx ? "bg-accent/70 text-accent-foreground" : "text-foreground",
                  )}
                >
                  {Icono ? (
                    <Icono
                      className={cn(
                        "size-3.5 shrink-0",
                        i === idx ? "text-accent-foreground" : "text-muted-foreground",
                      )}
                    />
                  ) : null}
                  <span className="min-w-0 flex-1 truncate">{c.label}</span>
                  {c.detalle ? (
                    <span className="text-etiqueta text-muted-foreground shrink-0 font-mono">{c.detalle}</span>
                  ) : null}
                </button>
              </div>
            );
          })}
        </div>
      </DialogContent>
    </Dialog>
  );
}
