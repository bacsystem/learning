---
id: SIA-E2-07
titulo: Gestión de fuentes (admin)
epica: E2
fase: MVP
prioridad: Alta
estimacion: 5
roles: [admin]
depende_de: [SIA-E2-06]
---

# SIA-E2-07 Gestión de fuentes (admin)

**Como** admin, **quiero** una pantalla para subir, listar, ver estado, versiones y retirar fuentes, **para** administrar el repositorio sin ayuda técnica.

## Contexto
Pantalla nueva en el panel admin con khipu (`TablaListado`, formulario con formik/yup).

## Reglas de negocio
1. Listado paginado (20 por defecto, máx. 100) con título, tipo, profesión, versión vigente, estado, insignia «Oficial verificada», nº de pasajes y fecha.
2. Filtros: estado, tipo, profesión y oficial verificada (sí/no); búsqueda por título (máx. 100 caracteres).
3. Detalle: historial de versiones (con la marca oficial de cada una), entidad emisora y nota de verificación si está marcada, nº de contenidos que la citan y motivo de fallo si lo hay.
4. Acciones: subir, nueva versión, editar metadatos (regla 7), reintentar, retirar (con motivo), restaurar, marcar como oficial verificada (entidad emisora + nota) y desmarcar (con motivo), según SIA-E2-06.
5. Marcar como oficial verificada es una acción explícita con diálogo de confirmación que explica su efecto: las preguntas pendientes de ese balotario sin conflicto de clave se aprueban revisando solo clasificación y estructura (SIA-E3-05). Al subir una fuente nunca se marca sola.
6. Estados de UI: carga, vacío, error; el estado `procesando` se refresca cada 5 s.
7. Editar metadatos (decisión PO, contrato MVP E1+E2; necesario porque el duplicado es global, SIA-E2-03 regla 3): el admin puede cambiar el título (3–150) y las profesiones aplicables (0..N) de una fuente existente sin crear versión nueva. Se puede editar en los estados `activa`, `fallida` y `retirada`; en `procesando`, 409 `estado_invalido`. Si al añadir profesiones se supera un límite de fuentes, responde 422 `limite_fuentes` y no cambia nada. El cambio se audita como `fuente.editar` con valores antes/después. Tipo, año y proceso no se editan en el MVP.
8. Si una subida responde 409 `fuente_duplicada`, la pantalla muestra la fuente existente (título y estado) con un enlace a su detalle, desde donde se puede reintentar, restaurar o editar profesiones.

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

Escenario: Marcar como oficial verificada
  Dado una fuente activa de tipo balotario sin marca
  Cuando el admin pulsa «Marcar como oficial verificada», escribe entidad y nota y confirma
  Entonces la fila muestra la insignia «Oficial verificada» y el detalle muestra entidad y nota

Escenario: Validación del diálogo de marca
  Dado el diálogo de marca abierto
  Cuando deja vacía la entidad emisora o la nota
  Entonces ve el error en el campo y el botón confirmar está deshabilitado

Escenario: Subida sin marca automática
  Dado el formulario de subida de una fuente nueva
  Cuando el admin sube el archivo
  Entonces la fuente se crea sin la insignia «Oficial verificada»

Escenario: Editar profesiones
  Dado una fuente activa para Medicina
  Cuando el admin añade Enfermería y guarda
  Entonces la fuente aplica a Medicina y Enfermería, sigue en la misma versión y audit_log registra "fuente.editar" con antes/después

Escenario: Editar profesiones por encima del límite
  Dado Enfermería con 100 fuentes entre "activa" y "procesando"
  Cuando el admin añade Enfermería a otra fuente activa
  Entonces recibe 422 "limite_fuentes" y las profesiones no cambian

Escenario: Subida duplicada
  Dado una fuente existente con el mismo contenido
  Cuando el admin sube el archivo
  Entonces ve el aviso de duplicado con el título y el estado de la fuente existente y un enlace a su detalle

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
`tsc`, `lint`, `build` en verde; estados de UI verificados; claves es/en; tests de API de la edición de metadatos (permisos 403, límite 422, estado 409, audit_log); review limpio.
