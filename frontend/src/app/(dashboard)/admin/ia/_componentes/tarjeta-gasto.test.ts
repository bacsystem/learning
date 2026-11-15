import en from "../../../../../i18n/messages/en.json";
import es from "../../../../../i18n/messages/es.json";
import { TarjetaGasto } from "./tarjeta-gasto";
import { type AbstractIntlMessages, NextIntlClientProvider } from "next-intl";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { GastoIa } from "@/lib/serums-ia/ia/api";

const GASTO: GastoIa = {
  mes: "2026-11",
  presupuesto_usd: 40,
  gastado_usd: 12.5,
  porcentaje: 31.25,
  alerta_80_at: null,
  agotado: false,
  reinicia_at: "2026-12-01T05:00:00Z",
  por_operacion: [{ operacion: "clasificar", invocaciones: 3, desde_cache: 1, costo_usd: 1.5 } as never],
};

const pagina = (locale: "es" | "en", messages: AbstractIntlMessages) =>
  renderToStaticMarkup(
    createElement(NextIntlClientProvider, {
      locale,
      messages,
      children: createElement(TarjetaGasto, { gasto: GASTO }),
    }),
  );

describe("TarjetaGasto", () => {
  it("writes the amounts with the khipu recipe, the same as the rest of the admin, in both languages", () => {
    for (const html of [pagina("es", es), pagina("en", en)]) {
      expect(html).toContain("$ 40.00");
      expect(html).toContain("$ 12.50");
      expect(html).toContain("$ 1.50");
      expect(html).not.toContain("US$");
    }
  });
});
