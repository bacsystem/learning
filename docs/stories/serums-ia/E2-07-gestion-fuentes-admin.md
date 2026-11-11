---
id: SIA-E2-07
titulo: Gestión de fuentes (admin)
epica: E2
fase: MVP
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: [SIA-E2-06]
---

# SIA-E2-07 Gestión de fuentes (admin)

**Como** admin, **quiero** una pantalla para subir, listar, ver estado, versiones y retirar fuentes, **para** administrar el repositorio sin ayuda técnica.

## Contexto
Pantalla nueva en el panel admin con khipu (`TablaListado`, formulario con formik/yup).

## Reglas de negocio
1. Listado paginado (20 por defecto, máx. 100) con título, tipo, profesión, versión vigente, estado, nº de pasajes y fecha.
2. Filtros: estado, tipo y profesión; búsqueda por título (máx. 100 caracteres).
3. Detalle: historial de versiones, nº de contenidos que la citan y motivo de fallo si lo hay.
4. Acciones: subir, nueva versión, reintentar, retirar (con motivo), restaurar.
5. Estados de UI: carga, vacío, error; el estado `procesando` se refresca cada 5 s.

## Criterios de aceptación
```gherkin
Escenario: Listado
  Dado 25 fuentes
  Cuando el admin abre la pantalla
  Entonces ve 20 en la primera página y paginación a la segunda

Escenario: Estado vacío
  Dado ninguna fuente registrada
  Cuando el admin abre la pantalla
  Entonces ve «Aún no hay fuentes» y el botón «Subir fuente»

Escenario: Seguimiento de procesamiento
  Dado una fuente "procesando" visible en el listado
  Cuando el worker termina de procesarla
  Entonces la fila se actualiza sola a "activa" o "fallida" sin recargar la página

Escenario: Validación en el cliente
  Dado el formulario de subida abierto
  Cuando selecciona un archivo de 3 MiB
  Entonces ve el error de tamaño antes de enviar

Escenario: Retirar con confirmación
  Dado una fuente activa citada por varios contenidos
  Cuando pulsa Retirar
  Entonces debe escribir un motivo y ve cuántos contenidos se marcarán «en revisión» antes de confirmar

Escenario: Acceso
  Dado un usuario con rol usuario
  Cuando abre la ruta de gestión de fuentes
  Entonces la ruta lo redirige y la API responde 403
```

## Fuera de alcance
Visor de pasajes con edición.

## Dependencias
SIA-E2-03, E2-04 (opcional), E2-06.

## Seguridad y privacidad
Guard de rol admin; no mostrar rutas de archivos del servidor.

## i18n
Todo en es/en.

## Definición de terminado
`tsc`, `lint`, `build` en verde; estados de UI verificados; claves es/en; review limpio.
