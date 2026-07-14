# Contrato del Token de Dispensado — v2

> **Fuente de verdad** de la interfaz entre [Software/Web (02)](../departamentos/02-software-web.md)
> y [Firmware (03)](../departamentos/03-firmware-electronica.md). El servidor **emite y firma**;
> la máquina **verifica y dispensa**. Ambos lados DEBEN implementar exactamente lo que dice
> este documento. Cambios ⇒ nueva versión + entrada en `DECISIONS.md`.

**Versión:** 2 · **Estado:** APROBADA por Daniel (2026-07-14) · **Reemplaza:** v1

> **Cambio v1 → v2 (ADR-006):** se eliminan del payload los campos `iss` e `iat` para
> **adelgazar el token** y dar holgura de tamaño de QR con pedidos de varios items. `iss` era
> constante (la máquina ya asume el emisor) e `iat` era solo auditoría (se registra en el
> servidor, no hace falta en el token; la máquina solo necesita `exp`). Ahorro ≈ 40 chars.
> **Los vectores de prueba v1 quedan obsoletos y deben regenerarse para v2.**

---

## 1. Resumen

- **Formato:** JWS compacto (estilo JWT) → `base64url(header).base64url(payload).base64url(firma)`.
- **Algoritmo de firma:** **Ed25519** (`alg: "EdDSA"`, RFC 8032 / SHA-512 — ver ADR-008).
- **Quién tiene qué:** servidor = llave privada; máquina = **solo** la llave pública.
- **Transporte:** el string del token se codifica en un **QR** que el cliente muestra al lector.
- **Verificación:** 100% **offline** en la máquina.

## 2. Header

```json
{ "alg": "EdDSA", "typ": "DSP", "kid": "k1" }
```

| Campo | Significado |
|-------|-------------|
| `alg` | Siempre `"EdDSA"`. La máquina RECHAZA cualquier otro valor (evita ataques de downgrade). |
| `typ` | `"DSP"` (dispensado). Distingue de otros tokens. |
| `kid` | Id de la llave usada. Permite rotar/revocar llaves sin cambiar todas las máquinas. La máquina guarda un mapa `kid → llave pública`. |

## 3. Payload (v2 — adelgazado)

```json
{
  "mid": "M001",
  "jti": "b3f1c9a7d2",
  "exp": 1752461100,
  "items": [
    { "s": 3, "q": 1 },
    { "s": 5, "q": 2 }
  ]
}
```

| Campo | Tipo | Obligatorio | Regla de validación en la máquina |
|-------|------|-------------|-----------------------------------|
| `mid` | string | sí | Debe ser **igual** al `machine_id` de ESTA máquina. Si no ⇒ `WRONG_MACHINE`. |
| `jti` | string | sí | Id único de la orden. Si ya está en la lista de usados ⇒ `ALREADY_USED`. |
| `exp` | int (epoch s) | sí | Si `now > exp` ⇒ `EXPIRED`. `now` viene del RTC. |
| `items` | array | sí | Lista de `{ s: slot (int), q: cantidad (int ≥1) }`. La máquina dispensa cada uno. |

> **Eliminados en v2:** `iss` (constante, implícito) e `iat` (auditoría, se guarda solo en el
> servidor). El servidor SIGUE registrando `iat` y el emisor en su base de datos; simplemente
> no viajan en el QR.

**Ventana de expiración:** el servidor pone `exp = now_emisión + 300` (5 min) por defecto.
Configurable en el servidor. Suficiente para escanear, corto para limitar abuso si el QR se filtra.

## 4. Codificación y firma (lado servidor)

1. Serializar header y payload como JSON **compacto** (sin espacios). Orden de claves sugerido: `mid, jti, exp, items`.
2. `signing_input = base64url(header) + "." + base64url(payload)` (base64url **sin padding**).
3. `firma = Ed25519_sign(clave_privada, signing_input)` → 64 bytes.
4. `token = signing_input + "." + base64url(firma)`.
5. Generar QR con el string `token`. Recomendado: **ECC nivel M**, y validar longitud (ver §6).

## 5. Verificación (lado máquina) — orden exacto

```
1. Separar el token en 3 partes por ".". Si no hay 3 partes ⇒ MALFORMED.
2. Decodificar header. Verificar alg=="EdDSA", typ=="DSP". Obtener kid.
3. Buscar la llave pública para ese kid. Si no existe ⇒ UNKNOWN_KEY.
4. Verificar la firma Ed25519 sobre (parte1 + "." + parte2). Si falla ⇒ BAD_SIGNATURE.
5. Decodificar payload.
6. mid == MACHINE_ID                      si no ⇒ WRONG_MACHINE
7. now (RTC) <= exp                        si no ⇒ EXPIRED
8. jti NO está en usados                   si no ⇒ ALREADY_USED
9. Guardar jti como usado (persistente) ANTES de dispensar.
10. Para cada item: dispensar slot s, cantidad q, esperando sensor de confirmación.
11. Registrar resultado (OK / fallo por slot) en log local.
```

> **Regla clave:** marcar `jti` como usado **antes** de accionar motores, para que un reinicio
> a mitad no permita un segundo dispensado. Manejar el fallo de sensor como incidencia de soporte.

## 6. Presupuesto de tamaño del QR

- **Objetivo:** token ≤ ~300 caracteres → QR cómodo y legible por el GM65 desde pantalla de celular.
- Con v2 (sin `iss` ni `iat`): header ~51 + firma ~86 + 2 puntos = 139 fijos; payload base (1 item)
  baja de ~160 a ~120 chars. Estimado: **1 item ≈ 259 · 2 items ≈ 278 · 3 items ≈ 297**. Un pedido
  normal (2–3 productos) queda **dentro del objetivo**, que era justo lo que buscábamos con v2.
- **Pendiente de confirmar con hardware:** que el GM65 lea bien ese QR desde varios celulares
  (checklist §11). Si aún hiciera falta más margen, la reserva es formato binario **COSE/CBOR**.

## 7. Códigos de error (feedback al usuario)

| Código | Causa | Qué ve/oye el cliente |
|--------|-------|-----------------------|
| `MALFORMED` | QR no es un token válido | "QR no válido" |
| `BAD_SIGNATURE` | Firma inválida | "QR no válido" |
| `UNKNOWN_KEY` / `WRONG_MACHINE` | Token no es para esta máquina | "Este código no es de esta máquina" |
| `EXPIRED` | Venció la ventana | "El código expiró, vuelve a comprar" |
| `ALREADY_USED` | Reuso | "Este código ya fue usado" |
| `DISPENSE_FAIL` | Sensor no confirmó salida | "Hubo un problema, contáctanos" + registrar para reembolso |

> `BAD_ISSUER` se elimina en v2 (ya no hay campo `iss`).

## 8. Reloj (RTC)

- La máquina usa **RTC DS3231** para `now`. Se ajusta al aprovisionar y se puede resincronizar
  cuando la máquina tenga wifi ocasional (opcional).
- Como `exp` es de minutos, un pequeño desfase del RTC es aceptable; mantenerlo en hora en cada visita.

## 9. Aprovisionamiento (qué se carga en cada máquina)

- `MACHINE_ID` (ej. `"M001"`).
- Mapa `kid → llave pública` (una o varias, para rotación).
- Hora del RTC.
- (Config) ventana de tolerancia, mapa de slots↔motores.

## 10. Vectores de prueba

> **Regenerar para v2.** Los vectores v1 (con `iss`/`iat`) quedan obsoletos. Tarea del agente de
> Software (02): con `dsp vectors` producir de nuevo `token-valido`, `token-expirado`,
> `token-firma-mala` (+ `token-valido.png` y `resultados-esperados.md`) usando el payload v2.
> El firmware (03) debe re-verificar contra ellos y seguir dando resultados idénticos al backend.

Estructura en `especificaciones/vectores-prueba/`:
```
llave-publica-k1.txt        (base64)
token-valido.txt / .png
token-expirado.txt
token-firma-mala.txt
resultados-esperados.md
```

## 11. Checklist de v2 (para cerrar)

- [x] Daniel aprueba algoritmo y campos (v2, sin `iss`/`iat`).
- [ ] Software (02): actualizar `dsptoken` al payload v2 y **regenerar vectores**.
- [ ] Firmware (03): actualizar la PoC al payload v2 y re-verificar contra los vectores.
- [ ] Confirmar con el GM65 real que el QR de 2–3 items se lee bien desde varios celulares.

---

### Historial de versiones
- **v2 (2026-07-14):** adelgazado. Se quitan `iss` e `iat` del payload (ADR-006). Vectores a regenerar.
- **v1 (2026-07-14):** propuesta inicial. JWT/JSON + Ed25519. Validada en PoC (ADR-008) antes del cambio de tamaño.
