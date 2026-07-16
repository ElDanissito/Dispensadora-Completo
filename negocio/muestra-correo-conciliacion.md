# Muestra real del correo de alerta (para calibrar el parser de conciliación)

> Basado en un correo de alerta real de Bancolombia recibido en grabibot (2026-07-16).
> **Datos personales anonimizados** (el repo es público). El `.eml` real NO se sube al repo.

## Identificación del correo (para filtrar/validar)

- **From:** `Alertas y Notificaciones <alertasynotificaciones@an.notificacionesbancolombia.com>`
- **Subject:** `Alertas y Notificaciones`
- **Formato:** `multipart/alternative` con `text/plain` y `text/html`. **Usar el `text/plain`** (es limpio y estable).
- Llega a grabibot **reenviado** desde el correo personal (el header `To` es el personal; NO usarlo para nada, el servicio simplemente lee el buzón de grabibot).

## Frase clave del cuerpo (text/plain)

```
¡Listo! Todo salió bien con tus movimientos Bancolombia: GRABI M001, recibiste
un pago de NOMBRE PAGADOR por $2.00 en tu cuenta *5322 conectado a la
llave 0092699654 el 16/07/2026 a las 02:47.
```

## Campos a extraer (regex sugeridas, sobre el text/plain con saltos de línea normalizados a espacios)

| Campo | Regex | Ejemplo |
|-------|-------|---------|
| **Máquina** (`mid`) | `movimientos Bancolombia:\s*(GRABI\s+M\d+)` | `GRABI M001` |
| **Pagador** | `recibiste\s+un\s+pago\s+de\s+(.+?)\s+por\s+\$` | `Nombre Apellido` |
| **Monto** | `por\s+\$\s*([\d.,]+)` | `2.00` |
| **Cuenta** (enmascarada) | `en\s+tu\s+cuenta\s+(\*\d+)` | `*5322` |
| **Llave** | `conectado\s+a\s+la\s+llave\s+(\d+)` | `0092699654` |
| **Fecha** | `el\s+(\d{2}/\d{2}/\d{4})` | `16/07/2026` |
| **Hora** | `a\s+las\s+(\d{2}:\d{2})` | `02:47` |

**Normalización del monto:** quitar separadores de miles y quedarse con el número. En pesos colombianos el valor es entero; Bancolombia lo muestra con 2 decimales (`$2.00` = 2 pesos). El parser debe tolerar formatos tipo `$1,234.00`.

**Zona horaria:** la hora del **cuerpo** (`a las 02:47`) es **hora local Colombia (America/Bogotá, UTC-5)**. El header `Date` viene en UTC (ej. `07:47 +0000`). Para el matching usar preferiblemente la hora de recepción del correo o la del cuerpo, siempre en la misma zona.

## Estrategia de conciliación (importante)

El correo **NO trae una referencia/id de orden**. Entonces el matching orden↔pago se hace por:

1. **Máquina** → el `GRABI M00X` del correo.
2. **Monto exacto** → por eso conviene cobrar **montos únicos por orden** (variar los últimos pesos, ej. 2003, 2017…, ya que no hay centavos transferibles).
3. **Ventana de tiempo** → el pago debe caer dentro de X minutos de la orden creada (ej. la ventana de `exp` del token, 5 min, con algo de margen).

Con (máquina + monto único + ventana), la atribución es inequívoca sin depender de referencia.

## Casos borde a manejar

- Pago sin orden pendiente que calce (monto/tiempo no coincide) → registrar como "no conciliado" para revisión/soporte.
- Monto distinto al esperado → no conciliar; flujo de corrección.
- Pago duplicado (dos correos) → idempotencia: no dispensar dos veces por el mismo movimiento.
- Correo que no calza el patrón → registrar y alertar (posible cambio de formato del banco).
