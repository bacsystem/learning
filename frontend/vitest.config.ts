import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

/**
 * Código de khipu copiado con su CLI (registry/registry.json del repo khipu): se prueba allí y aquí no se edita, así que
 * no cuenta para la cobertura de la app.
 */
const ARCHIVOS_KHIPU = [
  "src/components/{datos,feedback,formularios,nav,navegacion,patrones,ui}/**",
  "src/components/{apariencia-opciones,disenos}.ts",
  "src/components/{apariencia,diseno,movimiento,providers,script-diseno}.tsx",
  "src/lib/{calendario,estilos,exportar,formato,iconos,movimiento,navegacion,ordenar,paginacion,utils}.ts",
];

export default defineConfig({
  resolve: {
    alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
    coverage: {
      provider: "v8",
      reporter: ["text", "lcov"],
      reportsDirectory: "coverage",
      // Solo `src/`: `.next/` y `next-env.d.ts` (generados por Next) quedan fuera.
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/**/*.test.ts", "src/**/*.d.ts", ...ARCHIVOS_KHIPU],
    },
  },
});
