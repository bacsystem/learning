---
id: PLT-14
titulo: Institución visible al postulante y en la landing
epica: PLT-E2
fase: Diferida
estado: Diferida (alcance de la ronda: solo PLT-06, 11, 12, 13)
prioridad: Media
estimacion: 2
roles: [postulante, visitante]
depende_de: [PLT-06]
---

# PLT-14 Institución visible al postulante y en la landing

**Como** postulante, **quiero** ver a qué institución pertenezco, y **como** visitante, **quiero** ver los procesos agrupados por institución, **para** saber de quién es cada proceso.

## Contexto
El postulante no ve su institución en ningún sitio. `GET /procesos/publico` no devuelve la institución (`ProcesoDTO` no la trae), así que agrupar la landing requiere ampliarlo. Las instituciones son el contexto del postulante; los textos no sugieren vínculo con ellas (E2 decisión 10).

## Reglas de negocio
1. El postulante ve el nombre de su institución en configuración (solo lectura) y en el subtítulo de /procesos («procesos activos de {institución}»).
2. `GET /procesos/publico` añade la institución (id y nombre) a cada proceso; es un campo nuevo, no cambia los existentes. La landing agrupa los procesos por institución, sin botón de inscripción.
3. El nombre de institución no es dato personal.

## Criterios de aceptación
```gherkin
Escenario: Configuración
  Dado un postulante de la institución A
  Cuando abre su configuración
  Entonces ve el nombre de A en solo lectura, con la etiqueta en el idioma de la interfaz

Escenario: Subtítulo
  Dado un postulante de A
  Cuando abre /procesos en español o en inglés
  Entonces el subtítulo nombra a A y está traducido

Escenario: Landing agrupada
  Dado procesos activos de A y de B
  Cuando un visitante sin sesión abre la landing
  Entonces ve dos grupos, uno por institución, con sus procesos, sin opción de inscribirse

Escenario: Compatibilidad del endpoint público
  Dado un cliente que ya consume GET /procesos/publico
  Cuando recibe la respuesta nueva
  Entonces todos los campos anteriores siguen presentes y sin cambios
```

## Fuera de alcance
Logos o páginas por institución; filtros en la landing; cambiar la institución desde configuración.

## Seguridad y privacidad
El endpoint público no expone más que el nombre de la institución y los campos que ya devolvía.

## i18n
Etiqueta de configuración, subtítulo con parámetro y encabezados de grupo, en es/en.

## Definición de terminado
Test del endpoint público (campos nuevos y antiguos); `tsc`/`lint`/`build`; contrato en `docs/api/`. Review limpio.
