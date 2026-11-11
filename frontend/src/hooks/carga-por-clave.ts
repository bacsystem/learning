/**
 * Carga de datos asociada a una clave (p. ej. una asignatura, o el par proceso-especialidad), sin React.
 *
 * - Solo se aplica la respuesta de la última carga: una respuesta lenta de una clave anterior no pisa a la actual.
 * - `vistaDeCarga` nunca devuelve datos ni errores de otra clave: al cambiar de clave no quedan a la vista (ni
 *   editables) los de la anterior, y hasta que llega la carga de la clave actual se considera cargando.
 */
export interface EstadoCarga<T> {
  clave: string;
  datos: T | null;
  error: unknown;
  cargando: boolean;
}

export interface VistaCarga<T> {
  datos: T | null;
  error: unknown;
  cargando: boolean;
}

export class CargaPorClave<T> {
  private estado: EstadoCarga<T> = { clave: "", datos: null, error: null, cargando: false };
  private ultima = 0;
  private readonly oyentes = new Set<() => void>();

  suscribir = (oyente: () => void): (() => void) => {
    this.oyentes.add(oyente);
    return () => this.oyentes.delete(oyente);
  };

  obtenerEstado = (): EstadoCarga<T> => this.estado;

  /** Pide los datos de `clave`; con `clave = null` no pide nada. Los errores quedan en el estado, no se relanzan. */
  async cargar(clave: string | null, pedir: () => Promise<T>): Promise<void> {
    const id = ++this.ultima;
    if (!clave) {
      this.actualizar({ cargando: false });
      return;
    }
    this.actualizar({ cargando: true });
    try {
      const datos = await pedir();
      if (id === this.ultima) this.actualizar({ clave, datos, error: null, cargando: false });
    } catch (error) {
      if (id === this.ultima) this.actualizar({ clave, datos: null, error, cargando: false });
    }
  }

  /** Sustituye los datos de `clave` sin pedirlos (p. ej. con la respuesta de un guardado). */
  reemplazar(clave: string, datos: T): void {
    this.actualizar({ clave, datos, error: null });
  }

  private actualizar(cambio: Partial<EstadoCarga<T>>): void {
    this.estado = { ...this.estado, ...cambio };
    for (const oyente of this.oyentes) oyente();
  }
}

/** Lo que ve la pantalla para la clave actual. */
export function vistaDeCarga<T>(estado: EstadoCarga<T>, clave: string | null): VistaCarga<T> {
  const vigente = Boolean(clave) && estado.clave === clave;
  return {
    datos: vigente ? estado.datos : null,
    error: vigente ? estado.error : null,
    cargando: Boolean(clave) && (estado.cargando || !vigente),
  };
}
