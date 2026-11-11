---
id: SIA-E8-04
titulo: Exportación del plan a .ics
epica: E8
fase: v2
prioridad: Media
estimacion: 2
roles: [usuario]
depende_de: [SIA-E7-03]
---

# SIA-E8-04 Exportación del plan a .ics

**Como** postulante, **quiero** descargar mi plan y las fechas de mi proceso como archivo de calendario, **para** verlo en mi agenda del teléfono.

## Reglas de negocio
1. Descarga bajo demanda de un archivo `.ics` (RFC 5545) con: un evento por día de estudio (hora de inicio según el recordatorio, duración = minutos del día, descripción con áreas) y los hitos del proceso como eventos de día completo con alarma 1 día antes.
2. Zona horaria America/Lima declarada; cada evento tiene UID estable (regenerar el archivo actualiza eventos, no los duplica).
3. Máx. 400 eventos por archivo; si el plan es más largo, se exporta desde hoy hasta 400 eventos.
4. Autenticado y solo del dueño; no hay URL pública ni de suscripción en esta historia.
5. El contenido no incluye datos personales ni enlaces con tokens.
6. Nombre de archivo: `capacitate-plan.ics`.

## Criterios de aceptación
```gherkin
Escenario: Descarga
  Dado un plan activo de 30 días
  Cuando pulsa «Exportar a calendario»
  Entonces descarga un .ics con 30 eventos de estudio y los hitos del proceso

Escenario: Estabilidad de UID
  Dado un plan activo ya exportado una vez
  Cuando exporta dos veces
  Entonces los UID de los eventos son idénticos

Escenario: Límite
  Dado un plan de 500 días
  Cuando pulsa «Exportar a calendario»
  Entonces el archivo tiene como máximo 400 eventos y se informa del recorte

Escenario: Sin plan
  Dado un usuario sin plan activo
  Cuando pide la exportación
  Entonces recibe 409 "plan_sin_configuracion"

Escenario: Aislamiento
  Dado el plan de otro usuario
  Cuando un usuario pide el .ics de ese plan
  Entonces recibe 404

Escenario: Validez del archivo
  Dado un .ics exportado
  Cuando se valida con un validador RFC 5545 y se importa en un calendario estándar
  Entonces el archivo pasa la validación y se importa sin errores

Escenario: Privacidad
  Dado un .ics exportado
  Cuando se inspecciona su contenido
  Entonces el archivo no contiene email, DNI ni nombre del usuario
```

## Fuera de alcance
Suscripción por URL, sincronización bidireccional.

## Dependencias
SIA-E7-03, SIA-E8-01 (hitos).

## Seguridad y privacidad
Sin enlaces públicos ni tokens; escape correcto de texto (inyección de líneas).

## i18n
Títulos de evento en el idioma del usuario (es/en); botón y mensajes en es/en.

## Definición de terminado
Test con validador de .ics; escape de saltos de línea probado; review limpio.
