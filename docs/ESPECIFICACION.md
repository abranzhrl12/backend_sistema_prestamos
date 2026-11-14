# Especificación funcional — Sistema de Préstamos Multitenant

> **Alcance vigente:** [Préstamos personales](PRESTAMOS_PERSONALES.md). Las plantillas obligatorias, los rangos de tasas/montos y los métodos financieros descritos abajo son antecedentes; no deben volver a imponerse al flujo de préstamos personales.

> Documento vivo. Marca `[MVP]` = primera entrega, `[v2]` = después, `[idea]` = por validar.
> Moneda base por tenant (PEN por defecto). Todo el dinero: `NUMERIC(19,4)` / `decimal`, nunca `float`.

## 1. Módulos

| # | Módulo | Resumen |
|---|---|---|
| 1 | Clientes | Alta, edición, historial y calificación |
| 2 | Préstamos | Simulación, desembolso, cronograma, estados |
| 3 | Pagos | Registro, métodos (Yape, Plin, transferencia, efectivo, mixto), aplicación a cuotas |
| 4 | Mora | Días de atraso, penalidad, estado de riesgo |
| 5 | Capital | Capital disponible, aportes, retiros, flujo de caja |
| 6 | Reportes | Recaudación, cartera, morosidad, historial |
| 7 | Usuarios y roles | Admin, cobrador, solo lectura |
| 8 | Auditoría | Quién hizo qué y cuándo |

---

## 2. Clientes `[MVP]`

**Datos:** nombres, DNI/CE/RUC (único por tenant), teléfono(s), WhatsApp, dirección, referencia de ubicación, correo, ocupación, ingreso estimado, foto/documentos `[v2]`, notas.
**Aval / garante** `[v2]`: nombre, DNI, teléfono, relación.
**Reglas**
- Documento único por tenant; no se borra un cliente con préstamos (solo se desactiva).
- Estado: `activo`, `inactivo`, `bloqueado` (no se le puede prestar).
- Un cliente puede tener varios préstamos, con límite configurable de préstamos activos simultáneos.
**Vista de cliente:** préstamos (activos/cerrados), deuda total pendiente, días de atraso máximo, total pagado histórico, **score interno** (A/B/C/D) calculado por puntualidad `[v2]`.
**Acciones:** buscar por nombre/documento/teléfono, filtros por estado y por mora, exportar.

---

## 3. Préstamos `[MVP]`

### 3.1 Parámetros del préstamo
| Campo | Detalle |
|---|---|
| Monto (capital) | > 0, dentro de min/max del tenant |
| Tasa de interés | % por período; se guarda además la **tasa anual equivalente (TEA)** |
| Tipo de tasa | Simple sobre capital inicial · Sobre saldo (francés) · Alemán (capital constante) · Interés fijo plano |
| Frecuencia | Diario · Semanal · Quincenal · Mensual |
| Duración | Nº de cuotas (y plazo en días calculado) |
| Fecha de desembolso | Define la 1.ª fecha de cuota |
| Comisión / gastos de apertura | % o monto fijo, opcional, descontado o financiado |
| Penalidad por mora | Ver sección 5 |
| Cobrador asignado | Usuario responsable |
| Garantía / observaciones | Texto libre, adjuntos `[v2]` |

### 3.1.1 Productos de préstamo (plantillas configurables) `[MVP]`
Cada tenant define sus productos; al crear un préstamo se elige uno y se pueden sobreescribir los valores. Nada de tasas ni frecuencias fijas en el código.

| Producto (ejemplo) | Frecuencia | Tasa | Plazo | Notas |
|---|---|---|---|---|
| Mensual estándar *(por defecto)* | Mensual | p. ej. 10 % mensual | 1–24 cuotas | El caso más común |
| Microcrédito semanal | Semanal | menor, p. ej. 3 % semanal | 1–12 semanas | Montos bajos; incluye "1 semana, pago único" |
| Pago único (bullet) | Una sola cuota | por período | 1 cuota | Capital + interés al vencimiento |
| Diario `[v2]` | Diario | por día | hasta 60 días | Cobrador en ruta |

Cada producto define: nombre, frecuencia por defecto, rango de tasa (mín/máx), tipo de interés, rango de monto, rango de cuotas, días de gracia, regla de mora y comisión. Se pueden activar/desactivar.

**Regla de la tasa:** siempre se guarda **tasa + período al que aplica** (`rate=10`, `rate_period=monthly`) y el sistema calcula y muestra la **TEA equivalente**. Así un 3 % semanal y un 10 % mensual son comparables y no hay ambigüedad.

**Validación:** el sistema avisa (sin bloquear, salvo configuración) si la tasa queda fuera del rango del producto o supera el tope de usura vigente `[idea]`.

### 3.2 Simulador
Antes de crear: muestra cronograma, total a pagar, total de intereses, cuota, TEA. No persiste nada.

### 3.3 Cronograma (tabla de amortización)
Por cuota: nº, fecha de vencimiento, capital, interés, cuota total, saldo, y lo pagado de cada componente. Opción de **omitir domingos/feriados** (calendario por tenant) `[v2]`.

### 3.4 Estados
`pendiente` (aprobación) → `activo` (desembolsado) → `pagado` · `en mora` (derivado) · `refinanciado` · `castigado` · `cancelado`.
- `en mora` se **calcula** (cuota vencida impaga), no se edita a mano.
- Desembolsar descuenta del capital disponible (sección 6) y falla si no alcanza (configurable: permitir o no capital negativo).

### 3.5 Operaciones especiales
- **Pago anticipado / cancelación total** `[MVP]`: es una acción simple, sin políticas configurables. Al liquidar antes del vencimiento, el sistema **sugiere** el interés proporcional a los días transcurridos (interés del período ÷ días del período × días reales) y el **cobrador/admin puede editar el monto** (cobrar todo, solo los días, o lo que decida). Se guarda el interés cobrado, el sugerido y quién lo modificó. Al confirmar, el préstamo se cierra.
- **Ajustes especiales** `[MVP]`: descuento de interés, condonación total/parcial de mora, redondeo. Requieren permiso (`owner`/`admin`), **motivo obligatorio** y quedan en `audit_log` y en el historial del préstamo como movimiento separado (nunca se edita el cronograma original en silencio).
- **Renovación / refinanciación:** cierra el préstamo y abre uno nuevo con el saldo `[v2]`.
- **Reprogramación de cuota:** mover fecha con motivo y auditoría `[v2]`.
- **Perdón/condonación de mora** con permiso especial y auditoría.

---

## 4. Pagos `[MVP]`

### 4.1 Métodos de pago
`efectivo` (pago directo) · `yape` · `plin` · `transferencia` · `deposito` `[v2]` · `tarjeta` `[idea]` · **`mixto`**.

| Método | Datos a registrar |
|---|---|
| Efectivo | Monto, quién cobró |
| Yape / Plin | Monto, nº de operación, teléfono origen, captura `[v2]` |
| Transferencia | Monto, banco, nº de operación, cuenta destino |
| Mixto | Lista de **partes**: cada una con método y monto; la suma debe igualar el pago total |

**Modelo:** un `pago` tiene N `pago_partes` (método + monto + referencia). Un pago simple es un pago con 1 parte. Así "mixto" no es un caso especial.

### 4.2 Reglas de aplicación
Orden por defecto de imputación (configurable por tenant): **1) penalidad/mora → 2) interés vencido → 3) capital vencido → 4) cuotas futuras**.
- Cada pago genera `asignaciones` por cuota (capital / interés / mora) → trazabilidad exacta.
- Pago parcial permitido; pago mayor a la cuota adelanta las siguientes.
- Número de operación único por método y tenant (evita doble registro).
- **Anulación** de pago (nunca borrado): revierte asignaciones, exige motivo y rol, queda en auditoría.
- Comprobante/recibo imprimible o compartible (PDF/WhatsApp) `[v2]`.
- Verificación manual de Yape/Plin/transferencia: estado `por verificar` → `verificado` `[v2]`.

### 4.3 Historial de pagos
Por préstamo y por cliente: fecha, monto, método, partes, cobrador, cuotas afectadas, saldo posterior, estado (válido/anulado).

---

## 5. Mora y atrasos `[MVP]`

- **Días de atraso** = hoy − fecha de vencimiento de la cuota impaga más antigua (por cuota y máximo por préstamo).
- Configuración por préstamo/tenant:
  - **Días de gracia** (ej. 0–3).
  - **Modo de mora (un selector simple por producto, editable por préstamo):**
    1. `fijo` *(por defecto)*: monto fijo por cuota vencida (p. ej. S/ 10 una vez), o ese monto **por cada día** de atraso si se marca la casilla "por día".
    2. `proporcional`: la tasa del período se divide entre los días del período y se cobra por los días de atraso. Ej.: 15 % mensual → 15 % ÷ 30 = 0.5 % por día × 3 días = 1.5 % sobre la cuota vencida.
  - **Tope máximo** de penalidad (ej. 100 % del capital de la cuota).
- Cálculo por **job diario** (cron) que devenga la mora; idempotente, registra cuánto devengó por día.
- **Clasificación de riesgo** por días: `al día` · `1–7` · `8–30` · `31–60` · `61–90` · `+90 (crítico)`.
- Indicador por cuota: `pagada a tiempo` · `pagada con atraso (N días)` · `vencida` · `pendiente`.
- Recordatorios automáticos (WhatsApp/SMS) antes y después del vencimiento `[v2]`.

---

## 6. Capital disponible `[MVP]`

Controla el dinero con el que se puede prestar.

**Movimientos de capital** (libro contable simple, solo se agregan filas):
| Tipo | Efecto |
|---|---|
| `aporte` (inversión del dueño/socio) | + capital |
| `retiro` / utilidades | − capital |
| `desembolso` de préstamo | − capital |
| `cobro` de capital | + capital |
| `cobro` de interés/mora | + utilidad (separado del capital) |
| `gasto` operativo (movilidad, etc.) | − utilidad `[v2]` |
| `ajuste` manual (con motivo) | ± |

**Cálculos:** capital total · capital prestado en la calle · **capital disponible** · utilidad acumulada · cartera vencida.
**Caja única** `[MVP]` (decidido): cada tenant tiene **una sola caja**. Todo ingreso y egreso afecta ese único saldo, sin importar el método (efectivo, Yape, Plin o transferencia). El método de pago se guarda solo como dato informativo y para reportes (cuánto entró por cada método), no separa saldos.
**Si más adelante se necesitan varias cajas** `[idea]`: se agregaría una tabla `accounts` y `account_id` en los movimientos; por eso el libro de capital ya se diseña como movimientos, para que ese cambio sea posible sin reescribir.
**Cierre de caja diario por cobrador** `[v2]`: lo cobrado en efectivo vs. lo rendido.
**Socios/inversionistas** `[idea]`: aportes con rendimiento propio.

---

## 7. Reportes `[MVP]` salvo indicación

| Reporte | Contenido |
|---|---|
| **Dinero recaudado** | Por rango de fechas, por día/semana/mes, por método de pago, por cobrador; desglose capital / interés / mora |
| **Historial de pagos** | Por cliente, préstamo o cobrador; filtros por fecha, método, estado |
| **Cartera activa** | Préstamos vigentes, saldo por cobrar, capital en la calle |
| **Morosidad** | Cuotas vencidas, días de atraso, monto vencido + mora, % de cartera en riesgo (PAR 1/30/60/90) |
| **¿Pagó o debe?** | Estado por cliente hoy: pagó / vence hoy / atrasado N días |
| **Cobranza del día** | Lista de cuotas que vencen hoy y vencidas, por cobrador, con ruta/teléfono |
| **Proyección** | Ingresos esperados por semana/mes según cronogramas |
| **Utilidad** | Intereses + mora cobrados − gastos |
| **Flujo de capital** | Aportes, retiros, desembolsos, cobros `[MVP]` |
| **Estado de cuenta del cliente** | PDF con cronograma y pagos `[v2]` |
| **Dashboard** | Tarjetas: capital disponible, recaudado hoy/mes, por cobrar, en mora, nº clientes activos |

Exportación: CSV y Excel `[MVP]`, PDF `[v2]`. Todos los reportes filtran por tenant (RLS) y son paginados.

---

## 8. Usuarios, roles y permisos `[MVP]`

| Rol | Puede |
|---|---|
| `owner` | Todo, incluido capital y configuración del tenant |
| `admin` | Clientes, préstamos, pagos, reportes; no retiros de capital |
| `cobrador` | Ver sus clientes/cuotas asignadas, registrar pagos, ver su cierre |
| `viewer` | Solo lectura de reportes |

Permisos finos por acción (anular pago, condonar mora, ajustar capital) `[v2]`. Auth: ver `authkit` (cookie HttpOnly web / Bearer JWT móvil).

---

## 9. Configuración por tenant `[MVP]`
Moneda · tasa/duración mínimas y máximas · días de gracia · regla de penalidad · orden de imputación de pagos · límite de préstamos activos por cliente · permitir capital negativo · zona horaria (`America/Lima`) · feriados `[v2]` · datos para recibos (razón social, logo).

---

## 10. Auditoría y seguridad
- `audit_log` de: creación/edición de préstamos, pagos, anulaciones, condonaciones, ajustes de capital, cambios de configuración y roles.
- Datos sensibles (documentos) no se registran en logs.
- Aislamiento por tenant con RLS; `tenant_id` siempre desde el token.
- Idempotencia en el registro de pagos (header `Idempotency-Key`) para evitar duplicados por reintentos del móvil.
- Soft delete / anulación en lugar de borrado en datos financieros.

---

## 11. Modelo de datos — cambios sobre la migración inicial

Nuevas/ajustadas (migración `00002`):
- `customers`: + `phone2`, `whatsapp`, `email`, `occupation`, `status`, `notes`, `assigned_collector_id`.
- `loan_products` (nuevo): `id`, `tenant_id`, `name`, `frequency`, `rate_period`, `rate_min/max`, `interest_type`, `amount_min/max`, `terms_min/max`, `grace_days`, `late_fee_*`, `fee_*`, `active`. `loans.product_id` apunta aquí (los valores se copian al préstamo para que cambiar el producto no altere préstamos existentes).
- `loans`: al cancelar anticipadamente se guarda `payoff_interest_suggested`, `payoff_interest_charged` y quién lo editó.
- `loan_adjustments` (nuevo, append-only): `id`, `tenant_id`, `loan_id`, `schedule_id?`, `type` (`interest_discount|late_fee_waiver|rounding`), `amount`, `reason`, `created_by`, `created_at`.
- `loans`: + `frequency` (añadir `daily` y `single`), `rate_period`, `fee_amount`, `grace_days`, `late_fee_mode` (`fixed|proportional`), `late_fee_amount`, `late_fee_per_day` (bool), `collector_id`, `closed_at`.
- `loan_schedule`: + `late_fee_accrued`, `paid_at`, `status`.
- `payments`: + `status` (`valid`/`voided`), `voided_by`, `void_reason`, `idempotency_key`; el `method` pasa a **`payment_parts`** (`payment_id`, `method`, `amount`, `reference`, `bank`, `phone`).
- `capital_movements` (`id`, `tenant_id`, `type`, `amount`, `loan_id?`, `payment_id?`, `note`, `created_by`, `created_at`) — **append-only**.
- Sin tabla `accounts`: una sola caja por tenant. El saldo de capital disponible se obtiene sumando `capital_movements`.
- `tenant_settings` (clave/valor tipado o columnas).
- `holidays` `[v2]`.

Índices clave: `(tenant_id, due_date)` para cobranza del día, `(tenant_id, status)` en loans, único `(tenant_id, method, reference)` en `payment_parts` cuando hay referencia.

---

## 12. API (resumen, prefijo `/api/v1`)

```
POST   /auth/login | /auth/refresh | /auth/logout   GET /auth/me
CRUD   /customers            GET /customers/{id}/summary
POST   /loans/simulate       POST /loans   GET /loans?status=&overdue=
POST   /loans/{id}/disburse  GET /loans/{id}/schedule
POST   /loans/{id}/payments  GET /loans/{id}/payments
POST   /payments/{id}/void
GET    /collections/today
GET    /capital              POST /capital/movements   GET /capital/movements
GET    /reports/collected | /reports/overdue | /reports/portfolio | /reports/profit
GET    /dashboard
```

---

## 13. Orden de implementación sugerido

1. **Fase 1:** Clientes → Simulador y préstamos con cronograma → Pagos (efectivo/Yape/Plin/transferencia/mixto) con imputación → Estado de pago y días de atraso.
2. **Fase 2:** Mora devengada (job) → Capital (movimientos y disponible) → Reportes de recaudo, historial y morosidad → Dashboard.
3. **Fase 3:** Cobranza del día por cobrador, cierre de caja por cobrador, recibos PDF, recordatorios WhatsApp.
4. **Fase 4:** Refinanciación, pago anticipado, scoring, garantes, socios/inversionistas.

## 14. Fuera de alcance (por ahora)
Integración automática con APIs de Yape/Plin/bancos (se registran manualmente), contabilidad formal, firma electrónica, app móvil nativa.

## 15. Preguntas abiertas
1. ~~¿Interés por período o TEA?~~ **Decidido:** se guarda tasa + período y se muestra la TEA equivalente. **Decidido:** interés **plano por defecto** (fijo sobre el capital inicial); sobre saldo (francés) y alemán quedan como opciones por producto.
2. ~~¿Mensual o semanal?~~ **Decidido:** mensual por defecto, semanal/único/diario como productos configurables. Pendiente: ¿habrá cobradores en ruta con cobro diario?
3. ~~Mora y pago anticipado~~ **Decidido:** mora con 2 modos simples (`fijo` por defecto, o `proporcional` a la tasa) y pago anticipado manual con interés sugerido editable.
4. ~~¿Una sola caja o varias?~~ **Decidido:** una sola caja por tenant.
5. ¿Hay socios que aportan capital con rendimiento propio?
6. ¿Se necesita multimoneda o solo PEN?
