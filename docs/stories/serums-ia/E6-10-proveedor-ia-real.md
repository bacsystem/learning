---
id: SIA-E6-10
titulo: Adaptador del proveedor real de IA
epica: E6
fase: MVP
prioridad: Alta
estimacion: 3
roles: [admin]
depende_de: [SIA-E6-01, SIA-E6-02]
---

# SIA-E6-10 Adaptador del proveedor real de IA

**Como** admin, **quiero** que el servicio de IA use un proveedor real aprobado por el usuario, **para** clasificar y generar contenido en producción con un costo conocido y dentro del presupuesto.

## Contexto
SIA-E6-01 entregó el puerto, el registro de uso y un doble determinista. SIA-E6-02 aplica los límites, el presupuesto y la caché, y se prueba con el doble. Falta conectar un proveedor real: sin él, SIA-E3-04, E6-03, E6-06 y E6-08 no funcionan en producción. Decisión del PO del 12-nov, a pedido del architect: el contrato de E6-01 asignaba esto a E6-02, y se separa aquí para no frenarla.

## Reglas de negocio
1. **Elección con aprobación del usuario:** el tech-lead propone el proveedor y el modelo con una estimación del costo mensual. La estimación toma el uso del MVP (clasificar un balotario de 580 preguntas, generar 20 preguntas por solicitud) y lo compara con el presupuesto de 40 USD/mes. Nada se implementa ni se configura sin la aprobación escrita del usuario en el issue.
2. El adaptador implementa el puerto de SIA-E6-01 sin cambiarlo y devuelve el uso (tokens y costo en USD) que SIA-E6-02 registra.
3. La configuración va por variables de entorno (`IA_PROVEEDOR`, `IA_API_KEY`, `IA_MODELO`). Si `IA_PROVEEDOR` está vacío, la IA queda deshabilitada y el resto de la aplicación funciona (SIA-E6-01).
4. Los errores del proveedor (timeout, límite de tasa, respuesta inválida) se traducen a los errores del puerto. Una llamada fallida sigue la regla 6 de SIA-E6-02.
5. Solo se envían datos ya validados y enmascarados por SIA-E6-01 (supuesto 24 (e)). Nunca se envían datos de usuarios.
6. La clave del proveedor nunca aparece en logs, respuestas ni en el repositorio.

## Criterios de aceptación
```gherkin
Escenario: Proveedor aprobado
  Dado la propuesta de proveedor y modelo con su costo estimado
  Cuando el usuario la aprueba en el issue
  Entonces se implementa ese proveedor y no otro

Escenario: Llamada real registrada
  Dado el proveedor configurado en preproducción
  Cuando un admin clasifica un lote de 20 preguntas sintéticas
  Entonces la llamada se registra con tokens y costo en USD mayor que 0, y el gasto del mes sube en ese costo

Escenario: IA deshabilitada
  Dado IA_PROVEEDOR vacío
  Cuando un admin pide una clasificación
  Entonces recibe el error de IA no disponible y las funciones sin IA siguen operando

Escenario: Error del proveedor
  Dado un proveedor que responde con timeout
  Cuando se procesa una llamada
  Entonces el admin recibe el error traducido y no se le descuenta la acción

Escenario: Clave protegida
  Dado el adaptador en ejecución
  Cuando se revisan los logs y las respuestas de la API
  Entonces la clave del proveedor no aparece
```

## Fuera de alcance
Usar varios proveedores a la vez, conmutar automáticamente si uno falla, ajustar prompts por profesión y la búsqueda semántica (v3).

## Dependencias
SIA-E6-01, SIA-E6-02. Aprobación del usuario del proveedor y del modelo (bloqueante).

## Seguridad y privacidad
Secretos solo en variables de entorno de cada entorno (devops). El proveedor se evalúa también por sus términos de uso y de retención de datos, y esos términos van en la propuesta al usuario.

## i18n
Sin texto visible nuevo. El error de IA no disponible y los errores del proveedor se muestran traducidos en es/en, nunca con el código crudo. Si alguna clave falta, se agrega en ambos archivos.

## Definición de terminado
Propuesta aprobada por el usuario en el issue. Tests del adaptador contra un servidor simulado (éxito, timeout, límite de tasa, respuesta inválida). Variables documentadas en `docs/ops` por devops. Una prueba en preproducción con datos sintéticos (nunca de `assets/`). Review limpio.
