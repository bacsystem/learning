---
id: SIA-E2-08
titulo: Adaptador de búsqueda semántica (pgvector o NotebookLM)
epica: E2
fase: v3
prioridad: Baja
estimacion: 5
roles: [admin]
depende_de: [SIA-E2-01, SIA-E2-02]
---

# SIA-E2-08 Adaptador de búsqueda semántica (pgvector o NotebookLM)

**Como** admin, **quiero** que la búsqueda de pasajes use recuperación semántica mediante un adaptador del puerto `SourceRepository`, **para** encontrar pasajes relevantes aunque no compartan las mismas palabras, si la reevaluación de v3 lo recomienda.

## Contexto
Condicionada al resultado de SIA-E2-01 (P1, issue #65). Si el spike recomienda «mantener solo adaptador propio», esta historia se descarta. Si recomienda pgvector o NotebookLM, se implementa solo esa opción. El adaptador propio en PostgreSQL sigue siendo la fuente de verdad de las citas. Hasta v3 el costo autorizado para este adaptador es $0 USD.

## Reglas de negocio
1. El adaptador es activable por configuración; desactivado por defecto.
2. Solo se indexan o envían fuentes académicas activas; nunca datos de usuarios.
3. Las citas siempre se resuelven contra pasajes propios; un resultado sin pasaje propio equivalente se descarta.
4. Si el adaptador falla o supera 10 s, el sistema recurre al adaptador propio sin error visible al usuario.
5. Credenciales (si las hay) solo por variables de entorno.
6. Si la opción elegida es NotebookLM, crear cuenta o proyecto y asumir costos requiere aprobación previa del usuario.

## Criterios de aceptación
```gherkin
Escenario: Activación
  Dado el adaptador desactivado
  Cuando el admin lo activa en la configuración
  Entonces las búsquedas de pasajes usan también el adaptador semántico y el cambio se registra en audit_log

Escenario: Contrato
  Dado un doble de prueba del servicio semántico
  Cuando se ejecuta la suite de contrato del puerto
  Entonces el adaptador semántico pasa los mismos casos que el propio

Escenario: Falla externa
  Dado que el adaptador semántico responde error o tarda más de 10 s
  Cuando se busca un pasaje
  Entonces la búsqueda usa el adaptador propio y se registra el evento sin datos personales

Escenario: Cita sin pasaje propio
  Dado un resultado del adaptador semántico sin pasaje equivalente en el repositorio propio
  Cuando el servicio intenta usarlo como cita
  Entonces se descarta y no puede usarse como cita
```

## Fuera de alcance
Generar contenido dentro de NotebookLM; implementar más de una opción.

## Dependencias
SIA-E2-01 con recomendación pgvector o NotebookLM; SIA-E2-02; aprobación del usuario para cuentas, recursos externos o costos.

## Seguridad y privacidad
Sin datos personales hacia servicios externos; revisión de términos de uso de contenido (resultado del spike).

## i18n
Etiqueta de configuración es/en.

## Definición de terminado
Suite de contrato en verde con doble de prueba; sin llamadas reales sin aprobación; review limpio.
