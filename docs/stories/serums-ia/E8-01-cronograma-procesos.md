---
id: SIA-E8-01
titulo: Cronograma de procesos (admin)
epica: E8
fase: MVP
prioridad: Alta
estimacion: 3
roles: [admin, usuario]
depende_de: [SIA-E1-03]
---

# SIA-E8-01 Cronograma de procesos (admin)

**Como** admin, **quiero** cargar los hitos de cada proceso SERUMS (inscripción, examen, resultados), **para** que los postulantes vean fechas oficiales y reciban alertas.

## Contexto
Los hitos se configuran a mano; la plataforma no extrae fechas de sitios externos.

## Reglas de negocio
1. Hito: proceso, tipo (`inscripcion_inicio`, `inscripcion_fin`, `examen`, `resultados`, `otro`), título (3–120), fecha y hora (America/Lima) y enlace oficial opcional (HTTPS, ≤300 caracteres).
2. Máx. 30 hitos por proceso.
3. Al cambiar la fecha de un hito se encolan las alertas afectadas (SIA-E8-02) y se audita (antes/después).
4. Un hito de tipo `examen` puede ser la fecha por defecto en SIA-E7-01.
5. Los postulantes con inscripción aprobada en el proceso ven el cronograma en modo lectura.
6. Eliminar un hito es lógico (queda en auditoría).

## Criterios de aceptación
```gherkin
Escenario: Crear hito
  Dado un admin
  Cuando crea el hito "Examen" el 2026-12-14 09:00
  Entonces recibe 201 y se registra en audit_log

Escenario: Enlace inválido
  Dado un admin en el formulario de hito
  Cuando pone una URL http
  Entonces recibe 422 "enlace_invalido"

Escenario: Límite
  Dado un proceso con 30 hitos
  Cuando el admin crea otro hito
  Entonces recibe 422 "limite_hitos"

Escenario: Cambio de fecha
  Dado un hito con alertas pendientes
  Cuando el admin cambia la fecha
  Entonces las alertas se recalculan y se registra antes/después

Escenario: Vista de postulante
  Dado un usuario con inscripción aprobada en el proceso
  Cuando abre el cronograma e intenta editar un hito
  Entonces ve los hitos en orden cronológico y recibe 403 al intentar editarlos

Escenario: Sin hitos
  Dado un proceso sin hitos publicados
  Cuando el postulante abre el cronograma
  Entonces ve «Aún no hay fechas publicadas»

Escenario: Usuario sin inscripción
  Dado un usuario sin inscripción aprobada en el proceso
  Cuando pide los hitos del proceso
  Entonces recibe 404 y no ve hitos de procesos en los que no está inscrito
```

## Fuera de alcance
Importar fechas desde sitios web; calendario por profesión.

## Dependencias
SIA-E1-03.

## Seguridad y privacidad
Solo admin escribe; auditoría completa.

## i18n
Tipos de hito y pantallas en es/en; fechas con el locale.

## Definición de terminado
Migración con Down; `tsc`, `lint`, `build`; tests de permisos y recálculo; review limpio.
