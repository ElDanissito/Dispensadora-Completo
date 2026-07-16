# Spec — Conciliación de pagos Bre-B por correo (MVP / Fase 0)

> Autor: Agente de Negocio · v1 2026-07-14 · **v2 FINAL 2026-07-16** — lista para implementar.
> Destinatario: **Agente de Software** ([`02-software-web.md`](../departamentos/02-software-web.md), [`brief-software.md`](../agentes/brief-software.md)).
> Contexto: [`04-pagos.md`](../departamentos/04-pagos.md) · [`bre-b-guia-negocio.md`](./bre-b-guia-negocio.md) · ADR-004, ADR-007, ADR-010, ADR-012 en [`DECISIONS.md`](../DECISIONS.md).
> Esta spec describe **qué** debe hacer el servicio de conciliación y **con qué reglas**, y **qué reemplaza** del código actual. No impone el diseño interno.

---

## 0. Qué cambia respecto a hoy (objetivo concreto)

Hoy el flujo de pago está **simulado**: `POST /m/{id}/simular-pago` crea la orden con estado `paid_sim` y **firma el token/QR de inmediato**, sin pago real (ver [`software/README.md`](../software/README.md) §"Ciclo web→máquina"). Eso es un atajo de pruebas.

**Esta spec define el pago real que lo reemplaza:** la orden se crea `PENDIENTE`, la web muestra **cuánto y a qué llave Bre-B pagar** (monto único), y el QR **solo se emite** cuando la conciliación por correo confirma el abono real en la cuenta del negocio (Bancolombia, ver guía §3-A). El botón "Simular pago y generar QR" desaparece del flujo del cliente (puede quedar detrás de `ADMIN` o de un flag de pruebas, nunca en producción).

**Principio no negociable (ADR-004):** la orden solo pasa a `PAGADA` con base en la **notificación real de la cuenta**. **Jamás** con el pantallazo/comprobante que muestre el cliente.

**Separación de responsabilidades:** este módulo **solo confirma el pago**. **No** firma el JWT ni genera el QR (eso sigue en `dsptoken.Sign`). La salida es un evento/transición `orden.pagada` que dispara la firma — exactamente el mismo punto que hoy invoca `simular-pago`.

---

## 1. Objetivo

Confirmar de forma **automática y confiable** que una orden fue pagada por Bre-B, leyendo las **notificaciones de correo reales** de la cuenta del negocio, para disparar la emisión del **token/QR de dispensado** y el **descuento de stock** (ADR-012).

---

## 2. Mecanismo de matching: **monto único por orden**

El correo de una transferencia Bre-B entrante **no trae de forma confiable una referencia libre** que el comercio controle. Por eso el ancla del matching es el **monto exacto e irrepetible** dentro de una ventana de tiempo.

**Cómo se genera el monto único:**
- Precio base de la orden `P` = suma de ítems (`total_cop` actual).
- Se le suma un **desambiguador** `d` de pesos (rango sugerido **1–99**) que **no esté activo** en ninguna otra orden `PENDIENTE` de la **misma cuenta** dentro de la ventana.
- `monto_a_pagar = P + d`. Ejemplo: base $2.300 → se cobra **$2.347**.
- `d` se **reserva** mientras la orden esté `PENDIENTE`; se **libera** al pasar a `PAGADA`, `EXPIRADA` o `CANCELADA`.
- El cliente ve en pantalla **el valor exacto a transferir** (con los centavos/pesos del desambiguador resaltados) y la **llave Bre-B** (o el QR de negocio) de destino.

**Reglas del matching:**
- Un pago casa con una orden si: **`monto` entrante == `monto_a_pagar` exacto** (tolerancia **0**) **y** el correo llegó dentro de la **ventana de validez de pago** de la orden.
- La unicidad de `monto_a_pagar` entre órdenes `PENDIENTE` de la misma cuenta es **responsabilidad del generador** de `d`. Si no hay `d` libre en el rango, ampliarlo o **rechazar la creación** de la orden hasta liberar uno (poco probable con el volumen del piloto).

> Alternativa evaluada — **referencia/nota en la transferencia**: descartada como ancla principal porque su presencia y su propagación al correo del receptor **no está garantizada** entre entidades. Si se confirma que Bancolombia la expone de forma fiable en el correo, puede añadirse como **segundo factor** de confirmación (defensa en profundidad), nunca como sustituto del monto único.

---

## 3. Modelo de estados de la orden

```
PENDIENTE ──(pago casado en ventana)──► PAGADA ──► (firma token/QR + descuenta stock)
    │
    ├─(vence la ventana sin pago)──────► EXPIRADA
    │
    └─(cancelación manual/cliente)─────► CANCELADA

PAGO_HUERFANO = abono recibido que no casa con ninguna orden (revisión/soporte/reembolso)
```

**Ventana de validez de pago:** propuesta **15 min** desde la creación de la orden. Es **distinta** de la expiración del token de dispensado (`exp` = ~5 min tras **emitirlo**, ADR-006 / decisión de 300 s). Es decir: el reloj de 5 min del token **arranca cuando se paga**, no cuando se crea la orden. Configurable en servidor.

> **Cambio de esquema requerido (Dept. 02):** la tabla `orders` actual tiene `status` = `pending|paid|dispensed|expired` y `total_cop`, pero **no** tiene columna de monto único ni de ventana de pago. Añadir:
> - `unique_amount INTEGER NOT NULL` (= `total_cop + d`),
> - `pay_window_expires_at INTEGER NOT NULL` (epoch s),
> - y alinear el enum de estado con esta spec (`PENDIENTE→pending`, `PAGADA→paid`, `EXPIRADA→expired`, más `canceled` y el marcador de `paid_sim` solo para pruebas). El nombre exacto de columnas/estados lo decide Software; lo que importa es el contrato de §4.

---

## 4. Entradas y salidas (contrato)

### Entrada A — Orden pendiente (creada por el checkout, Dept. 02)
```json
{
  "order_id": "jti-uuid",
  "machine_id": "M001",
  "items": [{ "slot": 3, "qty": 1, "price_cop": 2300 }],
  "base_amount": 2300,
  "unique_amount": 2347,             // base + desambiguador d
  "currency": "COP",
  "created_at": "2026-07-16T10:00:00-05:00",
  "pay_window_expires_at": "2026-07-16T10:15:00-05:00",
  "status": "PENDIENTE"
}
```

### Entrada B — Notificación de correo (fuente de verdad del pago)
El servicio lee el buzón **dedicado** de la cuenta del negocio (**Gmail API** preferida sobre IMAP por OAuth y push/`watch`) filtrando por **remitente oficial verificado** de Bancolombia (allowlist estricta). De cada correo extrae **cuatro campos** (los que pidió Negocio):

| Campo | Origen en el correo | Uso | Obligatorio |
|---|---|---|---|
| **remitente** (`remitente_email`) | cabecera `From` | Debe estar en la **allowlist** oficial; si no, se descarta (seguridad). | Sí |
| **monto** (`monto`) | cuerpo del correo, normalizado a **entero COP** (quitar `$`, separadores de miles, decimales `,00`) | **Ancla del matching** contra `unique_amount`. | Sí |
| **hora** (`fecha_hora`) | fecha de la transacción en el cuerpo; *fallback* a la cabecera `Date` | Validar que cae dentro de la ventana de pago. | Sí |
| **referencia** (`referencia_banco`) | nº de comprobante/aprobación del correo | **Solo auditoría / segundo factor** (no ancla). | No |

Campos auxiliares: `pagador` (nombre o últimos dígitos, si el correo lo trae — **solo auditoría**, no se muestra al cliente ni decide nada) y `raw_email_id` (Message-ID, para **idempotencia** y auditoría).

> ⚠️ **Normalización del monto**: el parser debe ser robusto al formato COP de Bancolombia (`$2.347,00` / `$ 2.347` / `2347`). Casar por **entero de pesos**. Definir el formato exacto con las **muestras reales** (§9).

### Salida — Transición `orden.pagada`
```json
{
  "order_id": "jti-uuid",
  "machine_id": "M001",
  "paid_amount": 2347,
  "paid_at": "2026-07-16T10:03:11-05:00",
  "match_source": "email",
  "bank_message_id": "…",
  "matched_at": "2026-07-16T10:03:14-05:00"
}
```
Esta transición es el **único** disparador autorizado para (1) firmar el JWT + generar el QR (donde hoy actúa `simular-pago`) y (2) **descontar stock** de la máquina (ADR-012).

---

## 5. Algoritmo de conciliación (pseudocódigo)

```
LOOP (poll cada ~10–15 s, o push de Gmail watch):
  por cada correo NUEVO en el buzón dedicado:
      si remitente NO en ALLOWLIST_BANCOLOMBIA:       -> descartar (log seguridad)
      si message_id ya procesado:                     -> ignorar (idempotencia)
      monto, fecha, referencia = parse(correo)
      si parse falla (monto/fecha no extraíbles):     -> PARSE_FALLIDO + alerta operador
      candidatas = ordenes(status = PENDIENTE,
                           unique_amount == monto,
                           created_at <= fecha <= pay_window_expires_at)
      segun cardinalidad(candidatas):
          1  -> orden PAGADA; emitir orden.pagada (firma QR + descuenta stock); registrar pago
          0  -> PAGO_HUERFANO (revisión/soporte/reembolso)
          >1 -> no debería ocurrir (unicidad de monto); CONFLICTO + alerta manual
      marcar message_id como procesado (SIEMPRE, aunque no case)

APARTE (barrido de expiración): ordenes PENDIENTE con now > pay_window_expires_at -> EXPIRADA (libera d)
```

---

## 6. Casos borde y política

| Caso | Qué pasa | Política |
|---|---|---|
| **Pago correcto en ventana** | Casa 1 orden | `PAGADA` → firma QR + descuenta stock. Camino feliz. |
| **Pago tardío** (tras expirar la ventana) | No casa (orden ya `EXPIRADA`) | `PAGO_HUERFANO` → soporte contacta y **reembolsa** o **reemite** manual. |
| **Monto incorrecto** (cliente tecleó mal el valor) | No casa por monto exacto | `PAGO_HUERFANO` → soporte reembolsa; instrucciones claras en la web (mostrar el valor exacto, resaltar los pesos del desambiguador) para minimizarlo. |
| **Pago parcial / de más** | No casa (tolerancia 0) | `PAGO_HUERFANO` → reembolso o ajuste manual. |
| **Pago duplicado** (cliente paga dos veces) | 1.º casa; 2.º ya no encuentra orden `PENDIENTE` con ese monto | 2.º → `PAGO_HUERFANO` → **reembolso** del duplicado. |
| **Orden expirada antes de pagar** | Ventana vencida | Cliente **recrea** la orden (nuevo `unique_amount`). La web debe ofrecer "volver a intentar". |
| **Doble entrega del mismo correo** (banco reenvía) | Mismo `message_id` | Idempotencia: se ignora, no marca dos veces. |
| **Correo no parseable / cambió el formato** | `PARSE_FALLIDO` | Alerta a operador; el pago **no se pierde** (queda el correo), se resuelve manual y se corrige el parser. Señal para migrar a webhook. |
| **Correo de phishing** simulando un pago Bre-B | Remitente fuera de allowlist | Descartado por seguridad; **nunca** dispara emisión. (Hay olas de phishing Bre-B — guía §5.) |
| **Colisión de monto** entre 2 órdenes | Prevención en creación (unicidad de `unique_amount` entre `PENDIENTE`) | Si aun así ocurre: `CONFLICTO` → resolución manual. |
| **Pagó y no cayó** (`DISPENSE_FAIL`) | Pago OK pero la máquina no entregó | Fuera de este módulo: protocolo de Operaciones (05) + reembolso; se reconcilia con el log de la máquina (ADR-012 sync oportunista). |

---

## 7. Seguridad e idempotencia (requisitos duros)

1. **Allowlist estricta de remitente**: solo correos del remitente/dominio oficial verificado de Bancolombia disparan lógica de pago. Todo lo demás se ignora y se registra.
2. **Idempotencia por `message_id`**: un mismo correo nunca marca dos veces una orden ni emite dos QR.
3. **Nunca seguir enlaces ni ejecutar contenido** del correo; solo extraer texto plano.
4. **Confianza única en la cuenta**: el pantallazo del cliente **no** es entrada válida (ADR-004).
5. **Auditoría total**: persistir cada abono (casado, huérfano, fallido) con `message_id`, monto, hora, referencia y resultado, para conciliación contable (Dept. 07).
6. **Secretos fuera del repo**: credenciales/token OAuth de Gmail en variables de entorno o secret manager, **jamás** en git (misma regla que la llave privada del token; hoy `DSP_PRIVATE_KEY`).

---

## 8. Impacto en la web (UX del reemplazo del pago simulado)

Para que Software reemplace el botón simulado, el flujo del cliente en `GET /m/{id}` pasa a:

1. Cliente elige productos → **`POST /m/{id}/pagar`** (reemplaza `simular-pago` en el flujo público) → crea orden `PENDIENTE` con `unique_amount` y ventana.
2. La web muestra la **pantalla de pago**: valor exacto a transferir (resaltando el desambiguador), **llave/QR Bre-B** del negocio, cuenta atrás de la ventana, e instrucciones ("paga desde tu app, no cierres esta pantalla").
3. La página **consulta el estado** de la orden (p. ej. `GET /m/{id}/orden/{jti}/estado`, polling cada ~3–5 s o SSE) — **sin JS pesado**, coherente con ADR-011bis.
4. Al recibirse `orden.pagada`, la vista cambia y muestra el **QR de dispensado** (igual que hoy tras `simular-pago`).
5. Si vence la ventana → mensaje de expiración + botón "volver a intentar".

> El atajo `simular-pago`/`paid_sim` puede conservarse **solo** bajo `ADMIN` o un flag de entorno para pruebas de firmware/QR; nunca en la ruta pública de producción.

---

## 9. Muestras necesarias (bloqueante para implementar)

Negocio entregará al agente de Software **≥ 3 correos reales** de notificación de **abono/transferencia Bre-B entrante** de la cuenta Bancolombia del piloto (datos sensibles ofuscados si es preciso), para:
- fijar el **remitente oficial** exacto (allowlist),
- construir el **parser** de **monto + hora (+ referencia)**,
- validar el **formato** y su estabilidad entre montos distintos.

**Cómo obtenerlas** (ver checklist de la guía §6): activar alertas de correo con **tope mínimo bajo** (guía §3-A.2) → hacer **3 transferencias de prueba** de montos distintos desde otro banco/billetera a la llave del negocio → guardar los correos crudos (`.eml`).

> Sin muestras reales no se puede fijar el parser; es el **primer paso operativo** antes de codificar.

---

## 10. KPIs que debe permitir medir (Dept. 04)

- **Tiempo pago→QR** (objetivo Fase 0 < 30 s).
- **Tasa de conciliación automática** (% pagos casados sin intervención; objetivo > 98 %).
- **Pagos huérfanos / disputas por semana** (minimizar).
- **Parse fallidos por semana** (señal para migrar a webhook).

---

## 11. Camino de evolución (para no sobre-invertir en el parser)

Este mecanismo es **puente**. En cuanto haya volumen o suban los `PARSE_FALLIDO`, migrar a **QR dinámico** (Fase 1) y luego a **webhook de agregador** (Fase 2, ver [comparativa](./agregadores-bre-b-comparativa.md)), que entrega confirmación estructurada e instantánea y **elimina el parseo de correos**. El contrato de salida (`orden.pagada`) se mantiene **igual** para que el cambio no afecte al resto del sistema.

---

## Notas para otros departamentos

- **Software (02):** implementar según esta spec. Reemplazar el flujo público de `simular-pago` por `pagar` (orden `PENDIENTE` + monto único + pantalla de pago + polling). Añadir columnas `unique_amount` y `pay_window_expires_at` y alinear estados (§3). Mantener `orden.pagada` como **única interfaz** hacia la firma del token y el descuento de stock. Pedir a Negocio las **muestras de correo** (§9) antes de codificar el parser.
  - **HECHO (2026-07-16, ADR-015):** implementado en `software/internal/{bankmail,imapmail,concil,config}` + reemplazo del flujo web (`/pagar` + pantalla de estado con auto-refresh) + esquema (`unique_amount`, `pay_window_expires_at`, tabla `bank_movements`) + poller IMAP en `cmd/server -concil`. Matching por (máquina + monto único + ventana), idempotencia por `Message-ID`, allowlist de remitente, descuento de stock atómico. Verificado: login IMAP a grabibot OK y parseo del correo real (todos los campos). **Decisión de implementación:** IMAP + App Password en vez de Gmail API para el piloto (ver ADR-015). El monto se casa por **entero de pesos** con el formato real US (`$1,234.00`). **Pendiente:** cargar el valor de la llave Bre-B por máquina en `.env` (`GRABI_BREB_KEY_M001`) para mostrarla exacta en la pantalla de pago.
- **Finanzas/Legal (07):** la comisión Bre-B directa ≈ 0 en el piloto; **vigilar** el posible **decreto de retención ~1,5 %** (guía §5) para el unit economics. Los reembolsos manuales tienen costo operativo (tiempo).
- **Operaciones (05):** definir el **protocolo "pagué y no cayó"** y reembolso, que conecta con `PAGO_HUERFANO` y con el sensor de dispensado / `DISPENSE_FAIL` (ADR-012).

---

## Fuentes

- ADR-004 (pagos por fases), ADR-007 (piloto en Cali), ADR-010 (persona natural), ADR-012 (inventario en servidor) — [`DECISIONS.md`](../DECISIONS.md).
- Registro de llave y alertas de correo Bancolombia — ver fuentes citadas en [`bre-b-guia-negocio.md`](./bre-b-guia-negocio.md) §3-A y §Fuentes.
- Estado actual del código (pago simulado `paid_sim`, `POST /m/{id}/simular-pago`) — [`software/README.md`](../software/README.md), [`software/internal/store/schema.sql`](../software/internal/store/schema.sql).
