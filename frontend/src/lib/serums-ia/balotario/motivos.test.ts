import en from "../../../i18n/messages/en.json";
import es from "../../../i18n/messages/es.json";
import { MOTIVOS_PROBLEMA } from "./api";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const CONTRATO = fileURLToPath(new URL("../../../../../docs/api/serums-ia-e3-01-e3-03-balotario.md", import.meta.url));

/** Motivos de la tabla de §2.4 del contrato: las filas con un código entre comillas inversas en la primera columna. */
function motivosDelContrato(): string[] {
  const seccion = readFileSync(CONTRATO, "utf8").split("### 2.4")[1]?.split("\n### ")[0] ?? "";
  return [...seccion.matchAll(/^\| `([a-z_]+)` \|/gm)].map((m) => m[1]);
}

describe("motivos de «con problemas»", () => {
  it("covers every motivo of the contract, plus sin_pasaje", () => {
    const delContrato = motivosDelContrato();
    expect(delContrato.length).toBeGreaterThanOrEqual(9);
    expect([...MOTIVOS_PROBLEMA].sort()).toEqual([...delContrato, "sin_pasaje"].sort());
  });

  it.each([
    ["es", es.dashboard.serumsIa.balotario.motivos],
    ["en", en.dashboard.serumsIa.balotario.motivos],
  ])("has a text in %s for each motivo and no extra one", (_, textos) => {
    expect(Object.keys(textos).sort()).toEqual([...MOTIVOS_PROBLEMA].sort());
    for (const motivo of MOTIVOS_PROBLEMA) {
      expect((textos as Record<string, string>)[motivo]).toBeTruthy();
    }
  });
});
