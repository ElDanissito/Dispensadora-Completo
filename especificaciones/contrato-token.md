# Contrato del Token de Dispensado — v1

> **Fuente de verdad** de la interfaz entre [Software/Web (02)](../departamentos/02-software-web.md)
> y [Firmware (03)](../departamentos/03-firmware-electronica.md). El servidor **emite y firma**;
> la máquina **verifica y dispensa**. Ambos lados DEBEN implementar exactamente lo que dice
> este documento. Cambios ⇒ nueva versión (v2) + entrada en `DECISIONS.md`.

**Versión:** 1 · **Estado:** propuesta (pendiente de aprobación de Daniel) · **Fecha:** 2026-07-14

---

## 1. Resumen

- **Formato:** JWS compacto (estilo JWT) → `base64url(header).base64url(payload).base64url(firma)`.
- **Algoritmo de firma:** **Ed25519** (`alg: "EdDSA"`).
- **Quién tiene qué:** servidor = llave privada; máquina = **solo** la llave pública.
- **Transporte:** el string del token se codifica en un **QR** que el cliente muestra al lector.
- **Verificación:** 100% **offline** en la máquina.

## 2. Header

```json
{ "alg": "EdDSA", "typ": "DSP", "kid": "k1" }
```

| Campo | Significado |
|-------|-------------|
| `alg` | Siempre `"EdDSA"` en v1. La máquina RECHAZA cualquier otro valor (evita ataques de downgrade). |
| `typ` | `"DSP"` (dispensado). Ayuda a distinguir de otros tokens. |
| `kid` | Id de la llave usada. Permite rotar/revocar llaves sin cambiar todas las máquinas. La máquina guarda un mapa `kid → llave pública`. |

## 3. Payload

```json
{
  "iss": "dispensadoras.co",
  "mid": "M001",
  "jti": "b3f1c9a7d2",
  "iat": 1752460800,
  "exp": 1752461100,
  "items": [
    { "s": 3, "q": 1 },
    { "s": 5, "q": 2 }
  ]
}
```

| Campo | Tipo | Obligatorio | Regla de validación en la máquina |
|-------|------|-------------|-----------------------------------|
| `iss` | string | sí | Debe ser `"dispensadoras.co"`. |
| `mid` | string | sí | Debe ser **igual** al `machine_id` de ESTA máquina. Si no, rechazar (`WRONG_MACHINE`). |
| `jti` | string | sí | Id único de la orden. Si ya está en la lista de usados ⇒ rechazar (`ALREADY_USED`). |
| `iat` | int (epoch s) | sí | Momento de emisión. Informativo/auditoría. |
| `exp` | int (epoch s) | sí | Si `now > exp` ⇒ rechazar (`EXPIRED`). `now` viene del RTC. |
| `items` | array | sí | Lista de `{ s: slot (int), q: cantidad (int ≥1) }`. La máquina dispensa cada uno. |

**Ventana de expiración recomendada:** `exp = iat + 300` (5 min). Suficiente para que el
cliente escanee, corto para limitar abuso si el QR se filtra. Ajustable por configuración del servidor.

**Notas de diseño para mantener el QR pequeño:**
- Slots numéricos (`s`) y cantidades (`q`), no nombres de producto.
- Nada de campos innecesarios. Mantener el payload mínimo.

## 4. Codificación y firma (lado servidor)

1. Serializar header y payload como JSON **compacto** (sin espacios).
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
6. iss == "dispensadoras.co"           si no ⇒ BAD_ISSUER
7. mid == MACHINE_ID                     si no ⇒ WRONG_MACHINE
8. now (RTC) <= exp                       si no ⇒ EXPIRED
9. jti NO está en usados                  si no ⇒ ALREADY_USED
10. Guardar jti como usado (persistente) ANTES de dispensar.
11. Para cada item: dispensar slot s, cantidad q, esperando sensor de confirmación.
12. Registrar resultado (OK / fallo por slot) en log local.
```

> **Regla clave:** marcar `jti` como usado **antes** de accionar motores, para que un reinicio
> a mitad no permita un segundo dispensado. Manejar el fallo de sensor como incidencia de soporte.

## 6. Presupuesto de tamaño del QR

- El GM65 lee QR de pantalla de celular bien hasta cierto tamaño; **objetivo: token ≤ ~300 caracteres** para un QR cómodo (versión de QR baja/media, buena tolerancia).
- Header+firma ocupan ~120–130 chars. El resto es el payload → mantener `items` corto.
- **Si un pedido con muchos items excede el presupuesto:** opciones (decidir en v2 si ocurre):
  (a) limitar nº de items por orden; (b) migrar a un formato binario firmado **COSE/CBOR**
  (más compacto que JWT). Por ahora v1 usa JWT/JSON por simplicidad de implementación.

## 7. Códigos de error (feedback al usuario)

| Código | Causa | Qué ve/oye el cliente |
|--------|-------|-----------------------|
| `MALFORMED` | QR no es un token válido | "QR no válido" |
| `BAD_SIGNATURE` | Firma inválida | "QR no válido" |
| `UNKNOWN_KEY` / `BAD_ISSUER` / `WRONG_MACHINE` | Token no es para esta máquina | "Este código no es de esta máquina" |
| `EXPIRED` | Venció la ventana | "El código expiró, vuelve a comprar" |
| `ALREADY_USED` | Reuso | "Este código ya fue usado" |
| `DISPENSE_FAIL` | Sensor no confirmó salida | "Hubo un problema, contáctanos" + registrar para reembolso |

## 8. Reloj (RTC)

- La máquina usa **RTC DS3231** para `now`. Se ajusta al aprovisionar y se puede resincronizar
  cuando la máquina tenga wifi ocasional (opcional).
- Tolerancia: como `exp` es de minutos, un pequeño desfase del RTC es aceptable; aún así,
  mantener el RTC en hora en cada visita de mantenimiento.

## 9. Aprovisionamiento (qué se carga en cada máquina)

- `MACHINE_ID` (ej. `"M001"`).
- Mapa `kid → llave pública` (una o varias, para rotación).
- Hora del RTC.
- (Config) ventana de tolerancia, mapa de slots↔motores.

## 10. Vectores de prueba (para que ambos lados prueben igual)

> **Pendiente de generar.** Tarea conjunta 02+03: el servidor genera un par de llaves de
> prueba y produce 3 tokens de ejemplo (válido, expirado, firma corrupta) + la llave pública.
> Se guardan aquí en `especificaciones/vectores-prueba/` para que el firmware valide contra
> ellos **sin hardware**. El [simulador de verificación](../departamentos/02-software-web.md)
> del backend debe producir exactamente los mismos resultados que el ESP32.

Ejemplo de estructura a generar:
```
especificaciones/vectores-prueba/
  llave-publica-k1.txt        (base64)
  token-valido.txt
  token-expirado.txt
  token-firma-mala.txt
  resultados-esperados.md     (qué código de error debe dar cada uno)
```

## 11. Checklist de aprobación (antes de codificar en firme)

- [ ] Daniel aprueba algoritmo, campos y ventana de expiración.
- [ ] Confirmado que el GM65 lee un QR de ~300 chars desde varios celulares.
- [ ] Generados los vectores de prueba.
- [ ] Simulador (02) y firmware (03) dan resultados idénticos sobre los vectores.

---

### Historial de versiones
- **v1 (2026-07-14):** propuesta inicial. JWT/JSON + Ed25519.
