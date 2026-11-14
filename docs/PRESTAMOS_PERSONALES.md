# Préstamos personales: alcance vigente

Este documento reemplaza las decisiones de producto de `ESPECIFICACION.md` que exijan planes, rangos de tasas o métodos financieros configurables. El objetivo es prestar dinero propio a personas conocidas y registrar lo que devuelven.

## Registrar un préstamo

1. Elegir cliente en un único campo de búsqueda con autocompletado.
2. Indicar cuánto dinero se entrega.
3. Elegir sin interés, un monto fijo de interés total o un porcentaje total sobre el monto entregado.
4. Elegir la frecuencia: diaria, semanal, cada 15 días, mensual o un solo pago.
5. Indicar el número de pagos y la fecha del primero. Para un solo pago, elegir únicamente el vencimiento. Mantener la fecha de entrega separada.
6. Revisar el importe de cada pago, el total y las fechas del primer y último pago; confirmar y desembolsar.

El interés es **por todo el préstamo**, no por cuota ni por mes. Ejemplo: entregar 300, aplicar 20% total y registrar 5 pagos diarios desde el 15 de noviembre genera 5 cuotas de 72 y termina el 19 de noviembre. El plazo se deduce del número de pagos, su frecuencia y la primera fecha; no se pide una duración que pueda contradecir esos datos.

El contrato nuevo de simulación y creación usa `installments_count` (entero de 1 a 1000), `first_payment_date`, `frequency` y `disbursement_date`, además del cliente, principal e interés total. El primer pago no puede ser anterior a la entrega; `single` exige un solo pago. Los campos anteriores `term_value`/`term_unit` permanecen por compatibilidad con clientes antiguos y no se combinan con el nuevo contrato. La web ya no los envía. Cada fecha mensual conserva el día de anclaje del primer pago, limitado al último día del mes cuando corresponde.

Las frecuencias son opciones del sistema, no un maestro que el usuario deba mantener. El usuario no tiene que crear un plan, definir límites mínimos/máximos ni elegir cálculos francés/alemán. El backend calcula el número de cuotas, reparte los decimales sin alterar el total y valida montos, fechas y permisos.

## Capital necesario

La simulación no requiere capital. Registrar un préstamo sí exige que la caja de la misma empresa cubra el monto a entregar; el backend comprueba ese saldo en la transacción de creación. Si no alcanza, responde `409 INSUFFICIENT_CAPITAL`, informa el disponible y no registra préstamo, cuotas ni auditoría de creación. La opción histórica de permitir capital negativo no autoriza préstamos sin fondos.

El registro queda pendiente y no descuenta ni reserva dinero. Al entregar el dinero, el backend vuelve a comprobar la caja bajo el mismo bloqueo de capital usado por sus movimientos y descuenta el importe una sola vez. Así un retiro u otro desembolso posterior no permite entregar dinero inexistente. El formulario muestra la caja disponible y un acceso a Capital para registrar un aporte.

## Registrar devoluciones

Cada pago puede ser completo o parcial y tener una o varias partes: efectivo, Yape, Plin o transferencia. Las partes deben sumar exactamente el importe declarado. Se muestran el saldo, las cuotas pagadas/parciales/pendientes y el historial. El backend conserva la distribución entre capital, interés y mora, actualiza la caja en la misma transacción y evita duplicados mediante idempotencia.

## Mora opcional

La mora se configura una sola vez para la empresa y se aplica únicamente si el dueño decide usarla. Sin mora es una opción válida. Los días de gracia son un ajuste opcional, no un maestro ni un paso obligatorio para prestar. El sistema debe mostrar días de atraso con claridad.

Este alcance no implica que el devengo automático de mora ya esté implementado; debe verificarse antes de anunciarlo como disponible.

## Compatibilidad y datos

Los préstamos ya registrados conservan sus condiciones y siguen visibles y pagables. Las migraciones anteriores no se reescriben. Las tablas y referencias históricas de productos pueden conservarse por compatibilidad, pero el flujo nuevo no depende de ellas ni crea productos ocultos. No se borran pagos, movimientos de caja ni registros históricos para simplificar la interfaz.

## Interfaz y validaciones

Diseño móvil primero, formularios de registro en modales, textos breves en español y controles compartidos. El backend es la autoridad de las validaciones; el frontend conserva los datos ante fallos, muestra errores de campo y notifica el resultado con un toast visible sobre el modal.
