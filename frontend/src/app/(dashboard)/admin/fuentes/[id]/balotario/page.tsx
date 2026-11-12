"use client";

import { DialogoImportar } from "./_componentes/dialogo-importar";
import { ResumenBalotario } from "./_componentes/resumen-balotario";
import { TablaPreguntasBalotario } from "./_componentes/tabla-preguntas-balotario";
import { TablaProblemasBalotario } from "./_componentes/tabla-problemas-balotario";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { BotonReintentar } from "@/components/app/boton-reintentar";
import { Alerta } from "@/components/feedback/alerta";
import { Skeleton } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { useCarga } from "@/hooks/use-carga";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_PRINCIPAL, ACCION_SECUNDARIA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import {
  balotarioApi,
  type ImportacionCreada,
  PROBLEMAS_POR_PAGINA,
  type VistaPreviaBalotario,
} from "@/lib/serums-ia/balotario/api";
import { claveErrorBalotario, valoresDeError } from "@/lib/serums-ia/balotario/errores";
import { sePuedeImportar } from "@/lib/serums-ia/balotario/reglas";
import { ArrowLeftIcon, FileArrowDownIcon } from "@phosphor-icons/react";

/** Vista previa de la importación del balotario de una fuente (SIA-E3-03): analiza la versión vigente sin guardar nada. */
export default function ImportarBalotarioPage() {
  const { id } = useParams<{ id: string }>();
  const fuenteId = Number(id);
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();
  const [pagina, setPagina] = useState(1);
  const [porPagina, setPorPagina] = useState(PROBLEMAS_POR_PAGINA);
  const [importando, setImportando] = useState(false);
  const [creada, setCreada] = useState<ImportacionCreada | null>(null);
  // Al cambiar de página de problemas se sigue mostrando la anterior (atenuada) hasta que llega la nueva.
  const [mostrada, setMostrada] = useState<VistaPreviaBalotario | null>(null);

  const carga = useCarga(token && fuenteId > 0 ? `balotario-${fuenteId}-${pagina}-${porPagina}` : null, () =>
    balotarioApi.vistaPrevia(token ?? "", fuenteId, pagina, porPagina),
  );
  if (carga.datos && carga.datos !== mostrada) setMostrada(carga.datos);
  const vista = carga.datos ?? mostrada;

  const volver = (
    <Link
      href={`/admin/fuentes/${fuenteId}`}
      className="text-control text-muted-foreground hover:text-foreground inline-flex w-fit items-center gap-1.5"
    >
      <ArrowLeftIcon className="size-4" />
      {t("serumsIa.balotario.volver")}
    </Link>
  );

  if (carga.error && !vista) {
    return (
      <div className={CONTENEDOR_PAGINA}>
        {volver}
        <Alerta tono="error" accion={<BotonReintentar onReintentar={carga.recargar} />}>
          {t(claveErrorBalotario(carga.error, t.has), valoresDeError(carga.error))}
        </Alerta>
      </div>
    );
  }
  if (!vista || !token) {
    return (
      <div className={CONTENEDOR_PAGINA} aria-busy="true" aria-label={t("serumsIa.balotario.cargando")}>
        {volver}
        <Skeleton className="h-56 w-full rounded-xl" />
        <Skeleton className="h-72 w-full rounded-xl" />
      </div>
    );
  }

  /** Vuelve a analizar desde la primera página: tras un 409 la versión o la importación cambiaron. */
  function reanalizar() {
    setPagina(1);
    void carga.recargar();
  }

  return (
    <div className={CONTENEDOR_PAGINA}>
      {volver}
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-titulo-pagina text-foreground">{t("serumsIa.balotario.titulo")}</h1>
          <p className="text-control text-muted-foreground">{t("serumsIa.balotario.importarAyuda")}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button type="button" className={ACCION_SECUNDARIA} onClick={reanalizar}>
            {t("serumsIa.balotario.recargar")}
          </button>
          <button
            type="button"
            className={ACCION_PRINCIPAL}
            disabled={!sePuedeImportar(vista)}
            aria-describedby={vista.motivo_no_confirmable ? "balotario-no-confirmable" : undefined}
            onClick={() => setImportando(true)}
          >
            <FileArrowDownIcon className="size-4" />
            {t("serumsIa.balotario.importar")}
          </button>
        </div>
      </div>

      {vista.motivo_no_confirmable ? (
        <Alerta tono="aviso">
          <span id="balotario-no-confirmable">
            {t(`serumsIa.balotario.noConfirmable.${vista.motivo_no_confirmable}`)}
          </span>
        </Alerta>
      ) : null}
      {carga.error ? (
        <Alerta tono="error" accion={<BotonReintentar onReintentar={carga.recargar} />}>
          {t(claveErrorBalotario(carga.error, t.has), valoresDeError(carga.error))}
        </Alerta>
      ) : null}
      {creada ? (
        <Alerta tono="ok" titulo={t("serumsIa.balotario.exito.titulo")}>
          <p>{t("serumsIa.balotario.exito.texto", { creadas: creada.creadas, problemas: creada.con_problemas })}</p>
          <Link href="/admin/preguntas?origen=balotario" className="text-control font-medium underline">
            {t("serumsIa.balotario.exito.verListado")}
          </Link>
        </Alerta>
      ) : null}

      <ResumenBalotario vista={vista} />
      <TablaPreguntasBalotario preguntas={vista.preguntas} />
      <TablaProblemasBalotario
        problemas={vista.problemas}
        cargando={carga.cargando}
        onPagina={setPagina}
        onPorPagina={(n) => {
          setPorPagina(n);
          setPagina(1);
        }}
      />

      {importando ? (
        <DialogoImportar
          version={vista.fuente.version}
          onCerrar={() => setImportando(false)}
          onConfirmar={async (cuerpo) => {
            const r = await balotarioApi.confirmar(token, fuenteId, cuerpo);
            setCreada(r);
            setImportando(false);
            reanalizar();
            return r;
          }}
          onConflicto={(mensaje) => {
            setImportando(false);
            toast.error(mensaje);
            reanalizar();
          }}
        />
      ) : null}
    </div>
  );
}
