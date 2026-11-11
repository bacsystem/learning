"use client";

import { InsigniaOficial, PillEstadoFuente, TextoProfesiones } from "./componentes";
import { DialogoSubida } from "./dialogos";
import { useOpcionesFuente } from "./use-opciones-fuente";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useFormatter, useTranslations } from "next-intl";
import { useEffect, useMemo, useState } from "react";

import { BotonReintentar } from "@/components/app/boton-reintentar";
import { PieTabla } from "@/components/app/pie-tabla";
import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { SkeletonTabla } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { Buscador } from "@/components/formularios/buscador";
import { SelectCampo } from "@/components/formularios/select-campo";
import { CONTENEDOR_TABLA } from "@/components/patrones/pie-tabla";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useCarga } from "@/hooks/use-carga";
import { useSondeo } from "@/hooks/use-sondeo";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_PRINCIPAL, CABECERA_TABLA, CONTENEDOR_PAGINA } from "@/lib/estilos";
import {
  type EstadoFuente,
  type FuenteListQuery,
  fuentesApi,
  type NuevaFuente,
  type TipoFuente,
} from "@/lib/serums-ia/fuentes/api";
import { mensajeErrorFuente } from "@/lib/serums-ia/fuentes/errores";
import { BUSQUEDA_MAX, enProceso, POR_PAGINA_FUENTES, REFRESCO_MS, TIPOS } from "@/lib/serums-ia/fuentes/reglas";
import { BooksIcon, PlusIcon } from "@phosphor-icons/react";

const ESTADOS: readonly EstadoFuente[] = ["procesando", "activa", "fallida", "retirada"];
/** Espera tras la última tecla antes de buscar. */
const ESPERA_BUSQUEDA_MS = 300;

export default function FuentesPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const formato = useFormatter();
  const toast = useToast();
  const router = useRouter();
  const opciones = useOpcionesFuente(token);

  const [estado, setEstado] = useState("");
  const [tipo, setTipo] = useState("");
  const [especialidadId, setEspecialidadId] = useState("");
  const [oficial, setOficial] = useState("");
  const [busqueda, setBusqueda] = useState("");
  const [q, setQ] = useState("");
  const [pagina, setPagina] = useState(1);
  const [porPagina, setPorPagina] = useState(POR_PAGINA_FUENTES);
  const [subiendo, setSubiendo] = useState(false);

  useEffect(() => {
    const id = setTimeout(() => setQ(busqueda.trim()), ESPERA_BUSQUEDA_MS);
    return () => clearTimeout(id);
  }, [busqueda]);

  const query = useMemo<FuenteListQuery>(
    () => ({
      estado: (estado || undefined) as EstadoFuente | undefined,
      tipo: (tipo || undefined) as TipoFuente | undefined,
      especialidad_id: especialidadId ? Number(especialidadId) : undefined,
      oficial: oficial ? oficial === "si" : undefined,
      q: q || undefined,
      page: pagina,
      per_page: porPagina,
    }),
    [estado, tipo, especialidadId, oficial, q, pagina, porPagina],
  );
  const clave = token ? JSON.stringify(query) : null;
  const listado = useCarga(clave, () => fuentesApi.listar(token ?? "", query));
  const filas = listado.datos?.data ?? [];
  const total = listado.datos?.total ?? 0;
  const ultimaPagina = Math.max(1, Math.ceil(total / porPagina));
  const hayFiltros = Boolean(estado || tipo || especialidadId || oficial || q);

  // E2-07 regla 6: mientras algo está en proceso, se vuelve a pedir la página actual cada 5 s (sin parpadeo).
  useSondeo(filas.some(enProceso), listado.refrescar, REFRESCO_MS);

  const filtrar = (cambiar: (v: string) => void) => (v: string) => {
    cambiar(v);
    setPagina(1);
  };

  async function subir(fuente: NuevaFuente) {
    if (!token) return;
    const creada = await fuentesApi.crear(token, fuente);
    setSubiendo(false);
    toast.ok(t("serumsIa.fuentes.subir.ok", { titulo: creada.titulo }));
    router.push(`/admin/fuentes/${creada.id}`);
  }

  const botonSubir = (
    <button type="button" onClick={() => setSubiendo(true)} className={ACCION_PRINCIPAL}>
      <PlusIcon weight="bold" className="size-4" />
      {t("serumsIa.fuentes.subir.titulo")}
    </button>
  );

  return (
    <div className={CONTENEDOR_PAGINA}>
      <div className="flex flex-wrap items-end gap-2">
        <div className="min-w-56 flex-1">
          <Buscador
            etiqueta={t("serumsIa.fuentes.buscar")}
            placeholder={t("serumsIa.fuentes.buscar")}
            valor={busqueda}
            onCambio={(v) => {
              setBusqueda(v.slice(0, BUSQUEDA_MAX));
              setPagina(1);
            }}
            maxLength={BUSQUEDA_MAX}
          />
        </div>
        {botonSubir}
      </div>
      <div className="grid grid-cols-2 gap-2 sm:flex sm:flex-wrap">
        <SelectCampo
          id="fuentes-estado"
          etiqueta={t("serumsIa.fuentes.filtro.estado")}
          className="sm:w-44"
          opciones={[
            { value: "", label: t("serumsIa.fuentes.filtro.todos") },
            ...ESTADOS.map((e) => ({ value: e, label: t(`serumsIa.fuentes.estado.${e}`) })),
          ]}
          valor={estado}
          onCambio={filtrar(setEstado)}
        />
        <SelectCampo
          id="fuentes-tipo"
          etiqueta={t("serumsIa.fuentes.filtro.tipo")}
          className="sm:w-44"
          opciones={[
            { value: "", label: t("serumsIa.fuentes.filtro.todos") },
            ...TIPOS.map((x) => ({ value: x, label: t(`serumsIa.fuentes.tipo.${x}`) })),
          ]}
          valor={tipo}
          onCambio={filtrar(setTipo)}
        />
        <SelectCampo
          id="fuentes-profesion"
          etiqueta={t("serumsIa.fuentes.filtro.profesion")}
          className="sm:w-52"
          opciones={[
            { value: "", label: t("serumsIa.fuentes.filtro.todas") },
            ...opciones.profesiones.map((p) => ({ value: String(p.id), label: p.nombre })),
          ]}
          valor={especialidadId}
          onCambio={filtrar(setEspecialidadId)}
        />
        <SelectCampo
          id="fuentes-oficial"
          etiqueta={t("serumsIa.fuentes.filtro.oficial")}
          className="sm:w-44"
          opciones={[
            { value: "", label: t("serumsIa.fuentes.filtro.todas") },
            { value: "si", label: t("serumsIa.fuentes.filtro.oficialSi") },
            { value: "no", label: t("serumsIa.fuentes.filtro.oficialNo") },
          ]}
          valor={oficial}
          onCambio={filtrar(setOficial)}
        />
      </div>

      {listado.error ? (
        <Alerta tono="error" accion={<BotonReintentar onReintentar={listado.recargar} />}>
          {mensajeErrorFuente(listado.error, t, t("serumsIa.fuentes.errorCargar"))}
        </Alerta>
      ) : listado.cargando && !listado.datos ? (
        <SkeletonTabla columnas={6} />
      ) : filas.length === 0 && !hayFiltros ? (
        <EstadoVacio icon={BooksIcon} titulo={t("serumsIa.fuentes.vacio.titulo")} accion={botonSubir}>
          {t("serumsIa.fuentes.vacio.texto")}
        </EstadoVacio>
      ) : (
        <div className={CONTENEDOR_TABLA}>
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className={CABECERA_TABLA}>{t("serumsIa.fuentes.col.titulo")}</TableHead>
                <TableHead className={CABECERA_TABLA}>{t("serumsIa.fuentes.col.tipo")}</TableHead>
                <TableHead className={CABECERA_TABLA}>{t("serumsIa.fuentes.col.profesiones")}</TableHead>
                <TableHead className={`${CABECERA_TABLA} text-right`}>{t("serumsIa.fuentes.col.version")}</TableHead>
                <TableHead className={`${CABECERA_TABLA} text-right`}>{t("serumsIa.fuentes.col.pasajes")}</TableHead>
                <TableHead className={CABECERA_TABLA}>{t("serumsIa.fuentes.col.estado")}</TableHead>
                <TableHead className={CABECERA_TABLA}>{t("serumsIa.fuentes.col.actualizada")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="text-control">
              {filas.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="text-muted-foreground py-8 text-center">
                    {t("serumsIa.fuentes.vacioFiltros")}
                  </TableCell>
                </TableRow>
              ) : (
                filas.map((f) => (
                  <TableRow key={f.id}>
                    {/* `min-w-48`: en celular la tabla se desplaza dentro de su contenedor y el título no se parte letra a letra. */}
                    <TableCell className="max-w-80 min-w-48 whitespace-normal">
                      <div className="grid min-w-0 gap-1 wrap-anywhere">
                        <Link
                          href={`/admin/fuentes/${f.id}`}
                          className="text-foreground font-medium underline-offset-4 hover:underline"
                        >
                          {f.titulo}
                        </Link>
                        {f.version_vigente?.oficial_verificada ? <InsigniaOficial /> : null}
                      </div>
                    </TableCell>
                    <TableCell>{t(`serumsIa.fuentes.tipo.${f.tipo}`)}</TableCell>
                    <TableCell className="text-muted-foreground max-w-56 whitespace-normal">
                      <TextoProfesiones aplicaATodas={f.aplica_a_todas} especialidades={f.especialidades} />
                    </TableCell>
                    <TableCell className="text-right font-mono tabular-nums">
                      {f.version_vigente ? `v${f.version_vigente.numero}` : "—"}
                    </TableCell>
                    <TableCell className="text-right font-mono tabular-nums">
                      {f.version_vigente ? formato.number(f.version_vigente.pasajes_count) : "—"}
                    </TableCell>
                    <TableCell>
                      <div className="grid gap-1">
                        <PillEstadoFuente estado={f.estado} />
                        {f.version_en_proceso ? (
                          <span className="text-etiqueta text-muted-foreground">
                            {t(`serumsIa.fuentes.versionEnProceso.${f.version_en_proceso.estado}`, {
                              numero: f.version_en_proceso.numero,
                            })}
                          </span>
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell className="text-muted-foreground font-mono">
                      {formato.dateTime(new Date(f.updated_at), { dateStyle: "medium", timeZone: "America/Lima" })}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
          <PieTabla
            desde={filas.length === 0 ? 0 : (pagina - 1) * porPagina + 1}
            hasta={(pagina - 1) * porPagina + filas.length}
            total={total}
            unidad={t("serumsIa.fuentes.unidad")}
            porPagina={porPagina}
            onPorPagina={(n) => {
              setPorPagina(n);
              setPagina(1);
            }}
            pagina={Math.min(pagina, ultimaPagina)}
            ultimaPagina={ultimaPagina}
            onPagina={setPagina}
          />
        </div>
      )}

      {subiendo ? (
        <DialogoSubida
          profesiones={opciones.profesiones}
          procesos={opciones.procesos}
          onCerrar={() => setSubiendo(false)}
          onSubir={subir}
        />
      ) : null}
    </div>
  );
}
