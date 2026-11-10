---
id: SIA-E5-02
titulo: Detalle por tema y evolución
epica: E5
fase: v2
prioridad: Media
estimacion: 3
roles: [usuario]
depende_de: [SIA-E5-01]
---

# SIA-E5-02 Detalle por tema y evolución

**Como** postulante, **quiero** abrir un área y ver mis temas y cómo evoluciona mi dominio, **para** enfocarme en el tema concreto que fallo.

## Reglas de negocio
1. Detalle de área: lista de temas con dominio, nivel, confianza y nº de respuestas; temas sin respuestas se muestran como «sin datos».
2. Evolución: serie semanal del dominio del área de las últimas 12 semanas (máx. 12 puntos); semanas sin actividad conservan el valor anterior marcado como tal.
3. Muestra las 10 últimas preguntas falladas del área con enlace al cuaderno de errores (SIA-E9-02) cuando exista.
4. Datos solo del dueño.

## Criterios de aceptación
```gherkin
Escenario: Detalle de área
  Dado un área con 5 temas
  Cuando el usuario la abre
  Entonces ve 5 temas con dominio, nivel y confianza

Escenario: Tema sin datos
  Dado un tema sin respuestas
  Cuando el usuario abre el detalle del área
  Entonces aparece «sin datos» y no un 50 % engañoso

Escenario: Evolución
  Dado 12 semanas de historial
  Cuando el usuario abre el detalle del área
  Entonces ve una serie de 12 puntos y su descripción textual accesible

Escenario: Historial corto
  Dado 2 semanas de historial
  Cuando el usuario abre el detalle del área
  Entonces ve 2 puntos y no se rellena el resto

Escenario: Área ajena
  Dado un área que no pertenece a su profesión
  Cuando el usuario pide ese detalle
  Entonces recibe 404

Escenario: Estados de UI
  Dado el detalle de un área
  Cuando la petición está en curso, no tiene datos o falla
  Entonces hay carga, vacío y error con reintento
```

## Fuera de alcance
Comparación con otros usuarios o percentiles.

## Dependencias
SIA-E5-01.

## Seguridad y privacidad
Autorización por dueño.

## i18n
Textos y descripción del gráfico en es/en.

## Definición de terminado
`tsc`, `lint`, `build`; gráfico con alternativa textual; review limpio.
