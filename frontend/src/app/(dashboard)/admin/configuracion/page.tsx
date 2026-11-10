"use client";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";

import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { Skeleton } from "@/components/feedback/skeleton";
import { useToast } from "@/components/feedback/toast";
import { Campo, Entrada } from "@/components/formularios/campo";
import { Interruptor } from "@/components/formularios/casilla";
import { Deslizador } from "@/components/formularios/deslizador";
import { Tabs } from "@/components/navegacion/tabs";
import { BotonAsync } from "@/components/patrones/boton-async";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { Button } from "@/components/ui/button";
import { adminConfigApi, type Recurso } from "@/lib/auth/auth-api";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_SECUNDARIA, BOTON_PRIMARIO, CONTENEDOR_PAGINA, TARJETA } from "@/lib/estilos";
import { cn } from "@/lib/utils";
import { ArrowDownIcon, ArrowUpIcon, DevicesIcon, LinkIcon, PlusIcon, TrashIcon } from "@phosphor-icons/react";

type Seccion = "sesiones" | "recursos";

const MIN_SESIONES = 1;
const MAX_SESIONES = 10;
const MARCAS_SESIONES = [1, 2, 5, 10];

/** Valor por defecto del campo `icono` de un acceso directo (nombre que guarda el backend). */
const ICONO_RECURSO_DEFECTO = "NiLink";

function CargandoSeccion({ filas = 2 }: { filas?: number }) {
  return (
    <div className="grid gap-3 px-4 pt-2 pb-4" aria-busy="true">
      {Array.from({ length: filas }).map((_, i) => (
        <Skeleton key={i} className="h-control w-full rounded-lg" />
      ))}
    </div>
  );
}

export default function ConfiguracionAdminPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const toast = useToast();

  const [seccion, setSeccion] = useState<Seccion>("sesiones");

  const [maxSesiones, setMaxSesiones] = useState<number>(2);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [recursos, setRecursos] = useState<Recurso[]>([]);
  const [loadingRecursos, setLoadingRecursos] = useState(true);
  const [savingRecursos, setSavingRecursos] = useState(false);
  const [recursosErr, setRecursosErr] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    adminConfigApi
      .getConfigSesiones(token)
      .then((res) => setMaxSesiones(res.max_sesiones))
      .catch(() => setError(t("admin-config-error-cargar")))
      .finally(() => setLoading(false));
    adminConfigApi
      .getConfigRecursos(token)
      .then((res) => setRecursos(res))
      .catch(() => setRecursosErr(t("admin-config-recursos-error-cargar")))
      .finally(() => setLoadingRecursos(false));
  }, [token, t]);

  const handleSave = async () => {
    if (!token) return;
    setSaving(true);
    setError(null);
    try {
      await adminConfigApi.setConfigSesiones(token, maxSesiones);
      toast.ok(t("admin-config-guardado"));
    } catch {
      toast.error(t("admin-config-error-guardar"));
    } finally {
      setSaving(false);
    }
  };

  const updateRecurso = (index: number, patch: Partial<Recurso>) => {
    setRecursos((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch } : r)));
  };

  const addRecurso = () => {
    setRecursos((prev) => [
      ...prev,
      {
        id: crypto.randomUUID(),
        titulo: "",
        descripcion: "",
        url: "",
        icono: ICONO_RECURSO_DEFECTO,
        orden: prev.length + 1,
        activo: true,
      },
    ]);
  };

  const removeRecurso = (index: number) => {
    setRecursos((prev) => prev.filter((_, i) => i !== index));
  };

  const moveRecurso = (index: number, dir: -1 | 1) => {
    setRecursos((prev) => {
      const next = [...prev];
      const target = index + dir;
      if (target < 0 || target >= next.length) return prev;
      [next[index], next[target]] = [next[target], next[index]];
      return next.map((r, i) => ({ ...r, orden: i + 1 }));
    });
  };

  const handleSaveRecursos = async () => {
    if (!token) return;
    setSavingRecursos(true);
    setRecursosErr(null);
    try {
      const ordered = recursos.map((r, i) => ({ ...r, orden: i + 1 }));
      const saved = await adminConfigApi.setConfigRecursos(token, ordered);
      setRecursos(saved);
      toast.ok(t("admin-config-recursos-guardado"));
    } catch {
      toast.error(t("admin-config-recursos-error-guardar"));
    } finally {
      setSavingRecursos(false);
    }
  };

  const nombreItem = (r: Recurso) => r.titulo || t("admin-config-recursos-item");

  const panelSesiones = (
    <section className={cn(TARJETA, "max-w-2xl")}>
      <CabeceraSeccion icon={DevicesIcon} titulo={t("admin-config-sesiones-titulo")} />
      <p className="text-control text-muted-foreground px-4 leading-relaxed">{t("admin-config-sesiones-desc")}</p>

      {loading ? (
        <CargandoSeccion />
      ) : (
        <div className="grid gap-4 px-4 pt-4 pb-4">
          {error ? <Alerta tono="error">{error}</Alerta> : null}

          <div className="bg-secondary rounded-xl px-4 pt-3 pb-4">
            <p className="text-etiqueta text-muted-foreground font-medium">{t("admin-config-sesiones-max")}</p>
            <Deslizador
              etiqueta={t("admin-config-sesiones-max")}
              valor={maxSesiones}
              onCambio={(v) => setMaxSesiones(v as number)}
              min={MIN_SESIONES}
              max={MAX_SESIONES}
              paso={1}
              className="mt-0.5"
            />
            <div className="text-etiqueta text-texto-terciario relative h-4 tabular-nums" aria-hidden>
              {MARCAS_SESIONES.map((m) => (
                <span
                  key={m}
                  className="absolute -translate-x-1/2"
                  style={{ left: `${((m - MIN_SESIONES) / (MAX_SESIONES - MIN_SESIONES)) * 100}%` }}
                >
                  {m}
                </span>
              ))}
            </div>
          </div>

          <p className="text-etiqueta text-muted-foreground">
            {maxSesiones === 1
              ? t("admin-config-sesiones-caption-uno")
              : t("admin-config-sesiones-caption-varias", { n: maxSesiones })}
          </p>

          <div className="flex justify-end">
            <BotonAsync
              pendiente={saving}
              textoPendiente={t("comun-guardando")}
              onClick={handleSave}
              className={BOTON_PRIMARIO}
            >
              {t("comun-guardar-cambios")}
            </BotonAsync>
          </div>
        </div>
      )}
    </section>
  );

  const panelRecursos = (
    <section className={TARJETA}>
      <CabeceraSeccion
        icon={LinkIcon}
        titulo={t("admin-config-recursos-titulo")}
        subtitulo={t("admin-config-recursos-desc")}
        derecha={
          <button type="button" onClick={addRecurso} className={ACCION_SECUNDARIA} disabled={loadingRecursos}>
            <PlusIcon weight="bold" className="size-4" />
            {t("comun-agregar")}
          </button>
        }
      />

      {loadingRecursos ? (
        <CargandoSeccion filas={3} />
      ) : (
        <div className="grid gap-3 px-4 pt-2 pb-4">
          {recursosErr ? <Alerta tono="error">{recursosErr}</Alerta> : null}

          {recursos.length === 0 ? (
            <EstadoVacio
              icon={LinkIcon}
              titulo={t("admin-config-recursos-vacio")}
              accion={
                <button type="button" onClick={addRecurso} className={ACCION_SECUNDARIA}>
                  <PlusIcon weight="bold" className="size-4" />
                  {t("comun-agregar")}
                </button>
              }
            />
          ) : (
            <ol className="grid gap-2">
              {recursos.map((r, index) => {
                const base = `recurso-${r.id}`;
                const item = nombreItem(r);
                return (
                  <li
                    key={r.id}
                    className={cn(
                      "bg-secondary grid gap-3 rounded-xl p-3 transition-opacity md:grid-cols-[2rem_minmax(0,1.3fr)_minmax(0,1fr)_minmax(0,1.3fr)_minmax(0,0.8fr)_auto] md:items-end",
                      !r.activo && "opacity-70",
                    )}
                  >
                    <span
                      aria-hidden
                      className="size-control bg-card text-etiqueta text-muted-foreground hidden place-items-center rounded-full font-mono font-semibold md:grid"
                    >
                      {index + 1}
                    </span>
                    <Campo id={`${base}-titulo`} etiqueta={t("admin-config-recursos-campo-titulo")}>
                      <Entrada
                        id={`${base}-titulo`}
                        value={r.titulo}
                        onChange={(e) => updateRecurso(index, { titulo: e.target.value })}
                      />
                    </Campo>
                    <Campo id={`${base}-descripcion`} etiqueta={t("admin-config-recursos-campo-descripcion")}>
                      <Entrada
                        id={`${base}-descripcion`}
                        value={r.descripcion}
                        onChange={(e) => updateRecurso(index, { descripcion: e.target.value })}
                      />
                    </Campo>
                    <Campo id={`${base}-url`} etiqueta={t("admin-config-recursos-campo-url")}>
                      <Entrada
                        id={`${base}-url`}
                        type="url"
                        inputMode="url"
                        placeholder="https://..."
                        value={r.url}
                        onChange={(e) => updateRecurso(index, { url: e.target.value })}
                      />
                    </Campo>
                    <Campo id={`${base}-icono`} etiqueta={t("admin-config-recursos-campo-icono")}>
                      <Entrada
                        id={`${base}-icono`}
                        mono
                        placeholder={ICONO_RECURSO_DEFECTO}
                        value={r.icono}
                        onChange={(e) => updateRecurso(index, { icono: e.target.value })}
                      />
                    </Campo>
                    <div className="flex items-center justify-end gap-1">
                      <Interruptor
                        id={`${base}-activo`}
                        etiqueta={<span className="sr-only">{t("admin-config-recursos-activar", { item })}</span>}
                        checked={r.activo}
                        onCheckedChange={(v) => updateRecurso(index, { activo: v })}
                        className="mr-1 gap-0"
                      />
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        aria-label={t("admin-config-recursos-subir", { item })}
                        title={t("admin-config-recursos-subir", { item })}
                        onClick={() => moveRecurso(index, -1)}
                        disabled={index === 0}
                      >
                        <ArrowUpIcon weight="bold" />
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        aria-label={t("admin-config-recursos-bajar", { item })}
                        title={t("admin-config-recursos-bajar", { item })}
                        onClick={() => moveRecurso(index, 1)}
                        disabled={index === recursos.length - 1}
                      >
                        <ArrowDownIcon weight="bold" />
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon"
                        aria-label={t("admin-config-recursos-eliminar", { item })}
                        title={t("admin-config-recursos-eliminar", { item })}
                        onClick={() => removeRecurso(index)}
                        className="text-destructive-texto hover:bg-destructive-suave hover:text-destructive-texto"
                      >
                        <TrashIcon />
                      </Button>
                    </div>
                  </li>
                );
              })}
            </ol>
          )}

          <div className="flex justify-end">
            <BotonAsync
              pendiente={savingRecursos}
              textoPendiente={t("comun-guardando")}
              onClick={handleSaveRecursos}
              className={BOTON_PRIMARIO}
            >
              {t("admin-config-recursos-guardar")}
            </BotonAsync>
          </div>
        </div>
      )}
    </section>
  );

  return (
    <section className={CONTENEDOR_PAGINA} aria-label={t("admin-config-titulo")}>
      <p className="text-cuerpo text-muted-foreground">{t("admin-config-subtitulo")}</p>
      <Tabs<Seccion>
        valor={seccion}
        onCambio={setSeccion}
        items={[
          { id: "sesiones", etiqueta: t("admin-config-sesiones-titulo") },
          {
            id: "recursos",
            etiqueta: t("admin-config-recursos-titulo"),
            contador: loadingRecursos ? undefined : recursos.length,
          },
        ]}
        paneles={{ sesiones: panelSesiones, recursos: panelRecursos }}
      />
    </section>
  );
}
