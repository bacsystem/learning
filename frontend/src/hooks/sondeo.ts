/**
 * Sondeo encadenado: ejecuta `tarea` cada `intervalo` ms mientras `seguir()` sea verdadero. El siguiente ciclo se
 * programa cuando termina el anterior, así dos pedidos lentos nunca se apilan. `detener()` corta el ciclo pendiente.
 */
export interface Temporizador {
  setTimeout: (fn: () => void, ms: number) => unknown;
  clearTimeout: (id: unknown) => void;
}

export function crearSondeo({
  tarea,
  intervalo,
  seguir,
  temporizador = { setTimeout: (fn, ms) => setTimeout(fn, ms), clearTimeout: (id) => clearTimeout(id as number) },
}: {
  tarea: () => Promise<unknown>;
  intervalo: number;
  seguir: () => boolean;
  temporizador?: Temporizador;
}) {
  let pendiente: unknown = null;
  let activo = false;

  function programar() {
    if (!activo || !seguir()) {
      activo = false;
      return;
    }
    pendiente = temporizador.setTimeout(async () => {
      pendiente = null;
      try {
        await tarea();
      } catch {
        // Un ciclo fallido no corta el sondeo: el siguiente lo reintenta (el error ya lo maneja la tarea, si quiere).
      }
      programar();
    }, intervalo);
  }

  return {
    iniciar() {
      if (activo) return;
      activo = true;
      programar();
    },
    detener() {
      activo = false;
      if (pendiente !== null) temporizador.clearTimeout(pendiente);
      pendiente = null;
    },
  };
}
