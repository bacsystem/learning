"use client";

import { useTranslations } from "next-intl";
import { type FormEvent, useEffect, useRef, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { Spinner } from "@/components/feedback/spinner";
import { Buscador } from "@/components/formularios/buscador";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, TARJETA } from "@/lib/estilos";
import { type PasajeEncontrado, pasajesApi } from "@/lib/serums-ia/pasajes/api";
import { CONSULTA_MAX, CONSULTA_MIN, consultaValida } from "@/lib/serums-ia/revision/reglas";
import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";

type Resultado =
  | { tipo: "inicial" }
  | { tipo: "buscando" }
  | { tipo: "error" }
  | { tipo: "lista"; pasajes: PasajeEncontrado[] };

/** Buscador de pasajes de las fuentes activas (E3-07 §2), con la acción de añadir cada uno como cita. */
export function BuscadorPasajes({
  citados,
  llena,
  ocupado,
  onAnadir,
}: {
  /** Ids de los pasajes que la pregunta ya cita: reenviarlos daría 422. */
  citados: ReadonlySet<number>;
  llena: boolean;
  ocupado: boolean;
  onAnadir: (pasajeId: number) => void;
}) {
  const t = useTranslations("dashboard");
  const { token } = useAuth();
  const [texto, setTexto] = useState("");
  const [resultado, setResultado] = useState<Resultado>({ tipo: "inicial" });
  const pedido = useRef<AbortController | null>(null);

  useEffect(() => () => pedido.current?.abort(), []);

  const consulta = consultaValida(texto);

  async function buscar(e: FormEvent) {
    e.preventDefault();
    if (!token || !consulta) return;
    pedido.current?.abort();
    const actual = new AbortController();
    pedido.current = actual;
    setResultado({ tipo: "buscando" });
    try {
      const pasajes = await pasajesApi.buscar(token, consulta, actual.signal);
      if (!actual.signal.aborted) setResultado({ tipo: "lista", pasajes });
    } catch {
      // Una búsqueda cancelada por otra más reciente no es un error de la pantalla.
      if (!actual.signal.aborted) setResultado({ tipo: "error" });
    }
  }

  return (
    <section aria-labelledby="buscador-titulo" className={`${TARJETA} grid gap-3 p-4`}>
      <h2 id="buscador-titulo" className="text-titulo text-foreground">
        {t("serumsIa.revision.citas.buscador.titulo")}
      </h2>
      <form onSubmit={buscar} className="flex flex-wrap items-end gap-2" role="search">
        <div className="min-w-0 flex-1 basis-64">
          <Buscador
            variante="campo"
            etiqueta={t("serumsIa.revision.citas.buscador.etiqueta")}
            placeholder={t("serumsIa.revision.citas.buscador.placeholder")}
            valor={texto}
            onCambio={setTexto}
            maxLength={CONSULTA_MAX}
            aria-describedby="buscador-ayuda"
          />
        </div>
        <button type="submit" className={ACCION_PRINCIPAL} disabled={!consulta || resultado.tipo === "buscando"}>
          <MagnifyingGlassIcon weight="bold" className="size-4" />
          {t("serumsIa.revision.citas.buscador.buscar")}
        </button>
      </form>
      <p id="buscador-ayuda" className="text-etiqueta text-muted-foreground">
        {t("serumsIa.revision.citas.buscador.ayuda", { min: CONSULTA_MIN, max: CONSULTA_MAX })}
      </p>

      <div aria-live="polite">
        {resultado.tipo === "buscando" ? (
          <p className="text-control text-muted-foreground flex items-center gap-2">
            <Spinner tamano="sm" />
            {t("serumsIa.revision.citas.buscador.buscando")}
          </p>
        ) : null}
        {resultado.tipo === "error" ? (
          <Alerta tono="error">{t("serumsIa.revision.citas.buscador.errorBusqueda")}</Alerta>
        ) : null}
        {resultado.tipo === "lista" && resultado.pasajes.length === 0 ? (
          <p className="text-control text-muted-foreground">{t("serumsIa.revision.citas.buscador.sinResultados")}</p>
        ) : null}
      </div>

      {resultado.tipo === "lista" && resultado.pasajes.length > 0 ? (
        <>
          <p className="text-etiqueta text-muted-foreground">
            {t("serumsIa.revision.citas.buscador.resultados", { n: resultado.pasajes.length })}
          </p>
          <ul className="grid gap-2">
            {resultado.pasajes.map((p) => (
              <li key={p.pasaje_id} className="border-border grid gap-1 rounded-xl border p-3">
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div className="min-w-0">
                    <p className="text-cuerpo text-foreground font-semibold">{p.fuente_titulo}</p>
                    <p className="text-etiqueta text-muted-foreground">
                      {t("serumsIa.revision.citas.version", { n: p.version })}
                      {p.seccion ? ` · ${t("serumsIa.revision.citas.seccion", { seccion: p.seccion })}` : ""}
                    </p>
                  </div>
                  {citados.has(p.pasaje_id) ? (
                    <span className="text-etiqueta text-muted-foreground">
                      {t("serumsIa.revision.citas.buscador.yaCitado")}
                    </span>
                  ) : (
                    <button
                      type="button"
                      className={ACCION_SECUNDARIA}
                      disabled={ocupado}
                      aria-label={t("serumsIa.revision.citas.buscador.anadirDe", { fuente: p.fuente_titulo })}
                      onClick={() => onAnadir(p.pasaje_id)}
                    >
                      <PlusIcon weight="bold" className="size-4" />
                      {t("serumsIa.revision.citas.buscador.anadir")}
                    </button>
                  )}
                </div>
                <p className="text-control text-muted-foreground">{p.extracto}</p>
              </li>
            ))}
          </ul>
          {llena ? (
            <p className="text-etiqueta text-muted-foreground">
              {t("serumsIa.revision.citas.buscador.limiteAlcanzado")}
            </p>
          ) : null}
        </>
      ) : null}
    </section>
  );
}
