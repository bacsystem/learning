import type { ReactNode } from "react";

import { cn } from "@/lib/utils";

/** Atajo de teclado (`⌘K`, `Esc`, `Enter`). Mismo estilo que ya usaba `TopBar` armado a mano. */
export function Kbd({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <kbd
      className={cn(
        "border-border bg-card text-etiqueta text-muted-foreground/70 pointer-events-none inline-flex items-center rounded border px-1.5 py-0.5 font-mono shadow-xs",
        className,
      )}
    >
      {children}
    </kbd>
  );
}
