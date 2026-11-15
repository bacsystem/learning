const FECHA_SERVIDOR = /^(\d{2})\/(\d{2})\/(\d{4}) (\d{2}):(\d{2})$/;

/**
 * Instante ISO (UTC) de una fecha de inscripción del backend. El servidor la manda ya formateada, «dd/mm/yyyy hh:mm»
 * y sin zona (inscripciones/service.go), en UTC. Devuelve `null` si no tiene ese formato. Si el contrato pasa a ISO,
 * este adaptador desaparece.
 */
export function instanteDeFechaServidor(fecha: string | null | undefined): string | null {
  const m = FECHA_SERVIDOR.exec(fecha ?? "");
  if (!m) return null;
  const [, dia, mes, anio, hora, minuto] = m;
  return `${anio}-${mes}-${dia}T${hora}:${minuto}:00Z`;
}
