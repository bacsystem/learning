---
id: SIA-E9-02
titulo: Cuaderno de errores
epica: E9
fase: v2
prioridad: Media
estimacion: 3
roles: [usuario]
depende_de: [SIA-E9-01]
---

# SIA-E9-02 Cuaderno de errores

**Como** postulante, **quiero** un cuaderno con todas las preguntas que he fallado, con filtros y notas, **para** estudiar mis errores con calma.

## Reglas de negocio
1. Lista las preguntas falladas por el usuario (aprobadas) con: enunciado, mi última respuesta, la clave, la explicación aprobada si existe, área, tema, veces falladas y estado de repaso.
2. Filtros: área, tema, estado (pendiente de repaso, dominada) y rango de fechas; búsqueda de texto del enunciado (3–100 caracteres).
3. Nota personal por pregunta, hasta 500 caracteres, solo visible para el dueño.
4. Acciones: «Repasar» (SIA-E9-01) y «Practicar este tema» (SIA-E5-04).
5. Paginación 20 por defecto, máx. 100; ordenado por último fallo descendente.
6. Una pregunta que sale de aprobada deja de mostrarse (conserva la nota).

## Criterios de aceptación
```gherkin
Escenario: Listar
  Dado 45 preguntas falladas
  Cuando el usuario abre el cuaderno
  Entonces ve 20 en la primera página con clave, área y veces falladas

Escenario: Filtrar
  Dado un cuaderno con errores de varias áreas
  Cuando filtra por el área "Salud pública"
  Entonces ve solo las de esa área

Escenario: Nota
  Dado una pregunta del cuaderno
  Cuando guarda una nota de 500 caracteres y luego una de 501
  Entonces la primera se conserva y la segunda recibe 422

Escenario: Nota ajena
  Dado una nota personal de otro usuario
  Cuando un usuario intenta leerla
  Entonces recibe 404

Escenario: Explicación
  Dado una pregunta del cuaderno con explicación aprobada y otra sin ella
  Cuando el usuario abre ambas
  Entonces se muestra la explicación con sus citas en la primera y solo la clave en la segunda

Escenario: Vacío
  Dado ningún error registrado
  Cuando abre el cuaderno
  Entonces ve «Aún no tienes errores registrados» y un enlace a practicar

Escenario: Pregunta retirada
  Dado una pregunta con nota que pasa a "en_revision"
  Cuando el usuario abre el cuaderno
  Entonces no aparece, y reaparece con su nota si se revalida

Escenario: Estados de UI
  Dado el cuaderno de errores
  Cuando la petición está en curso o falla
  Entonces hay carga y error con reintento
```

## Fuera de alcance
Exportar a PDF; compartir cuaderno.

## Dependencias
SIA-E9-01; SIA-E6-08 para explicaciones.

## Seguridad y privacidad
Notas como texto no confiable (escape); privadas.

## i18n
Todo en es/en.

## Definición de terminado
`tsc`, `lint`, `build`; tests de límites y aislamiento; review limpio.
