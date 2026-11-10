"use client";

/**
 * Piezas compartidas por los catálogos de administración (instituciones, grados, especialidades, asignaturas,
 * etiquetas, configuración de examen): pestañas de estado, pills, diálogo de formulario y diálogo de confirmación.
 * Todo compuesto con khipu; la lógica (API, validaciones, textos) vive en cada página.
 */

import { useTranslations } from "next-intl";
import { type FormEvent, type ReactNode, useState } from "react";

import { Formulario } from "@/components/formularios/campo";
import { Tabs } from "@/components/navegacion/tabs";
import { BotonAsync } from "@/components/patrones/boton-async";
import { Aviso, CabeceraDialogo, PieDialogo, type TonoDialogo } from "@/components/patrones/cabecera-dialogo";
import { Chip, PillEstado } from "@/components/patrones/pill-estado";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { BOTON_DESTRUCTIVO, BOTON_PRIMARIO, BOTON_SECUNDARIO } from "@/lib/estilos";
import type { IconoComponente } from "@/lib/iconos";
import { cn } from "@/lib/utils";
import { WarningCircleIcon } from "@phosphor-icons/react";

// ── Estado ──

export const TABS_ESTADO = ["todos", "activo", "inactivo"] as const;
export type TabEstado = (typeof TABS_ESTADO)[number];

/** `t` de `useTranslations("dashboard")`. */
type Traductor = (clave: string) => string;

const CLAVE_TAB: Record<TabEstado, string> = {
  todos: "simulacros-todos",
  activo: "simulacros-estado-activo",
  inactivo: "simulacros-estado-inactivo",
};

const CLAVE_VALOR: Record<string, string> = {
  activo: "simulacros-estado-activo",
  inactivo: "simulacros-estado-inactivo",
  suboficial: "admin-catalogos-nivel-suboficial",
  oficial: "admin-catalogos-nivel-oficial",
};

/** "activo" → "Activo", "oficial" → "Oficial", traducidos (para la pill, la búsqueda y el CSV). */
export function etiquetaEstado(estado: string | null | undefined, t: Traductor): string {
  if (!estado) return "";
  const clave = CLAVE_VALOR[estado];
  return clave ? t(clave) : estado.charAt(0).toUpperCase() + estado.slice(1);
}

/** Pestañas segmentadas Todos / Activo / Inactivo con su contador, para la barra de filtros de `TablaListado`. */
export function TabsEstado({
  valor,
  onCambio,
  filas,
}: {
  valor: TabEstado;
  onCambio: (v: TabEstado) => void;
  filas: readonly { estado: string }[];
}) {
  const t = useTranslations("dashboard");
  const items = TABS_ESTADO.map((id) => ({
    id,
    etiqueta: t(CLAVE_TAB[id]),
    contador: id === "todos" ? filas.length : filas.filter((f) => f.estado === id).length,
  }));
  // Solo se usan las pestañas como filtro: los paneles (vacíos) se ocultan para no sumar alto a la barra.
  return (
    <Tabs items={items} valor={valor} onCambio={onCambio} estilo="segmentado" className="[&_[role=tabpanel]]:hidden" />
  );
}

/** Pill del estado de un registro de catálogo: verde activo, neutro inactivo. */
export function PillEstadoCatalogo({ estado }: { estado: string }) {
  const t = useTranslations("dashboard");
  return <PillEstado tono={estado === "activo" ? "ok" : "neutro"}>{etiquetaEstado(estado, t)}</PillEstado>;
}

/** Chip de nivel (suboficial / oficial): categoría, no estado. */
export function ChipNivel({ nivel }: { nivel: string | null | undefined }) {
  const t = useTranslations("dashboard");
  if (!nivel) return <span className="text-texto-terciario">—</span>;
  return (
    <Chip className={nivel === "oficial" ? "bg-accent text-accent-foreground" : "bg-info text-info-foreground"}>
      {etiquetaEstado(nivel, t)}
    </Chip>
  );
}

/** Celda principal: el nombre en peso medio. */
export function CeldaNombre({ children }: { children: ReactNode }) {
  return <span className="text-foreground font-medium">{children}</span>;
}

// ── Diálogo de formulario (crear / editar) ──

/**
 * Diálogo de alta/edición: cabecera con icono, rejilla de campos (2 columnas en desktop), aviso de error del guardado
 * y pie con Cancelar + Guardar. Enter en un campo envía el formulario (si `puedeGuardar`).
 */
export function DialogoFormulario({
  open,
  onCerrar,
  icon,
  titulo,
  descripcion,
  error,
  pendiente,
  puedeGuardar = true,
  textoGuardar,
  textoGuardando,
  textoCancelar,
  onGuardar,
  ancho = "sm:max-w-lg",
  children,
}: {
  open: boolean;
  onCerrar: () => void;
  icon: IconoComponente;
  titulo: ReactNode;
  descripcion?: ReactNode;
  error?: string | null;
  pendiente: boolean;
  puedeGuardar?: boolean;
  textoGuardar: string;
  textoGuardando: string;
  textoCancelar: string;
  onGuardar: () => void;
  ancho?: string;
  children: ReactNode;
}) {
  function enviar(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (pendiente || !puedeGuardar) return;
    onGuardar();
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v && !pendiente) onCerrar();
      }}
    >
      <DialogContent className={cn("gap-0 p-0", ancho)}>
        <CabeceraDialogo icon={icon} titulo={titulo} descripcion={descripcion} />
        <Formulario columnas={1} className="gap-0" onSubmit={enviar} noValidate>
          <div className="grid gap-4 px-5 pt-1 pb-2">
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2">{children}</div>
            {error ? (
              <Aviso tono="peligro" icon={WarningCircleIcon}>
                {error}
              </Aviso>
            ) : null}
          </div>
          <PieDialogo>
            <button type="button" disabled={pendiente} onClick={onCerrar} className={BOTON_SECUNDARIO}>
              {textoCancelar}
            </button>
            <BotonAsync
              type="submit"
              pendiente={pendiente}
              textoPendiente={textoGuardando}
              disabled={!puedeGuardar}
              className={BOTON_PRIMARIO}
            >
              {textoGuardar}
            </BotonAsync>
          </PieDialogo>
        </Formulario>
      </DialogContent>
    </Dialog>
  );
}

// ── Diálogo de confirmación (activar / desactivar / eliminar) ──

/**
 * Confirmación modal con el registro afectado a la vista. `onConfirmar` devuelve un mensaje de error para mostrarlo sin
 * cerrar, o `null` si salió bien (entonces se cierra). Los textos llegan traducidos desde la página.
 */
export function DialogoConfirmarAccion({
  open,
  onCerrar,
  icon,
  tono,
  titulo,
  descripcion,
  registro,
  detalle,
  destructiva = false,
  textoConfirmar,
  textoPendiente,
  textoCancelar,
  onConfirmar,
}: {
  open: boolean;
  onCerrar: () => void;
  icon: IconoComponente;
  tono: TonoDialogo;
  titulo: ReactNode;
  descripcion?: ReactNode;
  /** Nombre del registro afectado (se muestra en una tarjeta bajo la descripción). */
  registro?: ReactNode;
  detalle?: ReactNode;
  destructiva?: boolean;
  textoConfirmar: string;
  textoPendiente: string;
  textoCancelar: string;
  onConfirmar: () => Promise<string | null>;
}) {
  const [pendiente, setPendiente] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function cerrar() {
    if (pendiente) return;
    setError(null);
    onCerrar();
  }

  async function confirmar() {
    setPendiente(true);
    setError(null);
    try {
      const r = await onConfirmar();
      if (r) {
        setError(r);
        return;
      }
      onCerrar();
    } finally {
      setPendiente(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) cerrar();
      }}
    >
      <DialogContent className="gap-0 p-0 sm:max-w-md">
        <CabeceraDialogo icon={icon} titulo={titulo} descripcion={descripcion} tono={tono} />
        {registro || error ? (
          <div className="grid gap-3 px-5 pt-1 pb-2">
            {registro ? (
              <div className="bg-secondary rounded-lg px-3 py-2.5">
                <p className="text-control text-foreground font-semibold">{registro}</p>
                {detalle ? <p className="text-etiqueta text-muted-foreground mt-0.5">{detalle}</p> : null}
              </div>
            ) : null}
            {error ? (
              <Aviso tono="peligro" icon={WarningCircleIcon}>
                {error}
              </Aviso>
            ) : null}
          </div>
        ) : null}
        <PieDialogo>
          <button type="button" disabled={pendiente} onClick={cerrar} className={BOTON_SECUNDARIO}>
            {textoCancelar}
          </button>
          <BotonAsync
            pendiente={pendiente}
            textoPendiente={textoPendiente}
            onClick={confirmar}
            className={destructiva ? BOTON_DESTRUCTIVO : BOTON_PRIMARIO}
          >
            {textoConfirmar}
          </BotonAsync>
        </PieDialogo>
      </DialogContent>
    </Dialog>
  );
}
