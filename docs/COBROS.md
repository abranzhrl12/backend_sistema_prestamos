# Cobros y acuerdos

`Cobros` reúne las cuotas con saldo de préstamos activos de la empresa autenticada. No incluye préstamos pendientes de entrega ni cuotas completamente pagadas. Solo propietario y administrador pueden consultar esta vista y registrar acuerdos. La autenticación y el aislamiento RLS se mantienen en cada consulta.

## API

- `GET /api/v1/collections?status=all&q=&limit=20&offset=0`: estados `all`, `overdue`, `today`, `upcoming`, calculados en la zona horaria de la empresa. Busca nombre o documento, ordena por fecha e ID y devuelve la paginación habitual.
- `POST /api/v1/loans/:id/payments`: conserva el contrato anterior; admite `distribution: {principal, interest, late_fee}` como importes decimales opcionales. La suma debe coincidir con `amount` y cada componente debe caber en su saldo pendiente. Sin distribución explícita se conserva el orden automático configurado. Los componentes explícitos se aplican a las cuotas más antiguas primero.
- `POST /api/v1/loans/:id/extensions`: requiere `Idempotency-Key`, `first_payment_date`, `additional_interest` (importe total en soles, admite cero) y `note` (1–500 caracteres). Solo préstamos activos. La fecha debe ser posterior a la primera cuota con saldo. Desplaza todas las cuotas pendientes la misma cantidad de días y añade el interés acordado a la última cuota pendiente.
- `GET /api/v1/loans/:id/extensions?limit=20&offset=0`: historial de acuerdos.

Dinero recibido y distribución son conceptos independientes: un pago de S/70 puede corresponder a S/20 de interés y S/50 de capital y recibirse mediante efectivo más Yape. Pagar solamente interés no amplía el plazo ni genera interés adicional automáticamente.

## Integridad

Pagos y ampliaciones utilizan el mismo bloqueo transaccional por empresa. Un reintento con la misma clave y contenido devuelve la operación existente; cambiar el contenido con la misma clave devuelve conflicto. Una ampliación conserva una instantánea del cronograma anterior y una auditoría. Los acuerdos no admiten modificaciones, borrados ni truncado en PostgreSQL. Los pagos anteriores conservan sus importes y asignaciones. Ampliar no registra ingreso de caja.

El interés adicional de una ampliación es un nuevo importe explícitamente acordado; el interés original del préstamo sigue siendo TOTAL. No se recalcula automáticamente una tasa mensual, mora ni capitalización.

## Verificación

`TestCollections_InterestPrincipalAndExtension` utiliza PostgreSQL real en Docker y cubre saldos, métodos mixtos, asignación explícita, ampliación, reintentos, permisos, cookies/CSRF, validaciones, inmutabilidad y aislamiento entre empresas. Ejecutar con `TEST_DB_PORT=55433` cuando la base local de desarrollo usa 55432.
