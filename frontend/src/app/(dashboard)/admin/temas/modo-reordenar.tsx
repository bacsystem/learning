"use client";

import { useTemas } from "./use-temas";
import { useTranslations } from "next-intl";
import { useMemo, useRef, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { SkeletonTabla } from "@/components/feedback/skeleton";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, TARJETA } from "@/lib/estilos";
import { type Tema, temasApi } from "@/lib/temas/api";
import { claveErrorTema, mensajeErrorTema } from "@/lib/temas/errores";
import { type Direccion, mover } from "@/lib/temas/reglas";
import { cn } from "@/lib/utils";
import { ArrowDownIcon, ArrowsDownUpIcon, ArrowUpIcon } from "@phosphor-icons/react";

const CLAVE_DESACTUALIZADO = "serumsIa.errores.orden_desactualizado";

/**
 * Modo «Reordenar» (E1-02 regla 6): todos los temas activos de la asignatura en una lista, con «Subir» y «Bajar» por
 * fila. Pide de nuevo los activos al entrar (sin paginación ni filtro de especialidad) y envía la lista completa.
 */
export function ModoReordenar({
  token,
  asignaturaId,
  onSalir,
  onGuardado,
}: {
  token: string;
  asignaturaId: number;
  onSalir: () => void;
  onGuardado: () => void;
}) {
  const t = useTranslations("dashboard");
  const filtro = useMemo(() => ({ asignaturaId, estado: "activo" as const }), [asignaturaId]);
  const { temas, cargando, error: errorCarga, recargar } = useTemas(token, filtro);

  // `null` = sin cambios: se muestra el orden del servidor. Así una recarga no deja un orden viejo.
  const [orden, setOrden] = useState<Tema[] | null>(null);
  const [guardando, setGuardando] = useState(false);
  const [errorGuardar, setErrorGuardar] = useState<unknown>(null);
  const [anuncio, setAnuncio] = useState("");
  const botones = useRef(new Map<string, HTMLButtonElement>());

  const lista = orden ?? temas;
  const cambiado = orden !== null && orden.some((tema, i) => tema.id !== temas[i]?.id);
  const desactualizado = claveErrorTema(errorGuardar) === CLAVE_DESACTUALIZADO;

  function moverFila(indice: number, direccion: Direccion) {
    const nueva = mover(lista, indice, direccion);
    const tema = lista[indice];
    setOrden(nueva);
    // El foco sigue a la fila movida; si llegó al extremo, pasa al otro botón (el suyo queda deshabilitado).
    const posicion = nueva.findIndex((x) => x.id === tema.id);
    const extremo = direccion === "subir" ? posicion === 0 : posicion === nueva.length - 1;
    const destino: Direccion = extremo ? (direccion === "subir" ? "bajar" : "subir") : direccion;
    requestAnimationFrame(() => botones.current.get(`${tema.id}-${destino}`)?.focus());
    setAnuncio(
      t("serumsIa.temas.reordenar.anuncio", {
        nombre: tema.nombre,
        posicion: posicion + 1,
        total: nueva.length,
      }),
    );
  }

  async function recargarOrden() {
    setOrden(null);
    setErrorGuardar(null);
    await recargar();
  }

  async function guardar() {
    setGuardando(true);
    setErrorGuardar(null);
    try {
      await temasApi.reordenarTemas(
        token,
        asignaturaId,
        lista.map((x) => x.id),
      );
      onGuardado();
    } catch (e) {
      setErrorGuardar(e);
    } finally {
      setGuardando(false);
    }
  }

  const registrar = (clave: string) => (el: HTMLButtonElement | null) => {
    if (el) botones.current.set(clave, el);
    else botones.current.delete(clave);
  };

  return (
    <section className={cn(TARJETA, "flex flex-col")} aria-labelledby="reordenar-titulo">
      <CabeceraSeccion
        icon={ArrowsDownUpIcon}
        titulo={<span id="reordenar-titulo">{t("serumsIa.temas.reordenar.titulo")}</span>}
        subtitulo={t("serumsIa.temas.reordenar.ayuda")}
      />
      <div className="flex flex-col gap-3 p-4">
        {errorCarga ? (
          <Alerta
            tono="error"
            accion={
              <button type="button" onClick={recargarOrden} className={ACCION_SECUNDARIA}>
                {t("comun-reintentar")}
              </button>
            }
          >
            {mensajeErrorTema(errorCarga, t, t("serumsIa.temas.errorCargar"))}
          </Alerta>
        ) : null}
        {errorGuardar ? (
          <Alerta
            tono={desactualizado ? "aviso" : "error"}
            accion={
              desactualizado ? (
                <button type="button" onClick={recargarOrden} className={ACCION_SECUNDARIA}>
                  {t("serumsIa.temas.reordenar.recargar")}
                </button>
              ) : undefined
            }
          >
            {mensajeErrorTema(errorGuardar, t, t("comun-error-guardar"))}
          </Alerta>
        ) : null}

        {cargando ? (
          <SkeletonTabla columnas={2} />
        ) : lista.length === 0 && !errorCarga ? (
          <p className="text-control text-muted-foreground">{t("serumsIa.temas.reordenar.sinActivos")}</p>
        ) : (
          <ol className="divide-border border-border divide-y rounded-lg border">
            {lista.map((tema, i) => (
              <li key={tema.id} className="flex items-center gap-3 px-3 py-2">
                <span className="text-etiqueta text-muted-foreground w-6 shrink-0 text-right font-mono tabular-nums">
                  {i + 1}
                </span>
                <span className="text-control text-foreground min-w-0 flex-1 font-medium break-words">
                  {tema.nombre}
                </span>
                <button
                  ref={registrar(`${tema.id}-subir`)}
                  type="button"
                  disabled={i === 0 || guardando}
                  onClick={() => moverFila(i, "subir")}
                  aria-label={t("serumsIa.temas.reordenar.subirTema", { nombre: tema.nombre })}
                  className={ACCION_SECUNDARIA}
                >
                  <ArrowUpIcon className="size-4" />
                  <span className="max-sm:sr-only">{t("serumsIa.temas.reordenar.subir")}</span>
                </button>
                <button
                  ref={registrar(`${tema.id}-bajar`)}
                  type="button"
                  disabled={i === lista.length - 1 || guardando}
                  onClick={() => moverFila(i, "bajar")}
                  aria-label={t("serumsIa.temas.reordenar.bajarTema", { nombre: tema.nombre })}
                  className={ACCION_SECUNDARIA}
                >
                  <ArrowDownIcon className="size-4" />
                  <span className="max-sm:sr-only">{t("serumsIa.temas.reordenar.bajar")}</span>
                </button>
              </li>
            ))}
          </ol>
        )}
        <p className="sr-only" aria-live="polite">
          {anuncio}
        </p>

        <div className="flex flex-wrap justify-end gap-2">
          <button type="button" onClick={onSalir} disabled={guardando} className={ACCION_SECUNDARIA}>
            {t("comun-cancelar")}
          </button>
          <BotonAsync
            pendiente={guardando}
            textoPendiente={t("comun-guardando")}
            disabled={!cambiado || desactualizado}
            onClick={guardar}
            className={ACCION_PRINCIPAL}
          >
            {t("serumsIa.temas.reordenar.guardar")}
          </BotonAsync>
        </div>
      </div>
    </section>
  );
}
