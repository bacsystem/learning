"use client";

import { useDiseno } from "@/components/diseno";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { cn } from "@/lib/utils";

/**
 * Select con los diseños disponibles (el atributo `data-diseno` de `<html>`). Mismo ancho y fondo que `ThemeToggle conTexto`,
 * para ir justo debajo de él en el menú de usuario. No pinta nada sin `DisenoProvider` o con un solo diseño.
 */
export function SelectorDiseno({ className }: { className?: string }) {
  const { diseno, setDiseno, disenos } = useDiseno();
  if (disenos.length < 2) return null;

  return (
    <Select
      value={diseno}
      onValueChange={(valor) => {
        if (valor) setDiseno(valor);
      }}
      items={disenos.map((d) => ({ value: d.valor, label: d.etiqueta }))}
    >
      <SelectTrigger
        aria-label="Diseño de la interfaz"
        size="campo"
        className={cn("bg-muted text-control w-full font-medium", className)}
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false}>
        {disenos.map((d) => (
          <SelectItem key={d.valor} value={d.valor} className="text-control font-medium">
            {d.etiqueta}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
