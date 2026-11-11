---
id: SIA-E2-01
titulo: Spike - verificar la API de NotebookLM
epica: E2
fase: MVP
prioridad: Media
estimacion: 2
roles: [admin]
depende_de: []
---

# SIA-E2-01 Spike: verificar la API de NotebookLM

**Como** equipo de producto, **quiero** saber con evidencia qué ofrece hoy la API pública de NotebookLM, **para** decidir si vale la pena un adaptador (SIA-E2-08) sin bloquear el resto del módulo.

## Contexto
Habilitadora. Ninguna otra historia depende de ella salvo E2-08. Time-box: 4 horas. No se crean cuentas ni recursos en servicios externos ni se llama a APIs de pago sin aprobación del usuario; solo se lee documentación pública.

## Reglas de negocio
1. Se consulta solo documentación oficial de Google (enlaces citados).
2. Se documenta por producto/plan (consumer vs NotebookLM Enterprise en Google Cloud): crear notebooks, añadir fuentes, consultar con citas, generar contenido, autenticación, cuotas, precio, residencia de datos y términos sobre uso de contenido.
3. Cada afirmación lleva enlace y fecha de consulta; lo no verificado se marca «no verificado».
4. El resultado termina en una recomendación: viable, limitada o no viable.

## Criterios de aceptación
```gherkin
Escenario: Informe completo
  Dado el time-box de 4 horas
  Cuando termina el spike
  Entonces existe docs/design/notebooklm-spike.md con una tabla de capacidades (crear, añadir fuentes, consultar, generar) por producto/plan, cada fila con enlace oficial y fecha

Escenario: Recomendación
  Dado el informe del spike terminado
  Cuando el po y el tech-lead lo revisan
  Entonces el informe concluye viable, limitada o no viable y lista las condiciones (plan, costo, cuenta requerida)

Escenario: Sin acciones externas
  Dado el spike en ejecución
  Cuando se revisa el historial de acciones del spike
  Entonces no se crearon cuentas, proyectos ni llamadas de pago, y lo que hacía falta quedó como pregunta al usuario

Escenario: Efecto en el resto del módulo
  Dado el informe del spike terminado
  Cuando el po revisa las dependencias de las historias
  Entonces el informe confirma que ninguna historia fuera de SIA-E2-08 requiere NotebookLM
```

## Fuera de alcance
Implementar el adaptador.

## Dependencias
Ninguna.

## Seguridad y privacidad
No se envían datos de usuarios ni documentos reales a ningún servicio.

## i18n
No aplica (documento interno).

## Definición de terminado
Informe revisado por tech-lead y po; decisión registrada en el README de la épica.
