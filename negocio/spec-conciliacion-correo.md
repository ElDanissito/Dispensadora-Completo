# Spec — Conciliación de pagos Bre-B por correo (MVP / Fase 0)

> Autor: Agente de Negocio · Fecha: 2026-07-14 · Versión: **v1**
> Destinatario: **Agente de Software** ([`02-software-web.md`](../departamentos/02-software-web.md), [`brief-software.md`](../agentes/brief-software.md)).
> Contexto: [`04-pagos.md`](../departamentos/04-pagos.md) · [`bre-b-guia-negocio.md`](./bre-b-guia-negocio.md) · ADR-004 en [`DECISIONS.md`](../DECISIONS.md).
> Esta spec describe **qué** debe hacer el servicio de conciliación y **con qué reglas**. No impone el diseño interno.

---

## 1. Objetivo

Confirmar de forma **automática y confiable** que una orden fue pagada por Bre-B, leyendo las **notificaciones de correo reales** de la cuenta del negocio, para disparar la emisión del **token/QR de dispensado**.

**Principio no negociable (ADR-004):** la orden solo se marca `PAGADA` con base en la **notificación real de la cuenta**. **Jamás** con el pantallazo/comprobante que muestre el cliente.

**Separación de responsabilidades:** este módulo **solo confirma el pago**. **No** firma el JWT ni genera el QR de dispensado (eso es del módulo de tokens). La salida es un evento `orden.pagada`.

---

## 2. Mecanismo de matching: **monto único por orden**

El correo de una transferencia Bre-B entrante **no trae de forma confiable una referencia libre** que podamos controlar. Por eso el ancla del matching es el **monto exacto e irrepetible** dentro de una ventana de tiempo.

**Cómo se genera el monto único:**
- Precio base de la orden `P` (suma de ítems).
- Se le suma un **desambiguador de centavos/unidades** `d` (p. ej. 1–99 pesos) que **no esté activo** en ninguna otra orden pendiente de la **misma máquina/cuenta** en la ventana de tiempo.
- `monto_a_pagar = P + d`. Ejemplo: base $2.300 → se cobra **$2.347**.
- `d` se reserva mientras la orden esté `PENDIENTE`; se libera al pasar a `PAGADA`, `EXPIRADA` o `CANCELADA`.

**Reglas del matching:**
- Un pago casa con una orden si: **monto entrante == `monto_a_pagar` exacto** (tolerancia **0**) **y** el correo llegó dentro de la **ventana de validez** de la orden.
- Si dos órdenes pudieran colisionar en monto, el generador de `d` **debe impedirlo** (unicidad de `monto_a_pagar` entre órdenes `PENDIENTE` de la misma cuenta). Si no hay `d` libre, ampliar el rango o rechazar creación de orden hasta liberar uno.

> Alternativa evaluada — **referencia/nota en la transferencia**: descartada como ancla principal porque su presencia y propagación al correo del receptor no es garantizada entre entidades. Si se confirma que la entidad del negocio la expone de forma fiable, puede añadirse como **segundo factor** de confirmación (defensa en profundidad), no como sustituto del monto único.

---

## 3. Modelo de estados de la orden

```
PENDIENTE ──(pago casado en ventana)──► PAGADA ──► (emite token/QR)
    │
    ├─(vence la ventana sin pago)──────► EXPIRADA
    │
    └─(cancelación manual/cliente)─────► CANCELADA

PAGO_HUERFANO  = pago recibido que no casa con ninguna orden (requiere revisión/soporte)
```

**Ventana de validez de la orden (para pago):** propuesta **15 min** desde la creación (distinta de la expiración del token de dispensado, que es ~5 min tras emitirlo — ver decisión pendiente en `DECISIONS.md`). Configurable.

---

## 4. Entradas y salidas (contrato)

### Entrada A — Orden pendiente (creada por el front/checkout, Dept. 02)
```json
{
  "order_id": "uuid",
  "machine_id": "mid-001",
  "items": [{ "sku": "AGUA-600", "qty": 1, "price": 2300 }],
  "base_amount": 2300,
  "unique_amount": 2347,          // base + desambiguador
  "currency": "COP",
  "created_at": "2026-07-14T10:00:00-05:00",
  "pay_window_expires_at": "2026-07-14T10:15:00-05:00",
  "status": "PENDIENTE"
}
```

### Entrada B — Notificación de correo (fuente de verdad del pago)
El servicio lee el buzón de la cuenta del negocio (API Gmail o IMAP) filtrando por **remitente oficial verificado** de la entidad (allowlist estricta). De cada correo extrae:
- `remitente_email` (debe estar en allowlist)
- `monto` (normalizado a entero COP)
- `fecha_hora` de la transacción
- `pagador` (nombre/últimos dígitos, si el correo lo trae — **opcional**, solo auditoría)
- `referencia_banco` (si existe — auditoría)
- `raw_email_id` (message-id para idempotencia y auditoría)

### Salida — Evento `orden.pagada`
```json
{
  "order_id": "uuid",
  "machine_id": "mid-001",
  "paid_amount": 2347,
  "paid_at": "2026-07-14T10:03:11-05:00",
  "match_source": "email",
  "bank_message_id": "…",
  "matched_at": "2026-07-14T10:03:14-05:00"
}
```
Este evento es el **único** disparador autorizado para que el módulo de tokens firme el JWT y genere el QR.

---

## 5. Algoritmo de conciliación (pseudocódigo)

```
por cada correo NUEVO en el buzón:
    si remitente NO en ALLOWLIST_ENTIDAD:           -> descartar (log seguridad)
    si message_id ya procesado:                     -> ignorar (idempotencia)
    monto, fecha = parse(correo)
    si parse falla:                                 -> marcar PARSE_FALLIDO + alerta
    candidatas = ordenes(status=PENDIENTE,
                         unique_amount == monto,
                         created_at <= fecha <= pay_window_expires_at)
    segun cardinalidad(candidatas):
        1  -> marcar orden PAGADA; emitir evento orden.pagada; registrar pago
        0  -> registrar PAGO_HUERFANO (revisión/soporte/reembolso)
        >1 -> no debería ocurrir (unicidad de monto); marcar CONFLICTO + alerta manual
    marcar message_id como procesado (siempre)
```

---

## 6. Casos borde y política

| Caso | Qué pasa | Política |
|---|---|---|
| **Pago correcto en ventana** | Casa 1 orden | `PAGADA` → emite QR. Camino feliz. |
| **Pago tardío** (tras expirar la ventana) | No casa (orden ya `EXPIRADA`) | `PAGO_HUERFANO` → soporte contacta y **reembolsa** o **reemite** manualmente. |
| **Monto incorrecto** (cliente tecleó mal) | No casa por monto exacto | `PAGO_HUERFANO` → soporte reembolsa la diferencia o la totalidad; instrucciones claras en la web para reducirlo. |
| **Pago duplicado** (cliente paga dos veces) | 1.º casa, 2.º queda huérfano | 2.º pago → `PAGO_HUERFANO` → **reembolso** del duplicado. |
| **Orden expirada antes de pagar** | Ventana vencida | Cliente debe recrear la orden (nuevo `unique_amount`). |
| **Correo no parseable / formato cambió** | `PARSE_FALLIDO` | Alerta a operador; el pago no se pierde (queda el correo), se resuelve manual y se corrige el parser. |
| **Correo de phishing** simulando pago | Remitente fuera de allowlist | Descartado por seguridad; nunca dispara emisión. |
| **Colisión de monto** entre 2 órdenes | Prevención en creación de orden (unicidad de `unique_amount`) | Si aun así ocurre: `CONFLICTO` → resolución manual. |
| **Reembolso** | Cualquier huérfano/duplicado/erróneo | Proceso manual en MVP + registro; documentar en política de reembolsos (pendiente, Dept. 04). |

---

## 7. Seguridad e idempotencia (requisitos duros)

1. **Allowlist estricta de remitente**: solo correos del dominio/remitente oficial verificado de la entidad disparan lógica de pago. Todo lo demás se ignora y se registra.
2. **Idempotencia por `message_id`**: un mismo correo nunca marca dos veces una orden ni emite dos eventos.
3. **Nunca seguir enlaces ni ejecutar contenido** del correo; solo extraer texto.
4. **Confianza única en la cuenta**: el pantallazo del cliente **no** es entrada válida.
5. **Auditoría total**: persistir cada pago (casado, huérfano, fallido) con su `message_id`, monto, hora y resultado, para conciliación contable (Dept. 07).
6. **Secretos fuera del repo**: credenciales de Gmail/IMAP en variables de entorno/secret manager, jamás en git (coherente con la regla de la llave privada del token).

---

## 8. KPIs que debe permitir medir (Dept. 04)

- **Tiempo pago→QR** (objetivo Fase 0 < 30 s).
- **Tasa de conciliación automática** (% pagos casados sin intervención; objetivo > 98 %).
- **Pagos huérfanos / disputas por semana** (minimizar).
- **Parse fallidos por semana** (señal para migrar a webhook).

---

## 9. Muestras necesarias (bloqueante para implementar)

Negocio entregará al agente de Software **≥ 3 correos reales** de notificación de pago Bre-B entrante de la cuenta del piloto (con datos sensibles ofuscados si es preciso), para:
- fijar el **remitente oficial** exacto (allowlist),
- construir el **parser** de monto/fecha,
- validar el **formato** y su estabilidad.

> Sin muestras reales no se puede fijar el parser; es el primer paso operativo (ver checklist en la guía Bre-B §6).

---

## 10. Camino de evolución (para no sobre-invertir en el parser)

Este mecanismo es **puente**. En cuanto haya volumen o los `PARSE_FALLIDO` suban, migrar a **QR dinámico** (Fase 1) y luego a **webhook de agregador** (Fase 2, ver [comparativa](./agregadores-bre-b-comparativa.md)), que entrega confirmación estructurada e instantánea y **elimina el parseo de correos**. El contrato de salida (`orden.pagada`) se mantiene igual para que el cambio no afecte al resto del sistema.

---

## Notas para otros departamentos

- **Software (02):** implementar según esta spec; mantener el evento `orden.pagada` como única interfaz hacia el módulo de tokens. Pedir a Negocio las muestras de correo antes de codificar el parser.
- **Finanzas/Legal (07):** la comisión Bre-B directa ≈ 0 en el piloto; vigilar el posible **decreto de retención ~1,5 %** (ver guía Bre-B §5) para el unit economics. Los reembolsos manuales tienen costo operativo (tiempo).
- **Operaciones (05):** definir el **protocolo de "pagué y no cayó"** y reembolso, que se conecta con `PAGO_HUERFANO` y con el sensor de dispensado.
