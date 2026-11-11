---
id: SIA-E4-01
titulo: Test diagnóstico inicial por áreas
epica: E4
fase: MVP
prioridad: Alta
estimacion: 5
roles: [usuario]
depende_de: [SIA-E3-05, SIA-E1-04]
---

# SIA-E4-01 Test diagnóstico inicial por áreas

**Como** postulante, **quiero** rendir un test corto que mida mi nivel en cada área, **para** saber dónde estoy débil sin hacer un simulacro completo.

## Contexto
El diagnóstico es un intento cronometrado propio, distinto de práctica y simulacro, con preguntas aprobadas de la profesión elegida. Exige inscripción aprobada.

```mermaid
sequenceDiagram
  actor U as Postulante
  participant API as API
  participant S as Selector de preguntas
  participant D as Modelo de dominio
  U->>API: POST iniciar diagnóstico
  API->>API: verifica inscripción aprobada y profesión
  API->>S: pide 4 preguntas aprobadas por área, máximo 40
  S-->>API: preguntas con orden de alternativas estable
  API-->>U: 201 intento con fin de plazo en servidor
  loop cada pregunta
    U->>API: PUT respuesta
    API->>API: verifica plazo y responde 409 si venció
  end
  U->>API: POST finalizar
  API->>D: registra respuestas por área y tema
  API-->>U: resultado del diagnóstico
```

## Reglas de negocio
1. Exige inscripción aprobada al proceso y profesión elegida (403 si no).
2. 4 preguntas por área (mín. 3 si el área tiene pocas), máx. 40 en total; solo preguntas aprobadas, tipo única prioritaria; sin repetir preguntas ya respondidas por el usuario cuando haya alternativas.
3. Selección con dificultad mixta: 1 fácil, 2 medias, 1 difícil por área cuando existan.
4. Tiempo límite 45 min controlado en servidor; pasado el tiempo responde 409 `tiempo_agotado` y el intento se finaliza con lo respondido.
5. Un solo diagnóstico en curso por usuario; se permite repetir el diagnóstico una vez cada 30 días.
6. Si un área tiene menos de 3 preguntas aprobadas, se omite y se informa que no fue medida.
7. La clave y la explicación no se muestran hasta finalizar.
8. El diagnóstico está disponible aunque falten los pesos de la profesión: si la configuración de SIA-E1-03 no está `completa`, el intento y su resultado (SIA-E4-02) llevan `pesos_estimados: true` y la pantalla muestra «Pesos estimados: aún no tenemos los pesos oficiales de tu profesión» (decisión P6, issue #65).
9. Cooldown antifraude (decisión P8, issue #65): la fecha del último diagnóstico finalizado se registra también bajo `HMAC(usuario_id)` (secreto en variable de entorno), sin otros datos personales. Borrar los datos de aprendizaje (SIA-E9-07) no reinicia la espera de 30 días. El registro se elimina al cumplirse los 30 días.

## Criterios de aceptación
```gherkin
Escenario: Inicio
  Dado un usuario con inscripción aprobada y profesión Medicina con 6 áreas con banco suficiente
  Cuando inicia el diagnóstico
  Entonces recibe 24 preguntas aprobadas (4 por área) y el instante de fin a 45 min

Escenario: Sin inscripción
  Dado un usuario con inscripción pendiente
  Cuando inicia el diagnóstico
  Entonces recibe 403

Escenario: Área sin banco
  Dado un área con 2 preguntas aprobadas
  Cuando el usuario inicia el diagnóstico
  Entonces el área no se incluye y el resultado indica «no medida»

Escenario: Tiempo agotado
  Dado un diagnóstico en curso con el plazo de 45 min vencido
  Cuando el usuario responde una pregunta
  Entonces recibe 409 "tiempo_agotado" y el intento queda finalizado

Escenario: Ya en curso
  Dado un diagnóstico en curso
  Cuando inicia otro
  Entonces recibe 409 "diagnostico_en_curso" con el id del vigente

Escenario: Repetición temprana
  Dado un diagnóstico finalizado hace 10 días
  Cuando inicia otro
  Entonces recibe 409 "repeticion_no_permitida" con la fecha en que podrá

Escenario: Cooldown tras borrado de datos
  Dado un diagnóstico finalizado hace 10 días y el usuario borró después sus datos de aprendizaje
  Cuando inicia otro diagnóstico
  Entonces recibe 409 "repeticion_no_permitida" con la fecha en que podrá

Escenario: Registro de cooldown sin datos personales
  Dado el registro de cooldown de diagnóstico guardado
  Cuando se inspecciona
  Entonces solo contiene el HMAC del usuario y la fecha del último diagnóstico, sin id en claro, nombre, DNI ni email

Escenario: Pesos estimados
  Dado una profesión cuya configuración de pesos está "incompleta"
  Cuando el usuario inicia el diagnóstico
  Entonces el intento se crea con "pesos_estimados": true y la pantalla muestra el aviso de pesos estimados

Escenario: Aislamiento
  Dado el diagnóstico de otro usuario
  Cuando un usuario consulta ese intento
  Entonces recibe 404

Escenario: Borradores excluidos
  Dado preguntas en borrador o en_revision en las áreas de la profesión
  Cuando el usuario inicia el diagnóstico
  Entonces ninguna pregunta en borrador o en_revision se incluye
```

## Fuera de alcance
Diagnóstico adaptativo (selección dinámica según aciertos); resultado y dominio (SIA-E4-02, E4-03).

## Dependencias
SIA-E3-05 (banco aprobado), SIA-E1-04, SIA-E1-03 (servicio de pesos con `pesos_estimados`, transitiva vía SIA-E1-04).

## Seguridad y privacidad
Orden de alternativas estable por intento; la clave no viaja al cliente antes de finalizar. Logs solo con ids. El secreto del HMAC vive solo en variables de entorno.

## i18n
Pantalla, instrucciones y errores en es/en; estados de carga/error.

## Definición de terminado
Tests de plazo en servidor, inscripción, selección y aislamiento; `tsc`, `lint`, `build` en verde; review limpio.
