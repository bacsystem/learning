import { LogoMarca } from "@/components/nav/logo";
import { cn } from "@/lib/utils";
import { GraduationCapIcon } from "@phosphor-icons/react/dist/ssr";

/** Marca de Capacitate para el sidebar (LogoMarca de khipu con el birrete). */
export function MarcaSidebar({ href = "/dashboards/analytics" }: { href?: string }) {
  return <LogoMarca nombre="Capacitate" href={href} icono={<GraduationCapIcon weight="fill" className="size-5" />} />;
}

/** Marca suelta (auth, landing, 404): icono en caja + nombre. */
export function Marca({ className, tamano = "md" }: { className?: string; tamano?: "md" | "lg" }) {
  return (
    <span className={cn("inline-flex items-center gap-2.5", className)} aria-label="Capacitate">
      <span
        className={cn(
          "from-primary to-primary-light text-primary-foreground flex shrink-0 items-center justify-center rounded-lg bg-linear-to-br shadow-xs",
          tamano === "lg" ? "size-10" : "size-8",
        )}
      >
        <GraduationCapIcon weight="fill" className={tamano === "lg" ? "size-6" : "size-5"} />
      </span>
      <span className={cn("text-foreground font-bold", tamano === "lg" ? "text-titulo-pagina" : "text-titulo")}>
        Capacitate
      </span>
    </span>
  );
}
