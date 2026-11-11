---
id: PLT-01
titulo: Estado de salud agregado — API y base de datos
epica: PLT-E1
fase: MVP
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: []
---

# PLT-01 Estado de salud agregado — API y base de datos

**Como** admin, **quiero** que la API me diga en una sola respuesta si ella y la base de datos están bien y cuándo se comprobó, **para** saber si la plataforma funciona sin entrar a Railway.

## Contexto
Hoy existe `GET /health` público, que devuelve la versión del backend. Esta historia crea un recurso nuevo, **solo para admin**, con el estado detallado por servicio. El architect define la ruta y el contrato en `docs/api/`. El `/health` público no cambia. Decisiones de estados, umbrales, caché y categorías: ver [README](README.md#decisiones-del-po-aplican-a-toda-la-épica).

## Reglas de negocio
1. La respuesta trae el **estado global**, `comprobado_en` (instante UTC; la UI lo muestra en `America/Lima`) y una lista de servicios. Cada servicio trae: identificador estable (`api`, `base_datos`), estado (`operativo`, `degradado`, `caido`, `no_configurado`, `desconocido`), categoría de fallo si la hay y, según el servicio, un dato numérico no sensible.
2. **API:** si la respuesta llega, está `operativo`. Incluye la versión del backend (la misma que `/health`) y el tiempo en marcha del proceso.
3. **Base de datos:** se ejecuta una consulta trivial de solo lectura y se mide su latencia en ms. Con > 500 ms, `degradado`. Con error o sin respuesta en 2 s, `caido` (categoría `sin_conexion` o `tiempo_agotado`).
4. Las comprobaciones corren en paralelo, cada una con un máximo de 2 s. Una comprobación que no termina se informa como `caido` / `tiempo_agotado` y no bloquea la respuesta.
5. El estado global es el peor de los individuales: `caido` > `degradado` = `desconocido` > `operativo`; `no_configurado` no cuenta.
6. El resultado se guarda en caché 15 s. Dentro de ese plazo, cualquier consulta recibe el mismo resultado y el mismo `comprobado_en`.
7. El diseño debe permitir añadir servicios (PLT-03..05) sin cambiar el contrato: la lista de servicios es abierta y la UI ignora los identificadores que no conoce.
8. Consultar no escribe en `audit_log`: es una lectura sin efectos (decisión 9 del README).
9. **Nota para el architect:** si validar la sesión o el rol del admin consulta la base de datos, con la base caída la API no podrá responder 200 con `base_datos: caido`, porque fallará antes. Ese caso es válido y lo cubre el escenario «Backend inalcanzable» de PLT-02. Los escenarios «Base de datos caída» y «Comprobación que no termina» se prueban con la base de la sonda aislada (doble de prueba o pool aparte), o se ajustan según cómo se resuelva la autenticación.

## Criterios de aceptación
```gherkin
Escenario: Todo operativo
  Dado la base de datos responde en 20 ms
  Cuando un admin consulta el estado de salud
  Entonces recibe 200 con estado global "operativo", "api" y "base_datos" en "operativo", la latencia de la base de datos, la versión del backend y "comprobado_en"

Escenario: Base de datos lenta
  Dado la consulta de prueba tarda 800 ms
  Cuando un admin consulta el estado de salud
  Entonces "base_datos" está "degradado" y el estado global es "degradado"

Escenario: Base de datos caída
  Dado la base de datos no acepta conexiones
  Cuando un admin consulta el estado de salud
  Entonces recibe 200 con "base_datos" en "caido", categoría "sin_conexion", y el estado global es "caido"

Escenario: Comprobación que no termina
  Dado la consulta de prueba no responde
  Cuando un admin consulta el estado de salud
  Entonces la respuesta llega en menos de 3 s con "base_datos" en "caido" y categoría "tiempo_agotado"

Escenario: Caché de 15 s
  Dado un admin consultó el estado hace 5 s
  Cuando vuelve a consultar
  Entonces recibe el mismo "comprobado_en" y no se ejecuta una nueva consulta de prueba

Escenario: Sin secretos ni datos internos
  Dado la base de datos falla con un error que incluye host, usuario y cadena de conexión
  Cuando un admin consulta el estado de salud
  Entonces la respuesta no contiene host, IP, puerto, URL, usuario, contraseña, cadena de conexión ni el mensaje del error; solo la categoría

Escenario: Health público sin cambios
  Dado cualquier visitante sin sesión
  Cuando llama a GET /health
  Entonces recibe la misma respuesta de hoy (versión) sin detalle de servicios

Escenario: Acceso de un usuario sin rol admin
  Dado un usuario con rol usuario
  Cuando consulta el estado de salud detallado
  Entonces recibe 403

Escenario: Acceso sin sesión
  Dado una petición sin sesión
  Cuando consulta el estado de salud detallado
  Entonces recibe 401
```

## Fuera de alcance
Pantalla (PLT-02), correo y frontend (PLT-03), worker (PLT-04), copia (PLT-05), alertas, histórico y uptime.

## Dependencias
Ninguna. El architect define el recurso en `docs/api/` antes de implementar.

## Seguridad y privacidad
Solo rol `admin` (401 / 403 con `jsonError`). Sin host, IP, puerto, URL interna, usuario, credenciales, cadena de conexión ni mensaje de error crudo en la respuesta. Los logs del servidor pueden registrar el error técnico, pero sin credenciales ni datos personales. Sin datos personales en la respuesta. El `/health` público no gana detalle.

## i18n
Sin texto visible en esta historia: la API devuelve identificadores y categorías, y la UI los traduce (PLT-02).

## Definición de terminado
Tests de API: 401 / 403, todo operativo, degradado por latencia, caído por conexión, tiempo agotado < 3 s, caché de 15 s, respuesta sin secretos ante un error que los contiene, y `/health` público intacto. Contrato en `docs/api/`. Review limpio.
