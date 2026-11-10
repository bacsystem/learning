"use client";

import Image from "next/image";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";

import { Marca } from "@/components/app/marca";
import { Alerta } from "@/components/feedback/alerta";
import { EstadoVacio } from "@/components/feedback/estado-vacio";
import { Skeleton } from "@/components/feedback/skeleton";
import { Chip } from "@/components/patrones/pill-estado";
import { CONTACT_EMAIL } from "@/config";
import { useScrollReveal } from "@/hooks/use-scroll-reveal";
import { useAuth } from "@/lib/auth/auth-context";
import {
  ACCION_PRINCIPAL,
  ACCION_SECUNDARIA,
  ENTRADA,
  FOCO,
  pasoEntrada,
  ROTULO,
  TARJETA,
  TARJETA_ELEVABLE,
} from "@/lib/estilos";
import { fetchProcesosPublico, fetchStatsPublicos, Proceso, StatsPublicos } from "@/lib/inscripciones/api";
import { fetchTestimoniosPublicos, type TestimonioPublico } from "@/lib/testimonios/api";
import { cn } from "@/lib/utils";
import {
  ArrowRightIcon,
  BrainIcon,
  ChartLineIcon,
  type Icon,
  ListChecksIcon,
  RocketIcon,
  SealCheckIcon,
  ShieldCheckIcon,
  StarIcon,
  TrophyIcon,
  UserPlusIcon,
  UsersIcon,
} from "@phosphor-icons/react";

// ── Tono de cada tipo de proceso: icono en su color (sin caja) y pill con su relleno tenue ──
type TonoTipo = { icono: Icon; texto: string; pill: string };

/** Los tipos que acepta el backend (`oneof=ascenso pase_efectividad admision`), con su tono y su clave i18n. */
const TIPO_TONO: Record<string, TonoTipo & { clave: string }> = {
  ascenso: {
    icono: TrophyIcon,
    texto: "text-acento-2-texto dark:text-acento-2",
    pill: "bg-acento-2-suave text-acento-2-texto dark:text-acento-2",
    clave: "procesos-tipo-ascenso",
  },
  pase_efectividad: {
    icono: SealCheckIcon,
    texto: "text-success-foreground",
    pill: "bg-success text-success-foreground",
    clave: "procesos-tipo-pase-efectividad",
  },
  admision: {
    icono: ListChecksIcon,
    texto: "text-warning-foreground",
    pill: "bg-warning text-warning-foreground",
    clave: "admin-procesos-tipo-admision",
  },
};

const TONO_POR_DEFECTO: TonoTipo = {
  icono: BrainIcon,
  texto: "text-muted-foreground",
  pill: "bg-secondary text-muted-foreground",
};

function procesoTono(tipo: string): TonoTipo & { clave?: string } {
  return TIPO_TONO[tipo.toLowerCase()] ?? TONO_POR_DEFECTO;
}

function fmtNum(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(n % 1000 === 0 ? 0 : 1)}K+`;
  return `${n}+`;
}

/** Pasos de «Cómo funciona»: `clave` es el prefijo de sus textos (`-titulo`, `-desc`) en los mensajes. */
const STEPS: { icon: Icon; clave: string }[] = [
  { icon: UserPlusIcon, clave: "landing-paso-registro" },
  { icon: SealCheckIcon, clave: "landing-paso-aprobacion" },
  { icon: RocketIcon, clave: "landing-paso-preparacion" },
];

/** Bloque que aparece (fundido + 20 px) cuando entra en pantalla; `useScrollReveal` le agrega `sr-revealed`. Con «reducir movimiento» se ve desde el inicio. */
const REVELAR =
  "motion-safe:transition-[opacity,transform] motion-safe:duration-(--duracion-lenta) ease-salida motion-safe:[&:not(.sr-revealed)]:translate-y-5 motion-safe:[&:not(.sr-revealed)]:opacity-0";

const CONTENEDOR = "mx-auto w-full max-w-6xl px-4 md:px-6";

const ENLACE_PIE = cn(
  "rounded-sm text-control text-muted-foreground transition-colors duration-(--duracion-rapida) hover:text-foreground",
  FOCO,
);

/** Cabecera centrada de cada sección: pill, título y bajada. */
function CabeceraLanding({
  etiqueta,
  titulo,
  children,
}: {
  etiqueta: string;
  titulo: string;
  children: React.ReactNode;
}) {
  return (
    <header className="mx-auto mb-10 flex max-w-xl flex-col items-center gap-3 text-center">
      <span className="bg-acento-superficie text-etiqueta text-acento-superficie-texto ring-acento-borde rounded-full px-3 py-1 font-semibold ring-1">
        {etiqueta}
      </span>
      <h2 className="text-titulo-pagina text-foreground text-balance">{titulo}</h2>
      <p className="text-cuerpo text-muted-foreground leading-relaxed">{children}</p>
    </header>
  );
}

/** Escudo de la institución como marca de agua en la esquina de una tarjeta. */
function MarcaAgua() {
  return (
    <Image
      src="/images/ep/army.png"
      alt=""
      aria-hidden="true"
      width={44}
      height={44}
      className="pointer-events-none absolute top-3 right-3 size-11 opacity-15 mix-blend-multiply select-none dark:opacity-10 dark:mix-blend-normal"
    />
  );
}

export default function Home() {
  const t = useTranslations("dashboard");
  const { isAuthenticated, loading } = useAuth();
  const [procesos, setProcesos] = useState<Proceso[]>([]);
  const [loadingProcesos, setLoadingProcesos] = useState(true);
  const [errorProcesos, setErrorProcesos] = useState(false);
  const [stats, setStats] = useState<StatsPublicos | null>(null);
  const [testimonios, setTestimonios] = useState<TestimonioPublico[]>([]);

  const statsRef = useScrollReveal<HTMLDivElement>();
  const procesosRef = useScrollReveal<HTMLDivElement>();
  const testimoniosRef = useScrollReveal<HTMLDivElement>();
  const stepsRef = useScrollReveal<HTMLOListElement>();
  const ctaRef = useScrollReveal<HTMLDivElement>();

  useEffect(() => {
    fetchProcesosPublico()
      .then(setProcesos)
      .catch(() => setErrorProcesos(true))
      .finally(() => setLoadingProcesos(false));
    fetchStatsPublicos()
      .then(setStats)
      .catch(() => {});
  }, []);

  useEffect(() => {
    fetchTestimoniosPublicos()
      .then(setTestimonios)
      .catch(() => {});
  }, []);

  if (loading) return null;

  const metricas = [
    { icon: UsersIcon, value: stats ? fmtNum(stats.total_usuarios) : "—", label: t("landing-metrica-usuarios") },
    {
      icon: ListChecksIcon,
      value: stats ? fmtNum(stats.total_simulacros) : "—",
      label: t("landing-metrica-simulacros"),
    },
    { icon: ChartLineIcon, value: stats ? fmtNum(stats.total_practicas) : "—", label: t("landing-metrica-practicas") },
    { icon: ShieldCheckIcon, value: stats ? `${stats.total_procesos}` : "—", label: t("landing-metrica-procesos") },
  ];

  return (
    <div className="bg-background text-foreground min-h-dvh">
      {/* ── Navbar ── */}
      <nav className="border-border/80 bg-card/80 sticky top-0 z-50 border-b backdrop-blur">
        <div className={cn(CONTENEDOR, "flex h-14 items-center justify-between gap-3")}>
          <Link href="/" aria-label={t("landing-inicio-aria")} className={cn("rounded-lg", FOCO)}>
            <Marca />
          </Link>
          <div className="flex items-center gap-2">
            <Link href="/auth/sign-in" className={ACCION_SECUNDARIA}>
              {t("sign-in-ingresar")}
            </Link>
            <Link href="/auth/sign-up" className={ACCION_PRINCIPAL}>
              {t("landing-registrarme")}
            </Link>
          </div>
        </div>
      </nav>

      <main>
        {/* ── Hero ── */}
        <section className="relative overflow-hidden">
          <div
            aria-hidden
            className="bg-acento-suave pointer-events-none absolute inset-x-0 -top-40 mx-auto h-[420px] max-w-4xl rounded-full opacity-70 blur-3xl dark:opacity-40"
          />
          <div
            className={cn(
              CONTENEDOR,
              "relative flex flex-col items-center gap-6 pt-20 pb-16 text-center md:pt-28 md:pb-24",
            )}
          >
            <span
              className={cn(
                ENTRADA,
                "bg-acento-superficie text-etiqueta text-acento-superficie-texto ring-acento-borde rounded-full px-3 py-1 font-semibold ring-1",
              )}
              style={pasoEntrada(0)}
            >
              {t("landing-hero-etiqueta")}
            </span>
            {/* Excepción de escala: el titular de la landing pública es el único texto de display de la app. */}
            <h1
              className={cn(
                ENTRADA,
                "font-heading text-foreground max-w-3xl text-4xl font-bold tracking-tight text-balance md:text-6xl",
              )}
              style={pasoEntrada(1)}
            >
              {t.rich("landing-hero-titulo", {
                resaltado: (texto) => (
                  <span className="bg-acento-superficie text-acento-superficie-texto rounded-xl box-decoration-clone px-2">
                    {texto}
                  </span>
                ),
              })}
            </h1>
            <p
              className={cn(ENTRADA, "text-titulo text-muted-foreground max-w-xl leading-relaxed font-normal")}
              style={pasoEntrada(2)}
            >
              {t("landing-hero-descripcion")}
            </p>
            <div className={cn(ENTRADA, "flex flex-wrap justify-center gap-2")} style={pasoEntrada(3)}>
              <Link href="/auth/sign-up" className={ACCION_PRINCIPAL}>
                <RocketIcon className="size-4" />
                {t("landing-empezar-gratis")}
              </Link>
              <Link href="/auth/sign-in" className={ACCION_SECUNDARIA}>
                {t("landing-ya-tengo-cuenta")}
              </Link>
            </div>
          </div>
        </section>

        {/* ── Stats ── */}
        <section aria-label={t("landing-stats-aria")} className="pb-16 md:pb-24">
          <div ref={statsRef} className={cn(CONTENEDOR, REVELAR, "grid grid-cols-2 gap-3 lg:grid-cols-4")}>
            {metricas.map((s) => (
              <div key={s.label} className={cn(TARJETA_ELEVABLE, "flex flex-col gap-2 p-4 md:p-5")}>
                <s.icon className="text-acento-2-texto dark:text-acento-2 size-5" />
                <p className="text-metrica text-foreground tabular-nums">{s.value}</p>
                <p className="text-control text-muted-foreground">{s.label}</p>
              </div>
            ))}
          </div>
        </section>

        {/* ── Procesos ── */}
        <section className="border-border/60 border-t py-16 md:py-24">
          <div className={CONTENEDOR}>
            <CabeceraLanding etiqueta={t("landing-procesos-etiqueta")} titulo={t("landing-procesos-titulo")}>
              {t("landing-procesos-descripcion")}
            </CabeceraLanding>

            {loadingProcesos ? (
              <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4" aria-busy="true">
                {[...Array(4)].map((_, i) => (
                  <div key={i} className={cn(TARJETA, "flex flex-col gap-3 p-5")}>
                    <Skeleton className="size-6 rounded-full" />
                    <Skeleton className="h-3 w-24" />
                    <Skeleton className="h-4 w-3/4" />
                    <Skeleton className="h-3 w-full" />
                    <Skeleton className="mt-2 h-8 w-full rounded-full" />
                  </div>
                ))}
              </div>
            ) : errorProcesos ? (
              <Alerta tono="error" className="mx-auto max-w-xl">
                {t("landing-procesos-error")}
              </Alerta>
            ) : procesos.length === 0 ? (
              <EstadoVacio icon={ListChecksIcon} titulo={t("landing-procesos-vacio")} className="mx-auto max-w-xl" />
            ) : (
              <div ref={procesosRef} className={cn(REVELAR, "grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4")}>
                {procesos.map((p) => {
                  const tono = procesoTono(p.tipo);
                  return (
                    <article
                      key={p.id}
                      className={cn(TARJETA_ELEVABLE, "relative flex flex-col gap-3 overflow-hidden p-5")}
                    >
                      <MarcaAgua />
                      <tono.icono className={cn("size-6", tono.texto)} />
                      <div className="flex flex-1 flex-col gap-1.5">
                        <div className="flex flex-wrap items-center gap-1.5">
                          <Chip className={tono.pill}>{tono.clave ? t(tono.clave) : p.tipo}</Chip>
                          <Chip>{p.anio}</Chip>
                        </div>
                        <h3 className="text-titulo text-foreground">{p.nombre}</h3>
                        {p.grado ? <p className="text-etiqueta text-texto-terciario">{p.grado}</p> : null}
                      </div>
                      {p.descripcion ? (
                        <p className="text-control text-muted-foreground leading-relaxed">{p.descripcion}</p>
                      ) : null}
                      <Link href="/auth/sign-up" className={cn(ACCION_SECUNDARIA, "w-full justify-center")}>
                        {t("landing-inscribirme")}
                      </Link>
                    </article>
                  );
                })}
              </div>
            )}
          </div>
        </section>

        {/* ── Testimonios ── */}
        <section className="border-border/60 border-t py-16 md:py-24">
          <div className={CONTENEDOR}>
            <CabeceraLanding etiqueta={t("landing-testimonios-etiqueta")} titulo={t("landing-testimonios-titulo")}>
              {t("landing-testimonios-descripcion")}
            </CabeceraLanding>

            {testimonios.length === 0 ? (
              <p className="text-control text-muted-foreground py-6 text-center">{t("landing-testimonios-vacio")}</p>
            ) : (
              <div ref={testimoniosRef} className={cn(REVELAR, "grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3")}>
                {testimonios.slice(0, 6).map((testimonio) => (
                  <figure
                    key={testimonio.id}
                    className={cn(TARJETA_ELEVABLE, "relative flex flex-col gap-4 overflow-hidden p-5")}
                  >
                    <MarcaAgua />
                    <div
                      className="flex gap-0.5"
                      role="img"
                      aria-label={t("landing-calificacion-aria", { n: testimonio.calificacion })}
                    >
                      {[1, 2, 3, 4, 5].map((star) => (
                        <StarIcon
                          key={star}
                          weight="fill"
                          aria-hidden
                          className={cn(
                            "size-4",
                            star <= testimonio.calificacion ? "text-warning-solid" : "text-border",
                          )}
                        />
                      ))}
                    </div>
                    <blockquote className="text-cuerpo text-muted-foreground flex-1 leading-relaxed">
                      &ldquo;{testimonio.contenido}&rdquo;
                    </blockquote>
                    <figcaption className="text-control text-foreground font-semibold">
                      {testimonio.nombre_publico}
                    </figcaption>
                  </figure>
                ))}
              </div>
            )}

            <div className="mt-8 flex justify-center">
              {isAuthenticated ? (
                <Link href="/settings#opinion" className={ACCION_SECUNDARIA}>
                  {t("landing-dejar-opinion")}
                </Link>
              ) : (
                <Link href="/auth/sign-up" className={ACCION_SECUNDARIA}>
                  {t("landing-registrate-opinar")}
                </Link>
              )}
            </div>
          </div>
        </section>

        {/* ── Cómo funciona ── */}
        <section className="border-border/60 border-t py-16 md:py-24">
          <div className={CONTENEDOR}>
            <CabeceraLanding etiqueta={t("landing-pasos-etiqueta")} titulo={t("landing-pasos-titulo")}>
              {t("landing-pasos-descripcion")}
            </CabeceraLanding>

            <ol ref={stepsRef} className={cn(REVELAR, "grid grid-cols-1 gap-3 md:grid-cols-3")}>
              {STEPS.map((step, i) => (
                <li key={step.clave} className={cn(TARJETA, "flex flex-col gap-3 p-5")}>
                  <div className="flex items-center justify-between">
                    <step.icon className="text-acento-2-texto dark:text-acento-2 size-6" />
                    <span className="bg-acento-superficie text-control text-acento-superficie-texto ring-acento-borde flex size-7 items-center justify-center rounded-full font-semibold tabular-nums ring-1">
                      {i + 1}
                    </span>
                  </div>
                  <h3 className="text-titulo text-foreground">{t(`${step.clave}-titulo`)}</h3>
                  <p className="text-control text-muted-foreground leading-relaxed">{t(`${step.clave}-desc`)}</p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        {/* ── CTA final ── */}
        <section className="py-16 md:py-24">
          <div className={CONTENEDOR}>
            <div
              ref={ctaRef}
              className={cn(
                REVELAR,
                "bg-acento-superficie ring-acento-borde flex flex-col items-center gap-4 rounded-xl px-6 py-12 text-center ring-1 md:py-16",
              )}
            >
              <h2 className="text-titulo-pagina text-acento-superficie-texto text-balance">
                {t("landing-cta-titulo")}
              </h2>
              <p className="text-cuerpo text-acento-superficie-texto max-w-md leading-relaxed">
                {t("landing-cta-descripcion")}
              </p>
              <Link href="/auth/sign-up" className={cn(ACCION_PRINCIPAL, "mt-2")}>
                <UserPlusIcon className="size-4" />
                {t("landing-crear-cuenta")}
              </Link>
            </div>
          </div>
        </section>
      </main>

      {/* ── Footer ── */}
      <footer className="border-border/80 bg-card border-t">
        <div className={CONTENEDOR}>
          <div className="grid grid-cols-1 gap-8 py-10 md:grid-cols-[2fr_1fr_1fr] md:py-12">
            {/* Brand */}
            <div className="flex flex-col gap-3">
              <Marca />
              <p className="text-control text-muted-foreground max-w-xs leading-relaxed">
                {t("landing-pie-descripcion")}
              </p>
            </div>

            {/* Plataforma */}
            <div className="flex flex-col gap-3">
              <h2 className={ROTULO}>{t("landing-pie-plataforma")}</h2>
              <ul className="flex flex-col gap-2">
                {[
                  { label: t("sign-in-ingresar"), href: "/auth/sign-in" },
                  { label: t("landing-registrarme"), href: "/auth/sign-up" },
                  { label: t("auth-recuperar-contrasena"), href: "/auth/password-reset" },
                ].map((l) => (
                  <li key={l.href}>
                    <Link href={l.href} className={ENLACE_PIE}>
                      {l.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>

            {/* Soporte */}
            <div className="flex flex-col gap-3">
              <h2 className={ROTULO}>{t("landing-pie-soporte")}</h2>
              <ul className="flex flex-col gap-2">
                {/* La empresa va como texto (sin enlace muerto); CONTACT_EMAIL siempre tiene valor (config.ts). */}
                <li className="text-control text-muted-foreground">Bacsystem Solutions</li>
                <li>
                  <a href={`mailto:${CONTACT_EMAIL}`} className={ENLACE_PIE}>
                    {t("landing-pie-contacto")}
                  </a>
                </li>
              </ul>
            </div>
          </div>

          {/* Bottom bar */}
          <div className="border-border/60 flex flex-wrap items-center justify-between gap-3 border-t py-5">
            <p className="text-etiqueta text-texto-terciario">
              {t("landing-copyright", { anio: new Date().getFullYear() })}
            </p>
            <Link href="/auth/sign-in" className={cn(ENLACE_PIE, "text-etiqueta inline-flex items-center gap-1")}>
              {t("landing-ingresar-plataforma")}
              <ArrowRightIcon weight="bold" className="size-3" />
            </Link>
          </div>
        </div>
      </footer>
    </div>
  );
}
