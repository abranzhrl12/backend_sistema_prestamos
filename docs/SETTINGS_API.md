# Configuración de la empresa y productos de préstamo

Prefijo: `/api/v1`. Errores y paginación siguen `docs/API_CONVENCIONES.md`.
Dinero y porcentajes viajan **siempre como texto decimal** (`"10.00"`), nunca como número.

## 1. Configuración de la empresa (`/settings`)

Una fila por empresa (`tenant_settings`). Mientras la empresa no guarde la suya, la API devuelve los
valores por defecto con `is_default: true` y **no escribe nada**.

| Método | Ruta | Quién | Respuesta |
|---|---|---|---|
| GET | `/settings` | cualquier usuario autenticado | 200 `SettingsResponse` |
| PUT | `/settings` | `owner`, `admin` | 200 `SettingsResponse` · 400 · 403 |

`PUT` es un **reemplazo total**: los nueve campos son obligatorios. Si falta alguno, la respuesta lista
todos los faltantes en `fields` con el mensaje `es obligatorio`.

### Campos

| Campo | Tipo | Defecto | Reglas |
|---|---|---|---|
| `currency` | string | `PEN` | `PEN` o `USD` |
| `timezone` | string | `America/Lima` | Nombre IANA válido (`America/Bogota`, `UTC`...). `Local` y vacío no se aceptan |
| `late_fee_mode` | string | `fixed` | `fixed` o `proportional` |
| `late_fee_amount` | string decimal | `"10.00"` | ≥ 0, hasta 4 decimales |
| `late_fee_per_day` | bool | `false` | La mora se cobra por cada día de atraso |
| `grace_days` | int | `0` | 0 a 30 |
| `allocation_order` | string | `late_fee_interest_principal` | `late_fee_interest_principal` o `interest_principal_late_fee` (orden en que un pago cubre mora, interés y capital) |
| `max_active_loans_per_customer` | int | `3` | 1 a 50 |
| `allow_negative_capital` | bool | `false` | Permite que el capital disponible de la empresa quede negativo |

Campos de solo lectura en la respuesta: `is_default` (bool) y `updated_at` (`null` mientras es la
configuración por defecto).

### Ejemplo

```json
PUT /api/v1/settings
{
  "currency": "USD", "timezone": "America/Bogota", "late_fee_mode": "proportional",
  "late_fee_amount": "2.5", "late_fee_per_day": true, "grace_days": 3,
  "allocation_order": "interest_principal_late_fee", "max_active_loans_per_customer": 5,
  "allow_negative_capital": true
}

200
{ "currency": "USD", "timezone": "America/Bogota", "late_fee_mode": "proportional",
  "late_fee_amount": "2.50", "late_fee_per_day": true, "grace_days": 3,
  "allocation_order": "interest_principal_late_fee", "max_active_loans_per_customer": 5,
  "allow_negative_capital": true, "is_default": false, "updated_at": "2026-11-05T18:00:00Z" }
```

### Auditoría
Cada `PUT` escribe una fila en `audit_log` con `action = settings.update`, el usuario que la hizo y
en `data` la lista de campos que cambiaron (`{"fields": ["grace_days"]}`). Un `PUT` sin cambios
registra una lista vacía.

## 2. Productos de préstamo (`/loan-products`)

| Método | Ruta | Quién | Respuesta |
|---|---|---|---|
| GET | `/loan-products` | cualquier usuario autenticado | 200 página de productos |
| GET | `/loan-products/{id}` | cualquier usuario autenticado | 200 · 404 |
| POST | `/loan-products` | `owner`, `admin` | 201 · 400 · 409 |
| PUT | `/loan-products/{id}` | `owner`, `admin` | 200 · 400 · 404 · 409 |
| DELETE | `/loan-products/{id}` | `owner`, `admin` | 204 · 404 |

- `PUT` reemplaza el producto completo; los opcionales omitidos toman su defecto.
- `DELETE` es **soft delete**: marca `deleted_at` y `deleted_by`. La fila nunca se borra físicamente.
  Un producto eliminado responde 404 en todas las rutas, no aparece en listados y su nombre puede
  reutilizarse.
- Un `{id}` que no es UUID responde 404 `NOT_FOUND`.

### Campos del producto

| Campo | Tipo | Obligatorio | Reglas |
|---|---|---|---|
| `name` | string | sí | 2 a 80 caracteres; único entre productos vivos de la empresa (sin distinguir mayúsculas) |
| `frequency` | string | sí | `daily`, `weekly`, `biweekly`, `monthly` o `single` (pago único) |
| `interest_type` | string | no (`flat`) | `flat`, `french` o `german` |
| `rate_period` | string | sí | `daily`, `weekly`, `biweekly`, `monthly` o `annual` |
| `rate_default`, `rate_min`, `rate_max` | string decimal (%) | sí | 0 ≤ min ≤ default ≤ max ≤ 1000 |
| `amount_min`, `amount_max` | string decimal | sí | 0 < min ≤ max |
| `terms_min`, `terms_max` | int | sí | 1 ≤ min ≤ max ≤ 1000. Con `single` ambos deben ser 1 |
| `grace_days` | int o `null` | no | Override: 0 a 30 |
| `late_fee_mode` | string o `null` | no | Override: `fixed` o `proportional` |
| `late_fee_amount` | string decimal o `null` | no | Override: ≥ 0 |
| `late_fee_per_day` | bool o `null` | no | Override |
| `fee_type` | string | no (`none`) | `none`, `fixed` o `percent` |
| `fee_value` | string decimal | no (`"0"`) | `none` exige 0; `percent` ≤ 100 |
| `active` | bool | no (`true`) | Un producto inactivo no se ofrece para nuevos préstamos |

Los campos `id`, `created_at` y `updated_at` solo aparecen en la respuesta.

### Overrides: `null` significa "usar la empresa"
`grace_days`, `late_fee_mode`, `late_fee_amount` y `late_fee_per_day` son **opcionales**. `null`
(o no enviarlos) indica que el producto usa la configuración de la empresa de `/settings`. Un valor
distinto de `null` reemplaza ese campo solo para este producto.

### Listado
`GET /loan-products` usa la paginación estándar (`limit` 1–100, por defecto 20; `offset`; `has_more`).
`total` solo aparece con `include_total=true`.

| Filtro | Efecto |
|---|---|
| `q` | Texto en el nombre (sin distinguir mayúsculas) |
| `active` | `true` o `false`; omitido = todos |
| `frequency` | Una de las cinco frecuencias |

El orden es `lower(name)` con desempate por `id`.

### Errores

| HTTP | `code` | Cuándo |
|---|---|---|
| 400 | `VALIDATION_ERROR` | Campos inválidos (ver `fields`), paginación o filtros fuera de rango |
| 401 | `UNAUTHENTICATED` / `TOKEN_INVALID` | Sin sesión o token inválido |
| 403 | `FORBIDDEN` | Cobrador intentando escribir |
| 404 | `NOT_FOUND` | "El producto no existe": id inexistente, de otra empresa, eliminado o no UUID |
| 409 | `PRODUCT_NAME_TAKEN` | "Ya existe un producto con ese nombre" |

## 3. Configuración frente a constantes

**Configurable por empresa** (`/settings` y overrides de producto): moneda, zona horaria, modo y monto
de mora, cobro diario de mora, días de gracia, orden de aplicación de pagos, máximo de préstamos
activos por cliente y permiso de capital negativo.

**Constantes del sistema** (no se pueden cambiar desde la API):
- Límites de validación: `grace_days` 0–30, `max_active_loans_per_customer` 1–50, tasas hasta 1000,
  plazos hasta 1000 y nombre de producto de 2 a 80 caracteres.
- Opciones cerradas: monedas `PEN`/`USD`, frecuencias, periodos de tasa, tipos de interés y de cargo.
- Reglas de soft delete y de auditoría.

## 4. Los valores se copian al préstamo

Al crear un préstamo, la API **copia** a la fila del préstamo la tasa, el monto, el plazo, la
frecuencia, los días de gracia, la mora y el cargo que apliquen en ese momento, ya resueltos (producto
más configuración de la empresa). Por eso:

- Cambiar un producto o la configuración **no altera los préstamos existentes**; solo afecta a los
  préstamos creados después.
- Eliminar un producto tampoco altera los préstamos que ya lo usaron.
