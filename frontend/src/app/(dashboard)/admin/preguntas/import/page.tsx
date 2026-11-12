"use client";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import React, { useEffect, useRef, useState } from "react";

import { Campo } from "@/components/app/campo";
import { PieTabla } from "@/components/app/pie-tabla";
import { Alerta } from "@/components/feedback/alerta";
import { SelectCampo } from "@/components/formularios/select-campo";
import { ZonaArchivos } from "@/components/formularios/zona-archivos";
import { Pasos } from "@/components/navegacion/pasos";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraPagina, EnlaceVolver } from "@/components/patrones/cabecera-pagina";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { FilaMetricas, Metrica } from "@/components/patrones/metricas";
import { CONTENEDOR_TABLA, FilaVacia } from "@/components/patrones/pie-tabla";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { Separator } from "@/components/ui/separator";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useAuth } from "@/lib/auth/auth-context";
import { catalogsApi, Proceso, ProcesoAsignaturaItem } from "@/lib/catalogs/api";
import {
  ACCION_SECUNDARIA,
  BOTON_PRIMARIO,
  BOTON_SECUNDARIO,
  CABECERA_TABLA,
  CONTENEDOR_PAGINA,
  TARJETA,
} from "@/lib/estilos";
import { ImportResult, PreviewResponse, questionsApi } from "@/lib/questions/api";
import { cn } from "@/lib/utils";
import {
  ArrowCounterClockwiseIcon,
  CheckCircleIcon,
  DownloadSimpleIcon,
  EyeIcon,
  FileTextIcon,
  ListChecksIcon,
  TableIcon,
  UploadSimpleIcon,
  WarningCircleIcon,
  XCircleIcon,
} from "@phosphor-icons/react";

const TEMPLATE_HEADERS = [
  "enunciado",
  "tipo",
  "dificultad",
  "grado_id",
  "especialidad_id",
  "resp_1",
  "correcta_1",
  "resp_2",
  "correcta_2",
  "resp_3",
  "correcta_3",
  "resp_4",
  "correcta_4",
  "resp_5",
  "correcta_5",
  "resp_6",
  "correcta_6",
  "retroalimentacion",
  "referencia",
];

/** Columnas de la plantilla con la clave i18n de su descripción. */
const COLUMN_HINTS = [
  { label: "enunciado", desc: "admin-preguntas-import-col-enunciado" },
  { label: "tipo", desc: "admin-preguntas-import-col-tipo" },
  { label: "dificultad", desc: "admin-preguntas-import-col-dificultad" },
  { label: "grado_id", desc: "admin-preguntas-import-col-id" },
  { label: "especialidad_id", desc: "admin-preguntas-import-col-id" },
  { label: "resp_1 … resp_6", desc: "admin-preguntas-import-col-resp" },
  { label: "correcta_1 … correcta_6", desc: "admin-preguntas-import-col-correcta" },
  { label: "retroalimentacion", desc: "admin-preguntas-import-col-opcional" },
  { label: "referencia", desc: "admin-preguntas-import-col-opcional" },
];

const DIFICULTAD_TONO: Record<string, "ok" | "aviso" | "error"> = { facil: "ok", medio: "aviso", dificil: "error" };

/** Texto de ayuda con los valores entre comillas invertidas en mono (`unica`). */
function DescripcionColumna({ texto }: { texto: string }) {
  return (
    <>
      {texto.split(/(`[^`]+`)/).map((parte, i) =>
        parte.startsWith("`") ? (
          <code key={i} className="text-foreground font-mono">
            {parte.slice(1, -1)}
          </code>
        ) : (
          <React.Fragment key={i}>{parte}</React.Fragment>
        ),
      )}
    </>
  );
}

export default function ImportPreguntasPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const router = useRouter();
  const steps = [
    t("admin-preguntas-import-paso-configurar"),
    t("admin-preguntas-import-vista-previa"),
    t("admin-preguntas-import-paso-resultado"),
  ];

  const [step, setStep] = useState(0);

  // Step 0 state
  const [procesos, setProcesos] = useState<Proceso[]>([]);
  const [asignaturas, setAsignaturas] = useState<ProcesoAsignaturaItem[]>([]);
  const [procesoID, setProcesoID] = useState<number>(0);
  const [asignaturaID, setAsignaturaID] = useState<number>(0);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [previewing, setPreviewing] = useState(false);

  // Step 1 state
  const [preview, setPreview] = useState<PreviewResponse | null>(null);
  const [importing, setImporting] = useState(false);
  const [showErrorsOnly, setShowErrorsOnly] = useState(false);
  const [previewPage, setPreviewPage] = useState(0);
  const [previewPageSize, setPreviewPageSize] = useState(10);

  // Step 2 state
  const [result, setResult] = useState<ImportResult | null>(null);

  const [error, setError] = useState<string | null>(null);
  // Solo se aplica la respuesta de la última petición de asignaturas (cambiar rápido de proceso no deja las de otro).
  const asignaturasReq = useRef(0);

  useEffect(() => {
    if (!token) return;
    catalogsApi
      .listProcesos(token, {})
      .then(setProcesos)
      .catch(() => {});
  }, [token]);

  useEffect(() => {
    const reqId = ++asignaturasReq.current;
    if (!token || !procesoID) {
      setAsignaturas([]);
      setAsignaturaID(0);
      return;
    }
    catalogsApi
      .listProcesoAsignaturas(token, procesoID)
      .then((data) => {
        if (reqId === asignaturasReq.current) setAsignaturas(data);
      })
      .catch(() => {});
  }, [token, procesoID]);

  const acceptFile = (file: File | null) => {
    setSelectedFile(file);
    if (file) setError(null);
  };

  const handlePreview = async () => {
    if (!token || !selectedFile || !procesoID || !asignaturaID) return;
    setPreviewing(true);
    setError(null);
    try {
      const res = await questionsApi.previewPreguntas(token, selectedFile);
      setPreview(res);
      setShowErrorsOnly(false);
      setPreviewPage(0);
      setStep(1);
    } catch (e: unknown) {
      setError((e as { message?: string }).message || t("admin-preguntas-import-error-procesar"));
    } finally {
      setPreviewing(false);
    }
  };

  const handleImport = async () => {
    if (!token || !selectedFile || !procesoID || !asignaturaID) return;
    setImporting(true);
    setError(null);
    try {
      const res = await questionsApi.importPreguntas(token, selectedFile, procesoID, asignaturaID);
      setResult(res);
      setStep(2);
    } catch (e: unknown) {
      // 400: the backend explains why (subject not in this process, foreign tags…).
      setError((e as { message?: string }).message || t("admin-preguntas-import-error-importar"));
    } finally {
      setImporting(false);
    }
  };

  const handleReset = () => {
    setPreview(null);
    setResult(null);
    setSelectedFile(null);
    setError(null);
    setStep(0);
  };

  const downloadTemplate = () => {
    const csv = TEMPLATE_HEADERS.join(",") + "\n";
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "plantilla_preguntas.csv";
    a.click();
    URL.revokeObjectURL(url);
  };

  const previewStats = preview
    ? {
        unica: preview.rows.filter((r) => r.tipo === "unica").length,
        multiple: preview.rows.filter((r) => r.tipo === "multiple").length,
        facil: preview.rows.filter((r) => r.dificultad === "facil").length,
        medio: preview.rows.filter((r) => r.dificultad === "medio").length,
        dificil: preview.rows.filter((r) => r.dificultad === "dificil").length,
        conRetro: preview.rows.filter((r) => r.has_retro).length,
      }
    : null;

  const filteredRows = preview
    ? showErrorsOnly
      ? preview.rows.filter((r) => r.estado === "error")
      : preview.rows
    : [];

  const ultimaPagina = Math.max(1, Math.ceil(filteredRows.length / previewPageSize));
  const paginaActual = Math.min(previewPage, ultimaPagina - 1);
  const pagedRows = filteredRows.slice(paginaActual * previewPageSize, (paginaActual + 1) * previewPageSize);

  const procesoNombre = procesos.find((p) => p.id === procesoID)?.nombre;
  const asignaturaNombre = asignaturas.find((a) => a.asignatura_id === asignaturaID)?.nombre;

  return (
    <div className={CONTENEDOR_PAGINA}>
      {/* ── Header ── */}
      <CabeceraPagina
        volver={<EnlaceVolver href="/admin/preguntas">{t("menu-admin-preguntas")}</EnlaceVolver>}
        titulo={t("admin-preguntas-import-titulo")}
        descripcion={t("admin-preguntas-import-descripcion")}
        acciones={
          <button type="button" className={ACCION_SECUNDARIA} onClick={downloadTemplate}>
            <DownloadSimpleIcon className="size-4" />
            {t("admin-preguntas-import-descargar-plantilla")}
          </button>
        }
      />

      {/* ── Stepper ── */}
      <div className={cn(TARJETA, "px-4 py-4")}>
        <Pasos pasos={steps.map((titulo) => ({ titulo }))} actual={step} className="mx-auto max-w-2xl" />
      </div>

      {error ? <Alerta tono="error">{error}</Alerta> : null}

      {/* ══════════ STEP 0 — Configurar ══════════ */}
      {step === 0 && (
        <div className="grid items-start gap-3 lg:grid-cols-[minmax(0,1fr)_380px]">
          <section className={TARJETA}>
            <CabeceraSeccion
              icon={UploadSimpleIcon}
              titulo={t("admin-preguntas-import-destino-titulo")}
              subtitulo={t("admin-preguntas-import-destino-subtitulo")}
            />
            <div className="grid gap-4 px-4 pt-2 pb-4">
              <div className="grid gap-3 sm:grid-cols-2">
                <Campo id="import-proceso" etiqueta={t("admin-preguntas-proceso")}>
                  <SelectCampo
                    id="import-proceso"
                    placeholder={t("admin-preguntas-seleccionar-proceso")}
                    valor={procesoID ? String(procesoID) : ""}
                    onCambio={(v) => {
                      // H19: la asignatura elegida era del proceso anterior.
                      setProcesoID(Number(v) || 0);
                      setAsignaturaID(0);
                    }}
                    opciones={procesos.map((p) => ({ value: String(p.id), label: p.nombre }))}
                    required
                  />
                </Campo>
                <Campo id="import-asignatura" etiqueta={t("admin-preguntas-asignatura")}>
                  <SelectCampo
                    id="import-asignatura"
                    placeholder={t("admin-seleccionar-asignatura")}
                    valor={asignaturaID ? String(asignaturaID) : ""}
                    onCambio={(v) => setAsignaturaID(Number(v) || 0)}
                    opciones={asignaturas.map((a) => ({ value: String(a.asignatura_id), label: a.nombre }))}
                    disabled={!procesoID}
                    required
                  />
                </Campo>
              </div>

              <ZonaArchivos
                id="import-archivo"
                archivo={selectedFile}
                onCambio={acceptFile}
                accept=".xlsx,.csv"
                titulo={t("admin-preguntas-import-archivo-titulo")}
                ayuda={t("admin-preguntas-import-archivo-ayuda")}
                etiquetaQuitar={t("admin-preguntas-import-cambiar-archivo")}
              />
              {selectedFile ? (
                <p className="text-etiqueta text-success-foreground -mt-2 flex items-center gap-1.5">
                  <CheckCircleIcon aria-hidden weight="fill" className="size-3.5" />
                  {t("admin-preguntas-import-listo")}
                </p>
              ) : null}

              <div className="flex flex-wrap justify-end gap-2">
                <button type="button" className={BOTON_SECUNDARIO} onClick={() => router.push("/admin/preguntas")}>
                  {t("comun-cancelar")}
                </button>
                <BotonAsync
                  pendiente={previewing}
                  textoPendiente={t("comun-procesando")}
                  icon={<EyeIcon className="size-4" />}
                  className={BOTON_PRIMARIO}
                  onClick={handlePreview}
                  disabled={!selectedFile || !procesoID || !asignaturaID}
                >
                  {t("admin-preguntas-import-vista-previa")}
                </BotonAsync>
              </div>
            </div>
          </section>

          {/* Right: info panel */}
          <aside className={TARJETA}>
            <CabeceraSeccion
              icon={TableIcon}
              titulo={t("admin-preguntas-import-columnas-titulo")}
              subtitulo={t("admin-preguntas-import-columnas-subtitulo")}
            />
            <dl className="grid gap-1.5 px-4 pt-1 pb-3">
              {COLUMN_HINTS.map((col) => (
                <div key={col.label} className="flex items-baseline justify-between gap-3">
                  <dt className="bg-secondary text-etiqueta text-foreground shrink-0 rounded-md px-1.5 py-0.5 font-mono">
                    {col.label}
                  </dt>
                  <dd className="text-etiqueta text-muted-foreground text-right">
                    <DescripcionColumna texto={t(col.desc)} />
                  </dd>
                </div>
              ))}
            </dl>
            <Separator className="bg-border/60 mx-4 w-auto" />
            <ul className="text-etiqueta text-muted-foreground grid gap-2.5 px-4 pt-3 pb-4 leading-relaxed">
              <li className="flex items-start gap-2">
                <CheckCircleIcon aria-hidden className="text-success-foreground mt-0.5 size-4 shrink-0" />
                {t("admin-preguntas-import-regla-respuestas")}
              </li>
              <li className="flex items-start gap-2">
                <XCircleIcon aria-hidden className="text-destructive-texto mt-0.5 size-4 shrink-0" />
                {t("admin-preguntas-import-regla-errores")}
              </li>
              <li className="flex items-start gap-2">
                <ListChecksIcon aria-hidden className="text-acento-2-texto dark:text-acento-2 mt-0.5 size-4 shrink-0" />
                {t("admin-preguntas-import-regla-limite")}
              </li>
            </ul>
          </aside>
        </div>
      )}

      {/* ══════════ STEP 1 — Vista previa ══════════ */}
      {step === 1 && preview && previewStats && (
        <>
          <FilaMetricas columnas={4}>
            <Metrica
              etiqueta={t("admin-preguntas-import-metrica-validas")}
              icono={CheckCircleIcon}
              tono="ok"
              ayuda={t("admin-preguntas-import-metrica-validas-ayuda")}
            >
              {preview.total_valido}
            </Metrica>
            <Metrica
              etiqueta={t("admin-preguntas-import-con-error")}
              icono={XCircleIcon}
              tono={preview.total_error > 0 ? "error" : "neutro"}
              ayuda={
                preview.total_error > 0
                  ? t("admin-preguntas-import-metrica-error-ayuda")
                  : t("admin-preguntas-import-metrica-sin-errores")
              }
            >
              {preview.total_error}
            </Metrica>
            <Metrica
              etiqueta={t("admin-preguntas-import-metrica-retro")}
              icono={ListChecksIcon}
              tono="acento"
              sufijo={`/ ${preview.rows.length}`}
              visual={{
                tipo: "barra",
                valor: preview.rows.length ? (previewStats.conRetro / preview.rows.length) * 100 : 0,
              }}
              ayuda={t("admin-preguntas-import-metrica-retro-ayuda")}
            >
              {previewStats.conRetro}
            </Metrica>
            <Metrica
              etiqueta={t("admin-preguntas-import-metrica-archivo")}
              icono={FileTextIcon}
              tono="neutro"
              texto
              ayuda={[procesoNombre, asignaturaNombre].filter(Boolean).join(" · ") || "—"}
            >
              <span className="block max-w-full truncate" title={selectedFile?.name}>
                {selectedFile?.name}
              </span>
            </Metrica>
          </FilaMetricas>

          <div className={cn(TARJETA, "text-control flex flex-wrap items-center gap-x-5 gap-y-2 px-4 py-3")}>
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="text-etiqueta text-texto-terciario">{t("admin-preguntas-import-resumen-tipo")}</span>
              {previewStats.unica > 0 && (
                <Chip>{t("admin-preguntas-import-chip-unica", { n: previewStats.unica })}</Chip>
              )}
              {previewStats.multiple > 0 && (
                <Chip>{t("admin-preguntas-import-chip-multiple", { n: previewStats.multiple })}</Chip>
              )}
            </div>
            <Separator orientacion="vertical" className="hidden h-4 sm:block" />
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="text-etiqueta text-texto-terciario">
                {t("admin-preguntas-import-resumen-dificultad")}
              </span>
              {previewStats.facil > 0 && (
                <PillEstado tono="ok">{t("admin-preguntas-import-chip-facil", { n: previewStats.facil })}</PillEstado>
              )}
              {previewStats.medio > 0 && (
                <PillEstado tono="aviso">
                  {t("admin-preguntas-import-chip-medio", { n: previewStats.medio })}
                </PillEstado>
              )}
              {previewStats.dificil > 0 && (
                <PillEstado tono="error">
                  {t("admin-preguntas-import-chip-dificil", { n: previewStats.dificil })}
                </PillEstado>
              )}
            </div>
          </div>

          {preview.total_error > 0 && (
            <Alerta
              tono="aviso"
              accion={
                <button
                  type="button"
                  className={ACCION_SECUNDARIA}
                  onClick={() => {
                    setShowErrorsOnly((v) => !v);
                    setPreviewPage(0);
                  }}
                >
                  {showErrorsOnly
                    ? t("admin-preguntas-import-ver-todas")
                    : t("admin-preguntas-import-ver-solo-errores", { n: preview.total_error })}
                </button>
              }
            >
              {t.rich("admin-preguntas-import-aviso-errores", {
                n: preview.total_valido,
                b: (c) => <strong>{c}</strong>,
              })}
            </Alerta>
          )}

          {/* ── Preview list ── */}
          <div className={CONTENEDOR_TABLA}>
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className={cn(CABECERA_TABLA, "w-16 pl-4")}>#</TableHead>
                  <TableHead className={CABECERA_TABLA}>{t("admin-preguntas-enunciado")}</TableHead>
                  <TableHead className={cn(CABECERA_TABLA, "w-28")}>{t("admin-preguntas-tipo")}</TableHead>
                  <TableHead className={cn(CABECERA_TABLA, "w-28")}>{t("admin-preguntas-dificultad")}</TableHead>
                  <TableHead className={cn(CABECERA_TABLA, "w-20 text-right")}>
                    {t("admin-preguntas-import-col-resp-corta")}
                  </TableHead>
                  <TableHead className={cn(CABECERA_TABLA, "w-16 pr-4 text-center")}>
                    {t("admin-preguntas-import-col-retro-corta")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="text-control">
                {pagedRows.map((row) => {
                  const isError = row.estado === "error";
                  const celda = cn(
                    "px-3 py-2 align-top",
                    isError && "bg-destructive-suave/60 group-hover/fila:bg-destructive-suave",
                  );
                  return (
                    <TableRow key={row.row}>
                      <TableCell className={cn(celda, "pl-4")}>
                        <span className="text-texto-terciario flex items-center gap-1.5 tabular-nums">
                          {isError ? (
                            <XCircleIcon
                              aria-label={t("admin-preguntas-import-con-error")}
                              weight="fill"
                              className="text-destructive-texto size-3.5"
                            />
                          ) : (
                            <CheckCircleIcon
                              aria-label={t("admin-preguntas-import-fila-valida")}
                              weight="fill"
                              className="text-success-foreground size-3.5"
                            />
                          )}
                          {row.row}
                        </span>
                      </TableCell>
                      <TableCell className={cn(celda, "w-full max-w-0 whitespace-normal")}>
                        <p className="text-foreground leading-relaxed">{row.enunciado}</p>
                        {isError && row.error && (
                          <p className="text-etiqueta text-destructive-texto mt-0.5 flex items-start gap-1">
                            <WarningCircleIcon aria-hidden className="mt-px size-3.5 shrink-0" />
                            {row.error}
                          </p>
                        )}
                      </TableCell>
                      <TableCell className={celda}>
                        <Chip>
                          {row.tipo === "unica"
                            ? t("admin-preguntas-import-tipo-unica-corto")
                            : t("admin-preguntas-import-tipo-multiple-corto")}
                        </Chip>
                      </TableCell>
                      <TableCell className={celda}>
                        {row.dificultad ? (
                          DIFICULTAD_TONO[row.dificultad] ? (
                            <PillEstado tono={DIFICULTAD_TONO[row.dificultad]}>
                              {t(`admin-preguntas-dificultad-${row.dificultad}`)}
                            </PillEstado>
                          ) : (
                            <Chip>{row.dificultad}</Chip>
                          )
                        ) : (
                          <span className="text-texto-terciario">—</span>
                        )}
                      </TableCell>
                      <TableCell className={cn(celda, "text-right tabular-nums")}>
                        <span
                          className={cn(
                            "font-medium",
                            row.num_respuestas >= 2 ? "text-foreground" : "text-destructive-texto",
                          )}
                        >
                          {row.num_respuestas}
                        </span>
                      </TableCell>
                      <TableCell className={cn(celda, "pr-4 text-center")}>
                        {row.has_retro ? (
                          <CheckCircleIcon
                            aria-label={t("admin-preguntas-import-si")}
                            weight="fill"
                            className="text-success-foreground inline size-4"
                          />
                        ) : (
                          <span className="text-texto-terciario">—</span>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })}
                {pagedRows.length === 0 ? (
                  <FilaVacia columnas={6} icon={<FileTextIcon className="size-6" />}>
                    {t("admin-preguntas-import-vacio")}
                  </FilaVacia>
                ) : null}
              </TableBody>
            </Table>
            <PieTabla
              desde={pagedRows.length === 0 ? 0 : paginaActual * previewPageSize + 1}
              hasta={paginaActual * previewPageSize + pagedRows.length}
              total={filteredRows.length}
              unidad={t("admin-preguntas-import-unidad-filas")}
              porPagina={previewPageSize}
              onPorPagina={(n) => {
                setPreviewPageSize(n);
                setPreviewPage(0);
              }}
              pagina={paginaActual + 1}
              ultimaPagina={ultimaPagina}
              onPagina={(p) => setPreviewPage(p - 1)}
            />
          </div>

          <div className="flex flex-wrap gap-2">
            <BotonAsync
              pendiente={importing}
              textoPendiente={t("admin-preguntas-import-importando")}
              icon={<UploadSimpleIcon className="size-4" />}
              className={BOTON_PRIMARIO}
              onClick={handleImport}
              disabled={preview.total_valido === 0}
            >
              {t("admin-preguntas-import-confirmar", { n: preview.total_valido })}
            </BotonAsync>
            <button type="button" className={BOTON_SECUNDARIO} onClick={handleReset} disabled={importing}>
              <ArrowCounterClockwiseIcon weight="bold" className="size-4" />
              {t("admin-preguntas-import-cambiar-archivo")}
            </button>
          </div>
        </>
      )}

      {/* ══════════ STEP 2 — Resultado ══════════ */}
      {step === 2 && result && (
        <>
          <section className={cn(TARJETA, "flex flex-col items-center gap-3 px-6 py-10 text-center")}>
            {result.imported > 0 ? (
              <CheckCircleIcon aria-hidden weight="duotone" className="text-success-foreground size-12" />
            ) : (
              <WarningCircleIcon aria-hidden weight="duotone" className="text-warning-foreground size-12" />
            )}
            <p className="text-titulo-pagina text-foreground">
              {result.imported > 0
                ? t("serumsIa.avisosPregunta.importadasBorrador", { n: result.imported })
                : t("admin-preguntas-import-sin-registros")}
            </p>
            <div className="flex flex-wrap justify-center gap-2">
              {result.imported > 0 && (
                <PillEstado tono="ok">{t("admin-preguntas-import-pill-importadas", { n: result.imported })}</PillEstado>
              )}
              {result.errors.length > 0 && (
                <PillEstado tono="error">
                  {t("admin-preguntas-import-pill-errores", { n: result.errors.length })}
                </PillEstado>
              )}
            </div>
            <div className="mt-2 flex flex-wrap justify-center gap-2">
              <button type="button" className={BOTON_PRIMARIO} onClick={() => router.push("/admin/preguntas")}>
                {t("admin-preguntas-import-ver-preguntas")}
              </button>
              <button type="button" className={BOTON_SECUNDARIO} onClick={handleReset}>
                {t("admin-preguntas-import-otro-archivo")}
              </button>
            </div>
          </section>

          {/* Error list */}
          {result.errors.length > 0 && (
            <section className={TARJETA}>
              <CabeceraSeccion
                icon={WarningCircleIcon}
                titulo={t("admin-preguntas-import-filas-error", { n: result.errors.length })}
              />
              <ul className="grid gap-1 px-4 pt-1 pb-4">
                {result.errors.map((err, idx) => (
                  <li
                    key={idx}
                    className="bg-destructive-suave text-control text-destructive-texto flex gap-2 rounded-lg px-3 py-2"
                  >
                    <span className="shrink-0 font-semibold tabular-nums">
                      {t("admin-preguntas-import-fila", { n: err.row })}
                    </span>
                    <span>{err.message}</span>
                  </li>
                ))}
              </ul>
            </section>
          )}
        </>
      )}
    </div>
  );
}
