---
id: PLT-09
titulo: Pantallas de catálogo y selectores en el admin
epica: PLT-E2
fase: Diferida
estado: Diferida (alcance de la ronda: solo PLT-06, 11, 12, 13)
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: [PLT-07, PLT-08]
---

# PLT-09 Pantallas de catálogo y selectores en el admin

**Como** admin, **quiero** gestionar los niveles y tipos de cada institución y elegirlos en los formularios, **para** configurar una institución nueva sin ayuda técnica.

## Contexto
Usa los catálogos de PLT-07 y la regla de PLT-08. UI solo con componentes y tokens de khipu (si falta uno, PR a khipu primero).

## Reglas de negocio
1. Pantalla de catálogo con dos pestañas (Niveles, Tipos de proceso), filtrada por institución; crear, renombrar, reordenar, inactivar; marcar `exige_grado` en tipos.
2. Los formularios de grado, especialidad y proceso cargan sus opciones de nivel y tipo **de la institución elegida** y se recargan al cambiar de institución. Sin opciones fijas «suboficial/oficial» ni «ascenso/pase/admisión» en el frontend.
3. En el formulario de proceso, el campo grado aparece obligatorio cuando el tipo exige grado y solo lista grados de esa institución.
4. El formulario de proceso añade un selector de **nivel** opcional (vacío = «todos los niveles») con los niveles activos de la institución elegida (P5; el campo lo habilita PLT-07). Se limpia al cambiar de institución.
5. Los valores inactivos se ven en registros existentes pero no se ofrecen en altas.
6. Errores de la API (409, 422) con mensaje claro y traducido, por código: `codigo_duplicado`, `valor_en_uso`, `limite_catalogo`, `nivel_invalido`, `tipo_invalido`, `grado_requerido`, `grado_otra_institucion`, `referencia_invalida`.

## Criterios de aceptación
```gherkin
Escenario: Crear un nivel
  Dado el admin en la pantalla de catálogo de la institución B
  Cuando crea el nivel "Cadete"
  Entonces aparece en la lista y está disponible al crear un grado de B

Escenario: Selector depende de la institución
  Dado el formulario de proceso
  Cuando el admin cambia la institución de A a B
  Entonces los tipos y el grado se recargan con los de B y el valor previo se limpia

Escenario: Grado obligatorio
  Dado un tipo con exige_grado seleccionado
  Cuando el admin intenta guardar sin grado
  Entonces ve el error de campo requerido y no se envía

Escenario: Valor inactivo
  Dado el tipo "admision" inactivo
  Cuando el admin abre un proceso existente de ese tipo
  Entonces el tipo se muestra; y en un proceso nuevo no se ofrece

Escenario: Nivel del proceso
  Dado el formulario de proceso de la institución B
  Cuando el admin elige un nivel de B, o lo deja vacío, y guarda
  Entonces el proceso queda con ese nivel, o sin nivel («todos los niveles»)

Escenario: Idioma
  Dado el admin cambia el idioma a inglés
  Cuando recorre la pantalla de catálogo
  Entonces todos los textos propios de la app están en inglés
```

## Fuera de alcance
Importación masiva de catálogos; traducir los nombres que escribe el admin; reordenar con arrastrar y soltar (basta flechas o número).

## Seguridad y privacidad
Pantallas bajo el guard de rol admin; la API ya valida el rol (403 a usuarios).

## i18n
Todas las claves en `src/i18n/messages/{es,en}.json`, namespace `dashboard`.

## Definición de terminado
`tsc`, `lint`, `build` en limpio; capturas es/en en el PR; review limpio.
