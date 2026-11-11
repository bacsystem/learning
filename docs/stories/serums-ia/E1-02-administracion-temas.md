---
id: SIA-E1-02
titulo: Administración de temas
epica: E1
fase: MVP
prioridad: Alta
estimacion: 5
roles: [admin]
depende_de: [SIA-E1-01]
---

# SIA-E1-02 Administración de temas

**Como** admin, **quiero** una pantalla para listar, crear, editar, archivar y reordenar temas por área y profesión, **para** mantener la taxonomía SERUMS sin tocar la base de datos.

## Contexto
Pantalla nueva en el panel admin, junto a los catálogos. Usa `TablaListado` y componentes khipu.

## Reglas de negocio
1. Filtros: asignatura (obligatorio), especialidad y estado (activo/archivado).
2. Paginación de 20 por defecto, máx. 100.
3. Editar nombre, descripción y especialidades asociadas; reglas de SIA-E1-01 aplican.
4. Archivar pide confirmación y muestra cuántas preguntas tiene el tema.
5. Estados de UI: carga, vacío («Aún no hay temas en esta área») y error con reintento.
6. Reordenar (decisión PO, contrato MVP E1+E2): el orden es por asignatura y solo entre sus temas `activo`. El botón «Reordenar» abre un modo que muestra todos los temas activos de la asignatura en una sola lista, sin paginación ni filtro de especialidad (son 60 como máximo). Cada fila tiene botones «Subir» y «Bajar»; no se arrastra. «Guardar orden» envía la lista completa de ids de los temas activos de la asignatura en el nuevo orden, y el servidor la guarda de forma atómica.
7. Errores al reordenar: si la lista trae ids repetidos o mal formados, 422 `orden_invalido`. Si no coincide exactamente con los temas activos actuales de la asignatura (falta alguno, sobra alguno, hay un archivado o uno de otra asignatura), 409 `orden_desactualizado`: no se guarda nada y la UI pide recargar. Un tema nuevo queda al final (SIA-E1-01, regla 7). Un tema archivado sale del orden.
8. El reordenamiento se audita como `tema.reordenar` con el orden anterior y el nuevo.
9. Reactivar (Q10): en un tema archivado, la fila ofrece «Reactivar» con confirmación (reglas de SIA-E1-01, 6a). Si el servidor responde `limite_temas`, se muestra el mensaje y el tema sigue archivado.

## Criterios de aceptación
```gherkin
Escenario: Listar por área
  Dado una asignatura con 12 temas
  Cuando el admin la selecciona
  Entonces ve 12 filas con nombre, especialidades, número de preguntas y estado

Escenario: Estado vacío
  Dado una asignatura sin temas
  Cuando el admin la selecciona
  Entonces ve el mensaje de vacío y el botón "Crear tema"

Escenario: Editar con error de validación
  Dado el formulario de edición de un tema
  Cuando el admin deja el nombre con 2 caracteres y pulsa Guardar
  Entonces ve el error de validación bajo el campo y no se envía la petición

Escenario: Archivar con confirmación
  Dado un tema activo con 10 preguntas
  Cuando el admin pulsa Archivar
  Entonces aparece un diálogo con el número de preguntas afectadas
  Y al confirmar el tema pasa a archivado y se registra en audit_log

Escenario: Reordenar temas
  Dado una asignatura con los temas activos A, B y C en ese orden
  Cuando el admin entra en «Reordenar», sube C dos posiciones y pulsa «Guardar orden»
  Entonces el listado muestra C, A y B, y audit_log registra "tema.reordenar" con el orden anterior y el nuevo

Escenario: Orden desactualizado
  Dado el modo «Reordenar» abierto con los temas A, B y C
  Cuando otro admin archiva B y el primero guarda la lista A, B, C
  Entonces recibe 409 "orden_desactualizado", el orden no cambia y la pantalla pide recargar

Escenario: Lista de orden inválida
  Dado una asignatura con los temas activos A y B
  Cuando se envía la lista A, A
  Entonces recibe 422 "orden_invalido" y el orden no cambia

Escenario: Reactivar desde la pantalla
  Dado el filtro de estado en "archivado" y un tema archivado
  Cuando el admin pulsa «Reactivar» y confirma
  Entonces el tema sale de la lista de archivados y aparece activo al final del orden

Escenario: Acceso de usuario
  Dado un usuario con rol usuario
  Cuando abre la ruta de administración de temas
  Entonces es redirigido según el guard de rol admin

Escenario: Falla de red
  Dado un error 500 del servidor
  Cuando el admin guarda un tema
  Entonces ve un mensaje de error y los datos del formulario se conservan
```

## Fuera de alcance
Importar temas masivamente; fusionar temas; reordenar arrastrando; ordenar temas archivados.

## Dependencias
SIA-E1-01.

## Seguridad y privacidad
Ruta bajo el guard de admin; la API vuelve a verificar el rol.

## i18n
Todas las etiquetas, vacíos y errores en es/en (`dashboard.serumsIa.temas.*`).

## Definición de terminado
`tsc`, `lint` y `build` en verde; claves en es y en; estados carga/vacío/error verificados; review limpio.
