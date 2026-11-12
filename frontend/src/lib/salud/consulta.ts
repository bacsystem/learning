import type { Salud } from "./reglas";

/**
 * Estado de la consulta de salud, sin React (PLT-02 reglas 4–6):
 * - una consulta a la vez: si ya hay una en curso, el sondeo o «Actualizar» no lanzan otra;
 * - si falla, se conserva el último resultado válido (que la pantalla marca como desactualizado) y queda el fallo;
 * - el siguiente resultado válido borra el fallo.
 */
export interface EstadoConsultaSalud {
  /** Último resultado válido, o `null` si todavía no hubo ninguno. */
  salud: Salud | null;
  /** Error de la última consulta, o `null` si fue correcta. */
  fallo: unknown;
  enCurso: boolean;
}

export const ESTADO_INICIAL_SALUD: EstadoConsultaSalud = { salud: null, fallo: null, enCurso: false };

export class ConsultaSalud {
  private estado: EstadoConsultaSalud = ESTADO_INICIAL_SALUD;
  private readonly oyentes = new Set<() => void>();

  suscribir = (oyente: () => void): (() => void) => {
    this.oyentes.add(oyente);
    return () => this.oyentes.delete(oyente);
  };

  obtenerEstado = (): EstadoConsultaSalud => this.estado;

  /** Pide el estado; los errores quedan en `fallo`, no se relanzan. */
  async consultar(pedir: () => Promise<Salud>): Promise<void> {
    if (this.estado.enCurso) return;
    this.actualizar({ enCurso: true });
    try {
      this.actualizar({ salud: await pedir(), fallo: null, enCurso: false });
    } catch (fallo) {
      this.actualizar({ fallo, enCurso: false });
    }
  }

  private actualizar(cambio: Partial<EstadoConsultaSalud>): void {
    this.estado = { ...this.estado, ...cambio };
    for (const oyente of this.oyentes) oyente();
  }
}

/** Lo que muestra la pantalla según el estado (PLT-02 regla 6). */
export type FaseSalud = "cargando" | "datos" | "error" | "desactualizado";

export function faseSalud({ salud, fallo }: EstadoConsultaSalud): FaseSalud {
  if (salud === null) return fallo === null ? "cargando" : "error";
  return fallo === null ? "datos" : "desactualizado";
}
