---
id: SIA-E2-08
titulo: Adaptador NotebookLM
epica: E2
fase: v3
prioridad: Baja
estimacion: 5
roles: [admin]
depende_de: [SIA-E2-01, SIA-E2-02]
---

# SIA-E2-08 Adaptador NotebookLM

**Como** admin, **quiero** sincronizar mis fuentes con NotebookLM mediante un adaptador del puerto `SourceRepository`, **para** aprovechar su recuperación de pasajes si el spike confirma que es viable.

## Contexto
Condicionada al resultado de SIA-E2-01. Si el spike concluye «no viable», esta historia se descarta. El adaptador propio sigue siendo la fuente de verdad de las citas.

## Reglas de negocio
1. El adaptador es activable por configuración; desactivado por defecto.
2. Solo se envían fuentes académicas aprobadas; nunca datos de usuarios.
3. Las citas siempre se resuelven contra pasajes propios; una respuesta de NotebookLM sin pasaje propio equivalente se descarta.
4. Si NotebookLM falla o supera 10 s, el sistema recurre al adaptador propio sin error visible al usuario.
5. Credenciales solo por variables de entorno.

## Criterios de aceptación
```gherkin
Escenario: Activación
  Dado el adaptador desactivado
  Cuando el admin lo activa en la configuración
  Entonces las búsquedas de pasajes se consultan también en NotebookLM y se registran en audit_log

Escenario: Contrato
  Dado un doble de prueba de NotebookLM
  Cuando se ejecuta la suite de contrato del puerto
  Entonces el adaptador NotebookLM pasa los mismos casos que el propio

Escenario: Falla externa
  Dado que NotebookLM responde error o tarda más de 10 s
  Cuando se busca un pasaje
  Entonces la búsqueda usa el adaptador propio y se registra el evento sin datos personales

Escenario: Cita sin pasaje propio
  Dado un resultado de NotebookLM sin pasaje equivalente en el repositorio propio
  Cuando el servicio intenta usarlo como cita
  Entonces se descarta y no puede usarse como cita
```

## Fuera de alcance
Generar contenido dentro de NotebookLM.

## Dependencias
SIA-E2-01 con resultado «viable» o «limitada»; aprobación del usuario para crear cuentas o recursos externos y para costos.

## Seguridad y privacidad
Sin datos personales hacia el servicio externo; revisión de términos de uso de contenido (resultado del spike).

## i18n
Etiqueta de configuración es/en.

## Definición de terminado
Suite de contrato en verde con doble de prueba; sin llamadas reales sin aprobación; review limpio.
