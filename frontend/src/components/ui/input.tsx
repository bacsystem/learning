import * as React from "react";

import { cn } from "@/lib/utils";
import { Input as InputPrimitive } from "@base-ui/react/input";

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <InputPrimitive
      type={type}
      data-slot="input"
      className={cn(
        "h-control border-input bg-card text-campo md:text-control file:text-control file:text-foreground placeholder:text-texto-terciario focus-visible:border-acento-texto focus-visible:ring-acento/55 dark:focus-visible:border-acento dark:focus-visible:ring-acento/25 disabled:bg-secondary aria-invalid:border-destructive aria-invalid:ring-destructive/15 dark:aria-invalid:border-destructive/50 dark:aria-invalid:ring-destructive/40 w-full min-w-0 rounded-lg border px-3 py-1 transition-colors outline-none file:inline-flex file:h-6 file:border-0 file:bg-transparent file:font-medium focus-visible:ring-3 disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:ring-3",
        className,
      )}
      {...props}
    />
  );
}

export { Input };
