---
id: SIA-E4-02
titulo: Resultado del diagnóstico
epica: E4
fase: MVP
prioridad: Alta
estimacion: 3
roles: [usuario]
depende_de: [SIA-E4-01, SIA-E4-03]
---

# SIA-E4-02 Resultado del diagnóstico

**Como** postulante, **quiero** ver mi nivel por área al terminar el diagnóstico y revisar mis respuestas, **para** entender mis puntos débiles y empezar a estudiar.

## Reglas de negocio
1. Por área: aciertos/total, nivel (`bajo` < 50 %, `medio` 50–74 %, `alto` ≥ 75 %) y etiqueta de confianza `baja` (menos de 5 respuestas efectivas del área, incluye diagnósticos previos) o `normal`.
2. Se muestra el nivel como orientativo, nunca como nota del examen.
3. Revisión: cada pregunta con tu respuesta, la clave y la explicación aprobada si existe (SIA-E6-08); si no existe, solo la clave oficial.
4. Áreas no medidas se listan aparte.
5. El resultado solo lo ve su dueño; se conserva en el historial.
6. Botón «Ir a mi dashboard» (SIA-E5-01).

## Criterios de aceptación
```gherkin
Escenario: Resultado por área
  Dado un diagnóstico finalizado con 3 de 4 aciertos en Salud pública
  Cuando el usuario abre el resultado
  Entonces la pantalla muestra "3/4", nivel "alto" y confianza "baja"

Escenario: Orientativo
  Dado un diagnóstico finalizado
  Cuando el usuario abre el resultado
  Entonces se muestra el aviso «Resultado orientativo, no equivale a tu nota en el examen» (es/en)

Escenario: Revisión
  Dado un diagnóstico finalizado con una explicación aprobada en una pregunta
  Cuando el usuario abre la revisión
  Entonces ve cada pregunta con su respuesta, la clave y la explicación aprobada si existe

Escenario: Área no medida
  Dado un diagnóstico con un área omitida por falta de preguntas
  Cuando el usuario abre el resultado
  Entonces el área aparece en «No medidas»

Escenario: Intento en curso
  Dado un diagnóstico no finalizado
  Cuando el usuario pide su resultado
  Entonces recibe 409 "intento_en_curso"

Escenario: Aislamiento
  Dado el diagnóstico finalizado de otro usuario
  Cuando un usuario consulta ese resultado
  Entonces recibe 404

Escenario: Estados de UI
  Dado la pantalla de resultado
  Cuando la petición está en curso o falla
  Entonces hay un estado de carga y un estado de error con reintento
```

## Fuera de alcance
Nota estimada con intervalo (SIA-E9-04).

## Dependencias
SIA-E4-01, SIA-E4-03.

## Seguridad y privacidad
Autorización por dueño del intento.

## i18n
Todos los textos y niveles en es/en.

## Definición de terminado
`tsc`, `lint`, `build`; tests de umbrales (49,9 %, 50 %, 74,9 %, 75 %); review limpio.
