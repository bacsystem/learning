---
id: SIA-E9-06
titulo: Metas semanales y racha de estudio
epica: E9
fase: v3
prioridad: Baja
estimacion: 3
roles: [usuario]
depende_de: [SIA-E7-03]
---

# SIA-E9-06 Metas semanales y racha de estudio

**Como** postulante, **quiero** fijar una meta semanal de estudio y ver mi racha, **para** mantener la constancia. (Recomendación del PO.)

## Reglas de negocio
1. Meta semanal en minutos (30–2 400, pasos de 30); por defecto el total semanal del plan.
2. Un «día de estudio» es un día (America/Lima) con ≥ 10 minutos de actividad (práctica, simulacro, repaso, bloque del plan marcado).
3. Racha = días de estudio consecutivos hasta hoy; un día sin estudio reinicia a 0, con 1 «día de gracia» por semana (configurable).
4. Se muestra mejor racha y progreso semanal; sin rankings entre usuarios.
5. Sin textos de culpa: no hay mensajes negativos al perder la racha.
6. Desactivable en preferencias.

## Criterios de aceptación
```gherkin
Escenario: Racha
  Dado actividad de 10 minutos o más durante 5 días seguidos
  Cuando el usuario abre su progreso
  Entonces la racha es 5

Escenario: Día corto
  Dado un día con 8 minutos de actividad
  Cuando el sistema calcula la racha
  Entonces ese día no cuenta

Escenario: Día de gracia
  Dado un día sin estudio con día de gracia disponible
  Cuando el sistema calcula la racha
  Entonces la racha se conserva y se consume la gracia de la semana

Escenario: Meta fuera de rango
  Dado la pantalla de metas
  Cuando fija 20 minutos
  Entonces recibe 422

Escenario: Progreso
  Dado una meta de 600 minutos y 300 hechos
  Cuando el usuario abre su progreso semanal
  Entonces muestra 50 %

Escenario: Desactivar
  Dado un usuario con racha y meta activas
  Cuando desactiva la función en preferencias
  Entonces el usuario no ve racha ni meta

Escenario: Aislamiento
  Dado la racha de otro usuario
  Cuando un usuario intenta consultarla
  Entonces no existe ningún endpoint que la exponga y recibe 404
```

## Fuera de alcance
Insignias, rankings, notificaciones push.

## Dependencias
SIA-E7-03.

## Seguridad y privacidad
Datos solo del dueño.

## i18n
Textos en es/en.

## Definición de terminado
Tests de zona horaria Lima y día de gracia; `tsc`, `lint`, `build`; review limpio.
