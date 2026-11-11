---
id: PLT-02
titulo: Pantalla «Salud de servicios» en el panel admin
epica: PLT-E1
fase: MVP
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: [PLT-01]
---

# PLT-02 Pantalla «Salud de servicios» en el panel admin

**Como** admin, **quiero** una pantalla del panel que muestre el estado de cada servicio y la hora de la última comprobación, **para** ver de un vistazo si algo falla.

## Contexto
Pantalla nueva en el área de admin de `(dashboard)`, con entrada en la navegación de admin (`navegacion.ts`). Se construye con componentes y tokens de khipu. Consume el recurso de PLT-01.

## Reglas de negocio
1. Encabezado con el **estado global**, el texto «Última comprobación: <fecha y hora en America/Lima>» y el botón «Actualizar».
2. Una tarjeta por servicio conocido (en esta historia: API y base de datos), con nombre traducido, estado, categoría de fallo traducida si la hay y su dato (latencia en ms, versión y tiempo en marcha). Los servicios con identificador desconocido no se muestran y no rompen la pantalla.
3. El estado se comunica con **icono + texto + color** (tokens de khipu), nunca solo con color. Se usa el componente de estado o insignia de khipu. Si khipu no tiene uno adecuado, primero se abre un PR en khipu-design-system (regla del repo) y esta historia depende de él.
4. **Refresco automático cada 30 s** mientras la pestaña está visible. Se pausa en segundo plano y, al volver, refresca de inmediato. El botón «Actualizar» pide el estado al momento y se deshabilita mientras la petición está en curso.
5. Si la consulta falla (backend caído, red o 5xx), la pantalla muestra el aviso «No se pudo consultar el estado de los servicios». Si había datos, los conserva marcados como «desactualizados», con la hora de su última comprobación válida. Sigue reintentando en el siguiente ciclo. Un 503 (la base de datos está caída y la autenticación no puede verificar, PLT-01 regla 9) se trata igual: **no** cierra la sesión ni redirige al login.
6. Estados de UI: carga inicial, datos, error sin datos previos y error con datos previos (regla 5).
7. Texto fijo de ayuda: «Este panel muestra el estado al abrirlo. No envía alertas.»

## Criterios de aceptación
```gherkin
Escenario: Ver estado
  Dado la API y la base de datos operativas
  Cuando un admin abre «Salud de servicios»
  Entonces ve el estado global «Operativo», dos tarjetas «Operativo» con icono y texto, la latencia de la base de datos, la versión del backend y la última comprobación en hora de Lima

Escenario: Servicio caído
  Dado la base de datos caída
  Cuando un admin abre la pantalla
  Entonces ve el estado global «Caído» y la tarjeta de la base de datos «Caído — Sin conexión», sin ningún detalle técnico

Escenario: Refresco automático
  Dado la pantalla abierta y visible
  Cuando pasan 30 s
  Entonces se vuelve a consultar el estado y se actualiza la última comprobación sin recargar la página

Escenario: Pestaña en segundo plano
  Dado la pantalla abierta en una pestaña oculta
  Cuando pasan 2 minutos
  Entonces no se hace ninguna consulta, y al volver a la pestaña se consulta de inmediato

Escenario: Actualizar a mano
  Dado la pantalla abierta
  Cuando pulsa «Actualizar»
  Entonces el botón queda deshabilitado mientras se consulta y luego muestra el nuevo estado

Escenario: Backend inalcanzable con datos previos
  Dado la pantalla mostró datos a las 10:00
  Cuando la siguiente consulta falla
  Entonces ve «No se pudo consultar el estado de los servicios» y los datos de las 10:00 marcados como desactualizados

Escenario: Base de datos totalmente caída
  Dado la pantalla abierta y la API responde 503 porque la base de datos está caída
  Cuando se refresca el estado
  Entonces ve «No se pudo consultar el estado de los servicios», sigue en la pantalla con su sesión y no es redirigido al login

Escenario: Backend inalcanzable al abrir
  Dado el backend no responde
  Cuando un admin abre la pantalla
  Entonces ve el aviso de error y ninguna tarjeta con estado inventado

Escenario: Idioma
  Dado el idioma en inglés
  Cuando abre la pantalla
  Entonces título, estados, categorías, nombres de servicio y ayuda se ven en inglés

Escenario: Acceso
  Dado un usuario con rol usuario
  Cuando abre la ruta de salud de servicios
  Entonces la ruta lo redirige, no ve la entrada en la navegación y la API responde 403
```

## Fuera de alcance
Correo y versión del frontend (PLT-03), worker (PLT-04), copia (PLT-05), alertas, histórico, gráficos, umbrales editables y acciones correctivas.

## Dependencias
PLT-01. Componente de estado de khipu (PR previo a khipu si no existe).

## Seguridad y privacidad
Guard de rol admin en el layout y entrada de navegación visible solo para admin. La pantalla muestra solo lo que da la API: nada de URLs internas, hosts ni mensajes de error crudos.

## i18n
Todo en es/en, bajo `dashboard.saludServicios.*`: título, entrada de navegación, estado global, los 5 estados, las 5 categorías de fallo, los nombres de servicio, «Última comprobación», «Actualizar», «desactualizado», aviso de error y texto de ayuda. Cada clave existe en ambos archivos.

## Definición de terminado
`tsc`, `lint` y `build` en verde. Estados de UI verificados (carga, datos, error con y sin datos previos). Refresco y pausa en segundo plano verificados. Claves es/en. Accesibilidad: el estado no depende solo del color. Review limpio.
