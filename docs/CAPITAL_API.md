# API de capital (caja única)

Prefijo: `/api/v1/capital`. Todo el dinero viaja como **texto decimal** (`"1000.50"`), nunca como número JSON.

## Conceptos

**Caja única.** Cada empresa tiene una sola caja. Todo ingreso y egreso afecta ese único saldo, sin importar el método de pago (efectivo, Yape, Plin, transferencia). El método, si se usa, es solo un dato informativo en `reference`.

**Libro de movimientos (append-only).** El saldo no se guarda: se obtiene sumando los movimientos. Las filas **nunca se editan ni se borran**, ni siquiera lógicamente. Si un registro estaba mal, se corrige con un movimiento `adjustment_in` o `adjustment_out` que explique el motivo en `note`.

**Inmutabilidad en la base de datos.** La tabla `capital_movements` tiene triggers `BEFORE UPDATE OR DELETE` y `BEFORE TRUNCATE` que lanzan `capital_movements es append-only`. Se aplican también a superusuarios.

**Amount siempre positivo.** La dirección (entra o sale) la define el tipo, no el signo del monto.

**Buckets.** El saldo se separa en dos partes:

| Bucket | Qué contiene |
|---|---|
| `capital` | Dinero propio que se puede prestar (aportes, desembolsos, cobros de principal). |
| `profit` | Utilidad: intereses, mora cobrada y gastos. |

`total_cash` = `capital_balance` + `profit_balance`.

## Tipos de movimiento

| Tipo | Dirección | Bucket | Registro |
|---|---|---|---|
| `contribution` | in | capital | **Manual** (owner, admin) |
| `withdrawal` | out | capital | **Manual** (solo owner) |
| `adjustment_in` | in | capital | **Manual** (owner, admin); `note` obligatoria |
| `adjustment_out` | out | capital | **Manual** (owner, admin); `note` obligatoria |
| `disbursement` | out | capital | Interno: lo escribe el módulo de préstamos |
| `principal_collection` | in | capital | Interno: módulo de pagos |
| `interest_collection` | in | profit | Interno: módulo de pagos |
| `late_fee_collection` | in | profit | Interno: módulo de pagos |
| `expense` | out | profit | Interno |

Por la API manual solo se aceptan los cuatro primeros. Los demás tipos ya se suman en el saldo, pero los escribirán los módulos de préstamos y pagos.

`lent_outstanding` (capital prestado en la calle) = suma de `disbursement` − suma de `principal_collection`.

## Endpoints

### `GET /capital`
Saldo de la caja. Roles: `owner`, `admin`, `viewer`.

```json
{
  "total_cash": "700.00",
  "capital_balance": "700.00",
  "profit_balance": "0.00",
  "lent_outstanding": "0.00",
  "movements_count": 2,
  "as_of": "2026-11-05T10:00:00Z"
}
```

### `POST /capital/movements`
Registra un movimiento manual. Roles: `owner`, `admin`. Un `admin` no puede registrar `withdrawal` (403).

Request:
```json
{ "type": "withdrawal", "amount": "300.00", "note": "Retiro de utilidades", "reference": "TRF-0042" }
```

Respuesta `201`:
```json
{
  "id": "0190f3a2-...", "type": "withdrawal", "direction": "out", "bucket": "capital",
  "amount": "300.00", "note": "Retiro de utilidades", "reference": "TRF-0042",
  "created_by": "0190f39e-...", "created_at": "2026-11-05T10:00:00Z"
}
```

Reglas:
- `amount`: texto decimal mayor que cero, hasta 4 decimales (`"1.23456"` no es válido).
- `note`: máximo 500 caracteres. **Obligatoria** (mínimo 5) en `adjustment_in` y `adjustment_out`.
- `reference`: máximo 100 caracteres.
- `withdrawal` y `adjustment_out` no pueden dejar `total_cash` negativo, salvo que la empresa tenga activado `allow_negative_capital` (configuración de la empresa).

### `GET /capital/movements`
Listado del libro, de más reciente a más antiguo. Roles: `owner`, `admin`, `viewer`.

Parámetros:

| Parámetro | Descripción |
|---|---|
| `limit` | 1 a 100, por defecto 20. |
| `after` | `id` del último elemento visto (cursor). Debe ser un UUID. |
| `type` | Uno de los 9 tipos. |
| `from`, `to` | RFC3339 sobre `created_at` (`from` inclusive, `to` inclusive). |

Respuesta:
```json
{
  "items": [ { "id": "...", "type": "contribution", "...": "..." } ],
  "meta": { "limit": 20, "next_cursor": "0190f3a1-...", "has_more": true }
}
```

## Paginación por cursor

El libro crece sin límite, así que el listado usa **keyset** (`?after=<id>`) y no `OFFSET`: cada página cuesta lo mismo sin importar cuán profunda sea.

- Orden: `id DESC`. Los UUID son UUIDv7 (ordenables por tiempo).
- `meta.next_cursor` es el `id` del último item de la página, o `null` si no hay más páginas.
- `meta.has_more` se calcula pidiendo `limit+1` filas. **No hay `total`** (contar el libro completo es caro).
- `items` es siempre un arreglo (`[]`, nunca `null`).
- Para recorrer: pedir sin `after`, luego pasar `after=<next_cursor>` mientras `has_more` sea `true`.

El helper reutilizable está en `internal/platform/httpx/cursor.go` (`ParseCursor`, `NewCursorPage`).

## Roles

| Acción | owner | admin | viewer | cobrador |
|---|---|---|---|---|
| `GET /capital`, `GET /capital/movements` | sí | sí | sí | 403 |
| `POST` aporte, ajuste | sí | sí | 403 | 403 |
| `POST` retiro | sí | 403 | 403 | 403 |

## Errores

Formato estándar de la API (ver `docs/API_CONVENCIONES.md`).

| HTTP | `code` | Cuándo |
|---|---|---|
| 400 | `VALIDATION_ERROR` | Monto, nota, referencia, tipo, `limit`, `after`, `from` o `to` inválidos. `fields` indica cada campo. |
| 400 | `INVALID_JSON` | Cuerpo no válido, o `amount` enviado como número (`100` en vez de `"100"`). |
| 403 | `FORBIDDEN` | Rol sin permiso, o un `admin` intenta retirar capital. |
| 409 | `INSUFFICIENT_CAPITAL` | El movimiento dejaría la caja en negativo. El saldo no cambia. |

## Concurrencia

Las escrituras de una empresa se serializan con un bloqueo transaccional (`pg_advisory_xact_lock`) que cubre la comprobación de fondos y el INSERT. Dos retiros simultáneos no pueden sobregirar la caja.
