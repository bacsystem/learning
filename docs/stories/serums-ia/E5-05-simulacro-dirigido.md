---
id: SIA-E5-05
titulo: Acción rápida - simulacro dirigido
epica: E5
fase: v2
prioridad: Media
estimacion: 3
roles: [usuario]
depende_de: [SIA-E5-03]
---

# SIA-E5-05 Acción rápida: simulacro dirigido

**Como** postulante, **quiero** crear un simulacro cronometrado centrado en mis áreas bajas, **para** practicar bajo presión justo donde estoy débil.

## Reglas de negocio
1. Reutiliza `simulacros`: tiempo límite en servidor (409 `tiempo_agotado`), selección múltiple y nota sobre 100.
2. Tamaño 10–100 preguntas (defecto 30); tiempo = 1,5 min por pregunta, redondeado hacia arriba al minuto.
3. Distribución: 70 % de las preguntas en áreas débiles (máx. 3) y 30 % en el resto, repartidas por peso.
4. Se identifica como «dirigido» en el historial; su nota no se compara con simulacros completos.
5. Exige inscripción aprobada (403).
6. Un simulacro en curso por usuario y proceso (409 si ya existe uno).

## Criterios de aceptación
```gherkin
Escenario: Crear
  Dado 3 áreas débiles
  Cuando crea un simulacro de 30 preguntas
  Entonces 21 son de áreas débiles, 9 del resto y el tiempo es 45 min

Escenario: Etiqueta en historial
  Dado un simulacro dirigido creado
  Cuando el usuario abre su historial
  Entonces aparece como «Dirigido»

Escenario: Tamaño inválido
  Dado un usuario con inscripción aprobada
  Cuando pide 9 preguntas
  Entonces recibe 422

Escenario: Simulacro en curso
  Dado un simulacro en curso
  Cuando el usuario crea otro simulacro dirigido
  Entonces recibe 409 "simulacro_en_curso"

Escenario: Tiempo agotado
  Dado un simulacro dirigido con el plazo vencido
  Cuando el usuario responde una pregunta
  Entonces recibe 409 "tiempo_agotado" (comportamiento existente)

Escenario: Sin inscripción aprobada
  Dado un usuario sin inscripción aprobada
  Cuando crea un simulacro dirigido
  Entonces recibe 403
```

## Fuera de alcance
Formato oficial (SIA-E9-05).

## Dependencias
SIA-E5-03, simulacros existentes.

## Seguridad y privacidad
Autorización por dueño; estado y tiempo verificados en servidor.

## i18n
Textos y etiqueta «Dirigido» en es/en.

## Definición de terminado
Tests de proporción 70/30, tiempos y límites; review limpio.
