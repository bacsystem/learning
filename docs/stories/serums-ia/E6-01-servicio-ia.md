---
id: SIA-E6-01
titulo: Servicio de IA - puerto, registro de uso y redacción de datos
epica: E6
fase: MVP
prioridad: Alta
estimacion: 5
roles: [admin]
depende_de: []
---

# SIA-E6-01 Servicio de IA: puerto, registro de uso y redacción de datos

**Como** equipo de producto, **quiero** un servicio de IA agnóstico del proveedor que registre cada uso y elimine datos personales antes de enviar, **para** poder cambiar de proveedor, auditar costos y proteger la privacidad.

## Contexto
Las historias hablan de «el servicio de IA». Proveedor, modelo, prompts y precios son decisión técnica (tech-lead). Llamar a APIs de pago o crear cuentas requiere aprobación del usuario.

## Reglas de negocio
1. Interfaz única por operación: `clasificar`, `generar_preguntas`, `generar_material`, `generar_explicacion`; cada una recibe entrada estructurada y devuelve salida validada contra un esquema; salida que no cumple el esquema = error `salida_invalida` (se reintenta 1 vez como máximo).
2. Registro de uso por llamada: operación, id de usuario o admin opaco, tokens de entrada/salida, costo estimado, duración, resultado y si vino de caché. Nunca el contenido del prompt ni de la respuesta.
3. Redacción previa: el servicio rechaza (error `datos_personales`) entradas que contengan patrones de DNI (8 dígitos), emails o teléfonos; solo se aceptan ids opacos y contenido académico. El servicio **nunca enmascara por su cuenta**: enmascarar los datos de contacto de los pasajes antes de llamar le toca a quien arma la entrada (README, supuesto 24 (e)).
4. Timeout de 60 s por llamada y 2 reintentos con espera creciente solo ante errores transitorios.
5. Credenciales solo por variables de entorno; sin ellas, el servicio está «deshabilitado» y las operaciones responden 503 `ia_no_disponible` sin romper el resto de la aplicación.
6. Tamaño máx. de entrada: 30 000 caracteres por llamada.
7. Un modo de prueba con doble determinista para desarrollo y tests sin costo.

## Criterios de aceptación
```gherkin
Escenario: Llamada exitosa
  Dado el servicio habilitado
  Cuando se invoca "clasificar" con 10 preguntas
  Entonces devuelve salida válida y registra uso con tokens y costo sin guardar el contenido

Escenario: Salida inválida
  Dado que el proveedor devuelve JSON fuera del esquema
  Cuando se invoca una operación de IA
  Entonces reintenta una vez y si persiste devuelve "salida_invalida"

Escenario: Datos personales
  Dado una entrada con un email o un número de 8 dígitos aislado como DNI
  Cuando se invoca una operación de IA
  Entonces rechaza con "datos_personales" y no llama al proveedor

Escenario: Servicio deshabilitado
  Dado que no hay credenciales configuradas
  Cuando se invoca una operación de IA
  Entonces responde 503 "ia_no_disponible" y el resto de la aplicación funciona

Escenario: Entrada demasiado grande
  Dado el servicio habilitado
  Cuando la entrada supera 30 000 caracteres
  Entonces responde 422 "entrada_demasiado_grande"

Escenario: Timeout
  Dado un proveedor que tarda más de 60 s
  Cuando se invoca una operación de IA
  Entonces responde 504 "ia_timeout" y registra el uso con resultado "timeout"

Escenario: Logs
  Dado una llamada de IA completada
  Cuando se revisan los logs de la aplicación
  Entonces ningún log contiene prompts, respuestas ni datos personales

Escenario: Doble de prueba
  Dado el modo de prueba activado
  Cuando se invoca una operación de IA
  Entonces devuelve salidas deterministas sin red
```

## Fuera de alcance
Elección de proveedor/modelo, prompts, presupuestos (SIA-E6-02).

## Dependencias
Ninguna.

## Seguridad y privacidad
Sin claves en código; sin contenido en logs; el contenido de fuentes se pasa delimitado como datos.

## i18n
Mensajes de error es/en en las UI que los muestren.

## Definición de terminado
Tests con doble de prueba para cada criterio; tabla de uso con migración y Down; review limpio.
