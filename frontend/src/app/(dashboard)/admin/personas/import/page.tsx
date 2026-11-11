"use client";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useMemo, useState } from "react";

import { TablaDatos } from "@/components/app/tabla-datos";
import { type Columna } from "@/components/datos/tabla-datos";
import { Alerta } from "@/components/feedback/alerta";
import { ZonaArchivos } from "@/components/formularios/zona-archivos";
import { Pasos } from "@/components/navegacion/pasos";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { PersonaImportResult, PersonaPreviewResponse, PersonaPreviewRow, personasApi } from "@/lib/auth/auth-api";
import { useAuth } from "@/lib/auth/auth-context";
import {
  ACCION_PRINCIPAL,
  ACCION_SECUNDARIA,
  BOTON_PRIMARIO,
  BOTON_SECUNDARIO,
  CONTENEDOR_PAGINA,
  TARJETA,
} from "@/lib/estilos";
import { PESO_ICONO } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import {
  CheckCircleIcon,
  CopyIcon,
  DownloadSimpleIcon,
  FileTextIcon,
  ListChecksIcon,
  LockIcon,
  MagnifyingGlassIcon,
  UploadSimpleIcon,
  UsersIcon,
  WarningCircleIcon,
  XIcon,
} from "@phosphor-icons/react";

const TEMPLATE_HEADERS = [
  "dni",
  "nombres",
  "apellido_paterno",
  "apellido_materno",
  "grado_id",
  "especialidad_id",
  "institucion_id",
  "telefono",
  "email",
];

/** Columnas de la plantilla con la clave i18n de su descripción. */
const COLUMN_HINTS = [
  { label: "dni", desc: "admin-personas-import-col-dni" },
  { label: "nombres", desc: "perfil-nombres" },
  { label: "apellido_paterno", desc: "admin-personas-import-col-apellido-paterno" },
  { label: "apellido_materno", desc: "admin-personas-import-col-apellido-materno" },
  { label: "grado_id", desc: "admin-preguntas-import-col-id" },
  { label: "especialidad_id", desc: "admin-preguntas-import-col-id" },
  { label: "institucion_id", desc: "admin-preguntas-import-col-id" },
  { label: "telefono", desc: "admin-preguntas-import-col-opcional" },
  { label: "email", desc: "admin-personas-import-col-email" },
];

const PREVIEW_PAGE_SIZE = 20;

export default function ImportPersonasPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const steps = [
    t("admin-personas-import-paso-archivo"),
    t("admin-preguntas-import-vista-previa"),
    t("admin-preguntas-import-paso-resultado"),
  ];

  const columnasPreview = useMemo<Columna<PersonaPreviewRow>[]>(
    () => [
      {
        id: "row",
        titulo: "#",
        ordenable: true,
        ancho: "4.5rem",
        render: (r) => (
          <span className="inline-flex items-center gap-1.5">
            {r.estado === "error" ? (
              <WarningCircleIcon
                aria-label={t("admin-preguntas-import-con-error")}
                weight={PESO_ICONO}
                className="text-destructive-texto size-4"
              />
            ) : (
              <CheckCircleIcon
                aria-label={t("admin-preguntas-import-fila-valida")}
                weight={PESO_ICONO}
                className="text-success-foreground size-4"
              />
            )}
            <span className="text-etiqueta text-texto-terciario tabular-nums">{r.row}</span>
          </span>
        ),
      },
      {
        id: "nombre",
        titulo: t("admin-usuarios-col-nombre"),
        ordenable: true,
        valor: (r) => [r.nombres, r.apellido_paterno, r.apellido_materno].filter(Boolean).join(" "),
        render: (r) => {
          const fullName = [r.nombres, r.apellido_paterno, r.apellido_materno].filter(Boolean).join(" ");
          return (
            <div className="min-w-48">
              <p className="text-foreground">{fullName || "—"}</p>
              {r.estado === "error" && r.error ? (
                <p className="text-etiqueta text-destructive-texto mt-0.5">{r.error}</p>
              ) : null}
            </div>
          );
        },
      },
      {
        id: "dni",
        titulo: t("admin-usuarios-dni"),
        className: "font-mono tabular-nums text-muted-foreground",
        render: (r) => r.dni || "—",
      },
      {
        id: "email",
        titulo: t("admin-usuarios-col-email"),
        className: "break-all text-muted-foreground",
        render: (r) => r.email || "—",
      },
      {
        id: "grado_id",
        titulo: t("perfil-grado"),
        render: (r) => <Chip className="font-mono">{r.grado_id || "—"}</Chip>,
      },
      {
        id: "especialidad_id",
        titulo: t("admin-personas-import-col-esp-corta"),
        render: (r) => <Chip className="font-mono">{r.especialidad_id || "—"}</Chip>,
      },
    ],
    [t],
  );

  const [step, setStep] = useState(0);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [previewing, setPreviewing] = useState(false);
  const [importing, setImporting] = useState(false);
  const [preview, setPreview] = useState<PersonaPreviewResponse | null>(null);
  const [result, setResult] = useState<PersonaImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showErrorsOnly, setShowErrorsOnly] = useState(false);

  const acceptFile = (file: File | null) => {
    setSelectedFile(file);
    if (file) setError(null);
  };

  const handlePreview = async () => {
    if (!token || !selectedFile) return;
    setPreviewing(true);
    setError(null);
    try {
      const res = await personasApi.preview(token, selectedFile);
      setPreview(res);
      setStep(1);
    } catch (e: unknown) {
      setError((e as { message?: string }).message || t("admin-preguntas-import-error-procesar"));
    } finally {
      setPreviewing(false);
    }
  };

  const handleImport = async () => {
    if (!token || !selectedFile) return;
    setImporting(true);
    setError(null);
    try {
      const res = await personasApi.import(token, selectedFile);
      setResult(res);
      setStep(2);
    } catch (e: unknown) {
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
    setShowErrorsOnly(false);
  };

  const downloadTemplate = () => {
    const csv = TEMPLATE_HEADERS.join(",") + "\n";
    const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "plantilla_personas.csv";
    a.click();
    URL.revokeObjectURL(url);
  };

  const filteredRows: PersonaPreviewRow[] = preview
    ? showErrorsOnly
      ? preview.rows.filter((r) => r.estado === "error")
      : preview.rows
    : [];

  const totalErrores = result?.errors?.length ?? 0;

  return (
    <div className={CONTENEDOR_PAGINA}>
      {/* ── Cabecera ── */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-cuerpo text-muted-foreground">{t("admin-personas-import-descripcion")}</p>
        <button type="button" onClick={downloadTemplate} className={ACCION_SECUNDARIA}>
          <DownloadSimpleIcon className="size-4" />
          {t("admin-preguntas-import-descargar-plantilla")}
        </button>
      </div>

      {/* ── Pasos ── */}
      <div className={cn(TARJETA, "px-4 py-4")}>
        <Pasos pasos={steps.map((titulo) => ({ titulo }))} actual={step} className="mx-auto max-w-2xl" />
      </div>

      {error ? (
        <Alerta
          tono="error"
          accion={
            <Button variant="ghost" size="icon-sm" aria-label={t("comun-cerrar")} onClick={() => setError(null)}>
              <XIcon weight="bold" className="size-4" />
            </Button>
          }
        >
          {error}
        </Alerta>
      ) : null}

      {/* ══ PASO 0 — Seleccionar archivo ══ */}
      {step === 0 ? (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-12">
          <section className={cn(TARJETA, "flex flex-col gap-4 p-4 md:col-span-7")}>
            <CabeceraSeccion
              icon={UploadSimpleIcon}
              titulo={t("admin-personas-import-paso-archivo")}
              subtitulo={t("admin-personas-import-archivo-ayuda")}
              conBorde={false}
              className="px-0 pt-0 pb-0"
            />
            <ZonaArchivos
              id="archivo-personas"
              accept=".xlsx,.csv"
              archivo={selectedFile}
              onCambio={acceptFile}
              titulo={t("admin-preguntas-import-archivo-titulo")}
              ayuda={t("admin-personas-import-archivo-ayuda")}
              etiquetaQuitar={t("admin-preguntas-import-cambiar-archivo")}
              disabled={previewing}
              className="min-h-44"
            />
            {selectedFile ? (
              <p className="text-etiqueta text-success-foreground flex items-center gap-1.5">
                <CheckCircleIcon aria-hidden weight={PESO_ICONO} className="size-3.5" />
                {t("admin-preguntas-import-listo")}
              </p>
            ) : null}
            <div className="mt-auto flex flex-wrap justify-end gap-2">
              <Link href="/admin/personas" className={BOTON_SECUNDARIO}>
                {t("comun-cancelar")}
              </Link>
              <BotonAsync
                pendiente={previewing}
                textoPendiente={t("comun-procesando")}
                disabled={!selectedFile}
                onClick={handlePreview}
                icon={<MagnifyingGlassIcon weight="bold" className="size-4" />}
                className={BOTON_PRIMARIO}
              >
                {t("admin-preguntas-import-vista-previa")}
              </BotonAsync>
            </div>
          </section>

          <aside className={cn(TARJETA, "flex flex-col gap-4 p-4 md:col-span-5")}>
            <CabeceraSeccion
              icon={ListChecksIcon}
              titulo={t("admin-personas-import-columnas-titulo")}
              subtitulo={t("admin-preguntas-import-columnas-subtitulo")}
              conBorde={false}
              className="px-0 pt-0 pb-0"
            />
            <ul className="flex flex-col gap-1.5">
              {COLUMN_HINTS.map((col) => (
                <li key={col.label} className="flex items-center justify-between gap-2">
                  <code className="bg-secondary text-etiqueta text-foreground rounded-md px-1.5 py-0.5 font-mono">
                    {col.label}
                  </code>
                  <span className="text-etiqueta text-muted-foreground text-right">{t(col.desc)}</span>
                </li>
              ))}
            </ul>
            <Separator />
            <ul className="text-etiqueta text-muted-foreground flex flex-col gap-2.5 leading-relaxed">
              <li className="flex items-start gap-2">
                <LockIcon aria-hidden weight={PESO_ICONO} className="text-acento-2-texto mt-0.5 size-4 shrink-0" />
                {t("admin-personas-import-regla-cifrado")}
              </li>
              <li className="flex items-start gap-2">
                <CopyIcon aria-hidden weight={PESO_ICONO} className="text-success-foreground mt-0.5 size-4 shrink-0" />
                {t("admin-personas-import-regla-duplicados")}
              </li>
              <li className="flex items-start gap-2">
                <UsersIcon aria-hidden weight={PESO_ICONO} className="text-acento-2-texto mt-0.5 size-4 shrink-0" />
                {t("admin-personas-import-regla-limite")}
              </li>
            </ul>
          </aside>
        </div>
      ) : null}

      {/* ══ PASO 1 — Vista previa ══ */}
      {step === 1 && preview ? (
        <>
          <div className={cn(TARJETA, "flex flex-wrap items-center gap-x-5 gap-y-2 px-4 py-3")}>
            <span className="text-control text-foreground flex items-center gap-1.5">
              <CheckCircleIcon aria-hidden weight={PESO_ICONO} className="text-success-foreground size-[18px]" />
              {t.rich("admin-personas-import-validas", {
                n: preview.total_valido,
                b: (c) => <span className="text-titulo tabular-nums">{c}</span>,
              })}
            </span>
            {preview.total_error > 0 ? (
              <span className="text-control text-destructive-texto flex items-center gap-1.5">
                <WarningCircleIcon aria-hidden weight={PESO_ICONO} className="size-[18px]" />
                {t.rich("admin-personas-import-con-error-n", {
                  n: preview.total_error,
                  b: (c) => <span className="text-titulo tabular-nums">{c}</span>,
                })}
              </span>
            ) : null}
            <Separator orientacion="vertical" className="h-5" />
            <span className="text-etiqueta text-texto-terciario">
              {t.rich("admin-personas-import-total-filas", {
                n: preview.rows.length,
                b: (c) => <span className="text-foreground font-semibold tabular-nums">{c}</span>,
              })}
            </span>
            <span className="text-etiqueta text-texto-terciario ml-auto flex min-w-0 items-center gap-1">
              <FileTextIcon aria-hidden className="size-3.5 shrink-0" />
              <span className="truncate font-mono">{selectedFile?.name}</span>
            </span>
          </div>

          {preview.total_error > 0 ? (
            <Alerta
              tono="aviso"
              accion={
                <button type="button" className={ACCION_SECUNDARIA} onClick={() => setShowErrorsOnly((v) => !v)}>
                  {showErrorsOnly
                    ? t("admin-preguntas-import-ver-todas")
                    : t("admin-preguntas-import-ver-solo-errores", { n: preview.total_error })}
                </button>
              }
            >
              {t.rich("admin-personas-import-aviso-errores", {
                n: preview.total_valido,
                b: (c) => <strong>{c}</strong>,
              })}
            </Alerta>
          ) : null}

          <TablaDatos
            key={showErrorsOnly ? "errores" : "todas"}
            columnas={columnasPreview}
            filas={filteredRows}
            clave={(r) => String(r.row)}
            unidad={t("admin-preguntas-import-unidad-filas")}
            porPaginaInicial={PREVIEW_PAGE_SIZE}
            vacio={t("admin-personas-import-vacio")}
          />

          <div className="flex flex-wrap gap-2">
            <BotonAsync
              pendiente={importing}
              textoPendiente={t("admin-preguntas-import-importando")}
              disabled={preview.total_valido === 0}
              onClick={handleImport}
              icon={<UploadSimpleIcon className="size-4" />}
              className={BOTON_PRIMARIO}
            >
              {t("admin-personas-import-confirmar", { n: preview.total_valido })}
            </BotonAsync>
            <button type="button" onClick={handleReset} disabled={importing} className={BOTON_SECUNDARIO}>
              {t("admin-preguntas-import-cambiar-archivo")}
            </button>
          </div>
        </>
      ) : null}

      {/* ══ PASO 2 — Resultado ══ */}
      {step === 2 && result ? (
        <>
          <section className={cn(TARJETA, "flex flex-col items-center gap-4 px-6 py-10 text-center")}>
            <span
              className={cn(
                "grid size-14 place-items-center rounded-full",
                result.inserted > 0 ? "bg-success text-success-foreground" : "bg-warning text-warning-foreground",
              )}
            >
              {result.inserted > 0 ? (
                <UsersIcon aria-hidden weight={PESO_ICONO} className="size-7" />
              ) : (
                <WarningCircleIcon aria-hidden weight={PESO_ICONO} className="size-7" />
              )}
            </span>
            <div>
              <p className="text-titulo-pagina text-foreground">
                {result.inserted > 0
                  ? t("admin-personas-import-resultado-importadas", { n: result.inserted })
                  : t("admin-preguntas-import-sin-registros")}
              </p>
              {result.duplicates > 0 ? (
                <p className="text-cuerpo text-muted-foreground mt-1">
                  {t("admin-personas-import-duplicados-omitidos", { n: result.duplicates })}
                </p>
              ) : null}
            </div>
            <div className="flex flex-wrap justify-center gap-2">
              {result.inserted > 0 ? (
                <PillEstado tono="ok">{t("admin-personas-import-pill-insertadas", { n: result.inserted })}</PillEstado>
              ) : null}
              {result.duplicates > 0 ? (
                <PillEstado tono="aviso">
                  {t("admin-personas-import-pill-duplicados", { n: result.duplicates })}
                </PillEstado>
              ) : null}
              {totalErrores > 0 ? (
                <PillEstado tono="error">{t("admin-preguntas-import-pill-errores", { n: totalErrores })}</PillEstado>
              ) : null}
            </div>
          </section>

          {totalErrores > 0 ? (
            <section className={cn(TARJETA, "flex flex-col gap-2 p-4")}>
              <CabeceraSeccion
                icon={WarningCircleIcon}
                titulo={t("admin-preguntas-import-filas-error", { n: totalErrores })}
                conBorde={false}
                className="[&_svg]:text-destructive-texto px-0 pt-0 pb-0"
              />
              <ul className="flex flex-col gap-1">
                {result.errors!.map((err, idx) => (
                  <li
                    key={idx}
                    className="bg-destructive-suave text-control text-destructive-texto rounded-lg px-3 py-1.5"
                  >
                    {err}
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          <div className="flex flex-wrap gap-2">
            <Link href="/admin/personas" className={ACCION_PRINCIPAL}>
              <UsersIcon className="size-4" />
              {t("admin-personas-import-ver-personas")}
            </Link>
            <button type="button" onClick={handleReset} className={ACCION_SECUNDARIA}>
              {t("admin-preguntas-import-otro-archivo")}
            </button>
          </div>
        </>
      ) : null}
    </div>
  );
}
