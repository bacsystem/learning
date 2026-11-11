# ADR 0006: puerto del servicio de IA, sin proveedor elegido

- **Estado:** propuesto (2026-11-11). Pendiente de revisión del tech-lead.
- **Fecha:** 2026-11-11
- **Historias:** SIA-E6-01; lo consumen E3-04, E6-03, E6-06, E6-08 y siguientes
- **Contrato:** [docs/api/serums-ia-e6-01-servicio-ia.md](../api/serums-ia-e6-01-servicio-ia.md)

## Contexto

Varias historias necesitan «el servicio de IA». El proveedor, el modelo y los prompts son decisión técnica, pero **llamar a APIs de pago requiere la aprobación del usuario** (SIA-E6-01, Contexto), y todavía no hay ninguno aprobado. La aplicación maneja datos personales (padrones, personas) que no pueden salir hacia un tercero. Además, el servidor HTTP corta cada respuesta a los 15 s (`WriteTimeout`, `cmd/server/main.go`), y una llamada de IA puede tardar hasta 60 s.

## Opciones consideradas

1. **Integrar ya el SDK de un proveedor.** Ata el código a un proveedor no aprobado y agrega una dependencia. Descartada.
2. **Puerto propio (`Proveedor`) y un servicio que concentra límites, redacción, reintentos, validación y registro, con un doble determinista como único adaptador en E6-01.** **Elegida.**
3. **Validar las salidas con una librería de JSON Schema.** Agrega una dependencia. Las salidas son pocas y tipadas: con structs, `DisallowUnknownFields` y `validator/v10`, que ya es dependencia directa (`go.mod:8`), alcanza. Descartada.

## Decisión

- **Paquete:** `internal/ia`, con el puerto `Proveedor` y el `Servicio` de cuatro operaciones tipadas (contrato §1).
- **Doble:** el doble determinista es el único adaptador en E6-01.
- **Adaptador real:** se agrega en SIA-E6-02, con la aprobación del usuario y su propio ADR (proveedor, modelo, costos y región de los datos).
- **El servicio rechaza antes de llamar** cuando la entrada pasa de 30 000 runes, trae datos personales (DNI, email o teléfono) o el servicio está deshabilitado. Ese tipo de datos nunca llega a un tercero.
- **El doble falla cerrado:** solo se acepta con `ENV` `development`, `test` o `preprod`. Con cualquier otro valor la aplicación no arranca, así un alumno nunca recibe contenido ficticio.
- **Registro de uso en `ia_usos`**, una fila por invocación, sin contenido ni texto detectado, escrita fuera de la transacción del consumidor.
- **Las llamadas de IA no van en requests síncronos:** el peor caso son 6 llamadas de hasta 60 s, frente al `WriteTimeout` de 15 s. Los consumidores las ejecutan en jobs o workers. Es una obligación para cada historia consumidora.

## Consecuencias

- **Positivas:**
  - Se puede cambiar de proveedor tocando solo un adaptador.
  - Los tests y la preproducción no cuestan nada.
  - Los datos personales quedan bloqueados en un único punto.
  - Los costos y el uso quedan auditados.
- **Negativas:**
  - Rechazar por patrón tiene falsos positivos, por ejemplo una fecha `20261111` (8 dígitos) o el teléfono de contacto de una norma. El consumidor tiene que limpiar sus entradas; enmascarar en lugar de rechazar se evalúa en E6-03 (pregunta al PO, contrato §7).
  - Cada consumidor necesita infraestructura asíncrona (job y estado), y eso aumenta el costo de E3-04 y E6-03.
  - Hasta E6-02, con `IA_PROVEEDOR` vacío, ninguna función de IA está disponible en producción.
