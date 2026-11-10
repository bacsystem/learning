"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { type ReactNode, useEffect, useId, useState } from "react";

import { GraficoBarras } from "@/components/datos/grafico";
import { Alerta } from "@/components/feedback/alerta";
import { Skeleton } from "@/components/feedback/skeleton";
import { CabeceraSeccion } from "@/components/patrones/cabecera-seccion";
import { Metrica, type TonoMetrica } from "@/components/patrones/metricas";
import { NumeroAnimado } from "@/components/patrones/numero-animado";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  adminGetDashboard,
  adminGetDashboardActividad,
  type DashboardActividad,
  type DashboardData,
} from "@/lib/admin/api";
import { systemApi, type UsuarioOnline } from "@/lib/auth/auth-api";
import { useAuth } from "@/lib/auth/auth-context";
import { ACCION_SECUNDARIA, CABECERA_TABLA, CONTENEDOR_PAGINA, FOCO, TARJETA } from "@/lib/estilos";
import { formatearFecha } from "@/lib/formato";
import { type IconoComponente, PESO_ICONO, PESO_ICONO_FUNCIONAL } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import {
  ArrowRightIcon,
  BookOpenIcon,
  ChartBarIcon,
  ChartLineIcon,
  CoinsIcon,
  HourglassIcon,
  ListNumbersIcon,
  TrendUpIcon,
  UserCheckIcon,
  UsersIcon,
  WifiHighIcon,
} from "@phosphor-icons/react";

/* ── Piezas de la página ─────────────────────────────────────── */

function Tarjeta({ children, className }: { children: ReactNode; className?: string }) {
  return <section className={cn(TARJETA, "flex min-w-0 flex-col", className)}>{children}</section>;
}

function Vacio({ icon: Icono, children }: { icon: IconoComponente; children: ReactNode }) {
  return (
    <div className="flex min-h-40 flex-col items-center justify-center gap-2 py-6 text-center">
      <Icono aria-hidden weight={PESO_ICONO} className="text-texto-terciario size-6" />
      <p className="text-control text-muted-foreground">{children}</p>
    </div>
  );
}

function StatCard({
  icon,
  label,
  value,
  sub,
  href,
  loading,
  tono,
}: {
  icon: IconoComponente;
  label: string;
  value: number;
  sub?: string;
  href?: string;
  loading: boolean;
  tono: TonoMetrica;
}) {
  const t = useTranslations("dashboard");
  const inner = (
    <Metrica
      etiqueta={label}
      icono={icon}
      tono={tono}
      ayuda={sub ?? (href ? t("admin-dashboard-ver-detalle") : t("admin-dashboard-hoy"))}
    >
      {loading ? <Skeleton className="my-1 h-5 w-14" /> : <NumeroAnimado valor={value} />}
    </Metrica>
  );

  return href ? (
    <Link href={href} className={cn("block min-w-0 rounded-xl", FOCO)}>
      {inner}
    </Link>
  ) : (
    inner
  );
}

function OnlineUserRow({ u }: { u: UsuarioOnline }) {
  const t = useTranslations("dashboard");
  const seenAgo = Math.round((Date.now() - new Date(u.last_seen_at).getTime()) / 1000);
  const seenLabel =
    seenAgo < 60
      ? t("admin-dashboard-hace-segundos", { n: seenAgo })
      : t("admin-dashboard-hace-minutos", { n: Math.round(seenAgo / 60) });

  return (
    <li className="bg-secondary flex items-center gap-3 rounded-lg px-3 py-2">
      <span className="relative shrink-0">
        <Avatar nombre={u.nombre_completo} tamano="lg" tono="accent" />
        <span
          aria-hidden
          className="bg-success-solid ring-secondary absolute right-0 bottom-0 size-2.5 rounded-full ring-2"
        />
      </span>
      <div className="min-w-0 flex-1">
        <p className="text-control text-foreground truncate font-semibold">{u.nombre_completo}</p>
        <p className="text-etiqueta text-texto-terciario truncate font-mono">@{u.username}</p>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <PillEstado tono={u.rol === "admin" ? "aviso" : "neutro"}>{u.rol}</PillEstado>
        <span className="text-etiqueta text-texto-terciario hidden tabular-nums sm:inline">{seenLabel}</span>
      </div>
    </li>
  );
}

/* ── Página ──────────────────────────────────────────────────── */

export default function AdminDashboardPage() {
  const { token } = useAuth();
  const t = useTranslations("dashboard");
  const idActividad = useId();

  const [stats, setStats] = useState<DashboardData | null>(null);
  const [actividad, setActividad] = useState<DashboardActividad | null>(null);
  const [online, setOnline] = useState<UsuarioOnline[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadingActividad, setLoadingActividad] = useState(true);
  const [loadingOnline, setLoadingOnline] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    adminGetDashboard(token)
      .then(setStats)
      .catch(() => setError(t("admin-dashboard-error-cargar")))
      .finally(() => setLoading(false));

    adminGetDashboardActividad(token)
      .then(setActividad)
      .catch(() => {})
      .finally(() => setLoadingActividad(false));
  }, [token, t]);

  useEffect(() => {
    if (!token) return;
    const load = () => {
      systemApi
        .listUsuariosOnline(token)
        .then((r) => setOnline(r.data))
        .catch(() => {})
        .finally(() => setLoadingOnline(false));
    };
    load();
    const interval = setInterval(load, 30_000);
    return () => clearInterval(interval);
  }, [token]);

  const practicasData = actividad?.practicas_ultima_semana ?? [];
  const simulacrosData = actividad?.simulacros_ultima_semana ?? [];

  // Merge both series onto same date axis
  const allDates = Array.from(
    new Set([...practicasData.map((d) => d.fecha), ...simulacrosData.map((d) => d.fecha)]),
  ).sort();

  const practicasValues = allDates.map((d) => practicasData.find((x) => x.fecha === d)?.total ?? 0);
  const simulacrosValues = allDates.map((d) => simulacrosData.find((x) => x.fecha === d)?.total ?? 0);
  // The series are zero-filled (H142): an all-zero week is still "no data".
  const sinActividad = [...practicasValues, ...simulacrosValues].every((v) => v === 0);

  // "2026-03-05" → "5 Mar" (formatearFecha sin el año).
  const dateLabels = allDates.map((d) => formatearFecha(d).replace(/ \d{4}$/, ""));

  const topAsignaturas = actividad?.top_asignaturas ?? [];
  const totalPracticasSemana = practicasValues.reduce((s, v) => s + v, 0);
  const totalSimulacrosSemana = simulacrosValues.reduce((s, v) => s + v, 0);

  return (
    <div className={CONTENEDOR_PAGINA}>
      {error && (
        <Alerta
          tono="error"
          accion={
            <Button variant="ghost" size="sm" onClick={() => setError(null)}>
              {t("comun-cerrar")}
            </Button>
          }
        >
          {error}
        </Alerta>
      )}

      <p className="text-cuerpo text-muted-foreground">{t("admin-dashboard-subtitulo")}</p>

      {/* Métricas — 6 tarjetas */}
      <section className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-6">
        <StatCard
          icon={UsersIcon}
          label={t("admin-dashboard-usuarios-registrados")}
          value={stats?.total_usuarios_registrados ?? 0}
          href="/admin/usuarios"
          loading={loading}
          tono="acento"
        />
        <StatCard
          icon={UserCheckIcon}
          label={t("admin-dashboard-usuarios-activos")}
          value={stats?.total_usuarios_activos ?? 0}
          href="/admin/usuarios"
          loading={loading}
          tono="ok"
        />
        <StatCard
          icon={HourglassIcon}
          label={t("admin-dashboard-pendientes")}
          value={stats?.total_usuarios_pendientes ?? 0}
          href="/admin/usuarios"
          loading={loading}
          tono="aviso"
        />
        <StatCard
          icon={BookOpenIcon}
          label={t("admin-dashboard-practicas-hoy")}
          value={stats?.total_practicas_hoy ?? 0}
          loading={loading}
          tono="neutro"
        />
        <StatCard
          icon={ChartLineIcon}
          label={t("admin-dashboard-simulacros-hoy")}
          value={stats?.total_simulacros_hoy ?? 0}
          loading={loading}
          tono="neutro"
        />
        <StatCard
          icon={TrendUpIcon}
          label={t("admin-dashboard-en-linea")}
          value={stats?.usuarios_online ?? 0}
          sub={t("admin-dashboard-sesiones-activas")}
          loading={loading}
          tono="ok"
        />
      </section>

      <div className="grid grid-cols-1 items-stretch gap-3 lg:grid-cols-3">
        {/* Actividad — últimos 7 días */}
        <Tarjeta className="lg:col-span-2">
          <CabeceraSeccion
            icon={ChartBarIcon}
            titulo={<span id={idActividad}>{t("admin-dashboard-actividad-titulo")}</span>}
            subtitulo={
              !loadingActividad && allDates.length > 0 && !sinActividad
                ? t("admin-dashboard-actividad-resumen", {
                    practicas: totalPracticasSemana,
                    simulacros: totalSimulacrosSemana,
                  })
                : undefined
            }
          />
          <div className="min-w-0 flex-1 px-4 pt-1 pb-4">
            {loadingActividad ? (
              <div aria-busy="true" className="bg-secondary h-[280px] w-full animate-pulse rounded-lg" />
            ) : allDates.length === 0 || sinActividad ? (
              <Vacio icon={ChartBarIcon}>{t("admin-dashboard-actividad-vacio")}</Vacio>
            ) : (
              <GraficoBarras
                plano
                etiquetadoPor={idActividad}
                alto={280}
                categorias={dateLabels}
                series={[
                  { nombre: t("menu-practicas"), valores: practicasValues },
                  { nombre: t("menu-simulacros"), valores: simulacrosValues },
                ]}
                formato={(v) => v.toLocaleString("es-PE")}
                descripcion={t("admin-dashboard-actividad-desc", {
                  practicas: totalPracticasSemana,
                  simulacros: totalSimulacrosSemana,
                })}
              />
            )}
          </div>
        </Tarjeta>

        {/* Top asignaturas */}
        <Tarjeta>
          <CabeceraSeccion icon={ListNumbersIcon} titulo={t("admin-dashboard-top-asignaturas")} />
          <div className="min-w-0 flex-1 px-2 pb-2">
            {loadingActividad ? (
              <div aria-busy="true" className="flex flex-col gap-[3px] px-2 pt-2">
                {[1, 2, 3, 4, 5].map((i) => (
                  <Skeleton key={i} className="h-9 rounded-xl" />
                ))}
              </div>
            ) : topAsignaturas.length === 0 ? (
              <Vacio icon={ListNumbersIcon}>{t("admin-dashboard-sin-datos")}</Vacio>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className={cn(CABECERA_TABLA, "pl-4")}>{t("admin-catalogos-col-asignatura")}</TableHead>
                    <TableHead className={cn(CABECERA_TABLA, "pr-4 text-right")}>{t("menu-practicas")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody className="text-control">
                  {topAsignaturas.map((a, i) => (
                    <TableRow key={a.nombre}>
                      <TableCell className="w-full max-w-0 pl-4">
                        <span className="flex min-w-0 items-center gap-2">
                          <span className="text-etiqueta text-texto-terciario w-4 shrink-0 tabular-nums">{i + 1}</span>
                          <span className="text-foreground truncate" title={a.nombre}>
                            {a.nombre}
                          </span>
                        </span>
                      </TableCell>
                      <TableCell className="pr-4 text-right font-semibold tabular-nums">{a.total}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </div>
        </Tarjeta>
      </div>

      {/* Usuarios en línea */}
      <Tarjeta>
        <CabeceraSeccion
          icon={WifiHighIcon}
          titulo={t("admin-dashboard-online-titulo")}
          subtitulo={t("admin-dashboard-online-subtitulo")}
          derecha={
            !loadingOnline ? (
              online.length > 0 ? (
                <PillEstado tono="ok">{online.length}</PillEstado>
              ) : (
                <Chip>{online.length}</Chip>
              )
            ) : undefined
          }
        />
        <div className="px-4 pt-1 pb-4">
          {loadingOnline ? (
            <div aria-busy="true" className="flex flex-col gap-1">
              {[1, 2, 3].map((i) => (
                <Skeleton key={i} className="h-[52px] rounded-lg" />
              ))}
            </div>
          ) : online.length === 0 ? (
            <Vacio icon={UsersIcon}>{t("admin-dashboard-online-vacio")}</Vacio>
          ) : (
            <ul className="grid grid-cols-1 gap-1 md:grid-cols-2">
              {online.map((u) => (
                <OnlineUserRow key={u.id} u={u} />
              ))}
            </ul>
          )}
        </div>
      </Tarjeta>

      {/* Ingresos */}
      <Tarjeta>
        <CabeceraSeccion
          icon={CoinsIcon}
          titulo={t("menu-admin-ingresos")}
          subtitulo={t("admin-dashboard-ingresos-subtitulo")}
          derecha={
            <Link href="/admin/ingresos" className={ACCION_SECUNDARIA}>
              {t("admin-ingresos-titulo")}
              <ArrowRightIcon aria-hidden weight={PESO_ICONO_FUNCIONAL} className="size-3.5 rtl:rotate-180" />
            </Link>
          }
          conBorde={false}
        />
      </Tarjeta>
    </div>
  );
}
