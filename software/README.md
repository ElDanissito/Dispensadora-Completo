# software — backend y herramientas del token de dispensado

Módulo Go del [Departamento 02 · Software/Web](../departamentos/02-software-web.md).
Esta primera tanda entrega el núcleo criptográfico: **firmar y verificar** el token de
dispensado (JWS Ed25519) según [`especificaciones/contrato-token.md`](../especificaciones/contrato-token.md) v2,
generar su **QR** y producir los **vectores de prueba** para que Firmware valide sin hardware.

## Estructura

```
software/
  cmd/dsp/            CLI `dsp` (keygen, sign, verify, qr, vectors, concil-parse, concil-login)
  cmd/server/         servidor web (página pública /m/{id} + panel admin + poller de conciliación)
  internal/dsptoken/  firma + verificación (referencia canónica del contrato)
  internal/qr/        generación de QR PNG
  internal/store/     capa de datos SQLite (máquinas, productos, órdenes, movimientos bancarios)
  internal/web/       handlers HTTP + plantillas html/template
  internal/bankmail/  parser de las alertas de correo Bancolombia (GRABI)
  internal/imapmail/  cliente IMAP mínimo (lee el buzón de conciliación grabibot)
  internal/config/    carga de .env + credenciales IMAP + mapa de llaves Bre-B
  internal/concil/    servicio de conciliación (casa pago↔orden, dispara el QR)
  .keys/              llaves privadas — IGNORADO por git (nunca commitear)
  .env                credenciales locales (IMAP, llaves Bre-B) — IGNORADO por git
```

La verificación en `internal/dsptoken` sigue **exactamente** el orden de validaciones del
contrato §5 y los códigos de error del §7. Es la implementación de referencia que el
firmware (Dept. 03) debe reproducir para dar resultados idénticos sobre los vectores.

## Uso

Compilar:

```sh
cd software
go build -o dsp ./cmd/dsp      # en Windows: dsp.exe
```

### Generar el par de llaves

```sh
./dsp keygen
```

Guarda la **privada** en `software/.keys/private-k1.key` (ignorada por git) y la **pública**
en `especificaciones/vectores-prueba/llave-publica-k1.txt`. La privada también puede
inyectarse por la variable de entorno `DSP_PRIVATE_KEY` (base64), sin tocar el disco.

> **Regla no negociable:** la llave privada nunca entra al repo ni sale del servidor.

### Firmar un token (+ QR opcional)

```sh
./dsp sign -mid M001 -items "3:1,5:2" -qr orden.png
```

Imprime el token en stdout y, por stderr, el `jti`, `exp` y la longitud en caracteres.
Por defecto `exp = ahora + 300s` (ventana de 5 min del contrato) y el `jti` es aleatorio.
En v2 el payload es `{mid, jti, exp, items}`: `iss`/`iat` ya no viajan en el token (ADR-006).

### Verificar un token (simulador de la máquina)

```sh
./dsp verify -mid M001 -in especificaciones/vectores-prueba/token-valido.txt -now 1752460900
```

Imprime `OK` o el código de error (`EXPIRED`, `BAD_SIGNATURE`, `WRONG_MACHINE`, …) y sale
con código 3 si no es `OK` (útil para scripts). `-now` fija la hora del RTC simulado.

### Regenerar los vectores de prueba

```sh
./dsp vectors
```

Escribe `token-valido`, `token-expirado`, `token-firma-mala` y `resultados-esperados.md`
en `especificaciones/vectores-prueba/`.

## Servidor web (`cmd/server`)

Sirve la **página pública por máquina** (`GET /m/{id}`) con catálogo, precios y stock, y un
**panel de administración** en `/admin` (crear máquinas, cargar productos, asignar
slot→producto/precio/stock, ver órdenes). Front server-rendered con `html/template`, sin JS
pesado (ADR-002). Datos en **SQLite** (pura-Go, sin cgo).

```sh
go build -o dispensadoras-web ./cmd/server        # en Windows: dispensadoras-web.exe
ADMIN_PASS=algo-seguro ./dispensadoras-web -seed   # -seed carga datos de demo
```

- `-db dispensadoras.db` ruta del archivo SQLite · `-addr :8080` dirección · `-seed` datos demo.
- El panel `/admin` va protegido con **Basic Auth** (`ADMIN_USER`/`ADMIN_PASS`, por defecto
  `admin`/`changeme` con aviso — define `ADMIN_PASS` antes de exponerlo).
- Rutas públicas: `GET /m/{id}` · `POST /m/{id}/pagar` · `GET /m/{id}/orden/{jti}/estado`
  · `POST /m/{id}/simular-pago` (solo con `-allow-sim`).
- Rutas admin: `GET /admin`, `POST /admin/machines`, `POST /admin/products`, `GET /admin/m/{id}`,
  `POST /admin/m/{id}/slot`, `GET /admin/orders`.

### Ciclo web→máquina (pago REAL Bre-B por conciliación de correo)

`GET /m/{id}` muestra el catálogo como **formulario**: el cliente elige cantidades y pulsa
**"Pagar con Bre-B"**. Ese `POST /m/{id}/pagar`:

1. Valida la selección contra el catálogo (slots existentes, stock suficiente).
2. Calcula un **monto único** = base + **desambiguador** `d` (1–99 pesos) que no colisione con otra
   orden `pending` de la máquina (ancla del matching, spec §2).
3. Crea la **orden** `pending` con `unique_amount` y una **ventana de pago** (`-pay-window`, 15 min).
4. Redirige a `GET /m/{id}/orden/{jti}/estado`: la **pantalla de pago** muestra el valor exacto a
   transferir (resaltando `d`), la **llave Bre-B** de la máquina y una cuenta atrás. Se **auto-refresca**
   cada 4 s con `<meta http-equiv="refresh">` (sin JS pesado, ADR-011bis).

El **QR NO se emite aquí**. Lo emite la **conciliación** (`internal/concil`) cuando la notificación
real de Bancolombia (correo a grabibot) casa con la orden por **(máquina + monto único + ventana)**.
Al casar: firma el token v2 (`dsptoken.Sign`), transiciona la orden `pending → paid` de forma
**atómica** (`store.MarkOrderPaid`), **descuenta stock** (ADR-012) y la pantalla de estado pasa a
mostrar el QR.

> **Seguridad (CLAUDE.md §4 / ADR-004):** la orden solo pasa a `paid` con base en la **notificación
> real de la cuenta**; jamás con el comprobante que muestre el cliente. Idempotencia por `Message-ID`:
> un mismo correo nunca emite dos QR ni descuenta stock dos veces (spec §7.2).

### Conciliación de pagos por correo (`internal/concil`)

El servidor puede correr un **poller** que lee el buzón de grabibot por IMAP, extrae cada abono con
`internal/bankmail` (regex sobre el `text/plain`, decodificando quoted-printable) y lo casa con una
orden. Estados de un abono: `matched` (casó → paga), `orphan` (no casó → soporte/reembolso),
`parse_failed` (cambió el formato → alerta), `discarded` (remitente fuera de la allowlist → seguridad),
`conflict` (>1 orden). Todo se persiste en la tabla `bank_movements` (auditoría, Dept. 07).

```sh
# arranca el servidor CON conciliación (requiere .env con GRABI_IMAP_* y llave privada)
ADMIN_PASS=algo ./dispensadoras-web -concil -concil-interval 12s
```

**Credenciales** (App Password de Gmail, llaves Bre-B) salen SOLO de `software/.env` (git-ignored,
ADR-013). Nunca del repo ni de argumentos en claro.

### Atajo de pruebas (`simular-pago`)

`POST /m/{id}/simular-pago` firma el QR **sin pago real** (orden marcada `paid_sim`, distinguible).
Solo está disponible con el flag **`-allow-sim`**; **nunca** en la ruta pública de producción (spec §8).
Requiere la **llave privada** cargada (`.keys/private-k1.key` o `DSP_PRIVATE_KEY`).

### Herramientas CLI de conciliación

```sh
./dsp concil-parse -in correo.eml   # parsea un .eml y muestra los campos (offline, sin red)
./dsp concil-login -list            # login IMAP a grabibot + lista los abonos no leídos (.env)
```

> **Nota sobre `exp` y el RTC:** el token se firma con `exp = ahora + 300s`. La máquina del piloto
> aún usa un `NOW` fijo (sin RTC), que está en el pasado respecto a ese `exp`, así que el token
> verifica `OK`. Cuando llegue el DS3231, `exp` se validará contra la hora real.

## Pruebas

```sh
go test ./...
```

- `internal/dsptoken`: cada código de error del contrato v2 y el orden de validación
  (p. ej. que la firma se valida antes que la expiración).
- `internal/store`: catálogo (upsert de slots) y órdenes (incl. rechazo de `jti` duplicado).

## Estado y pendientes

Entregado: `dsp` (keygen/sign/verify/qr/vectors/concil-parse/concil-login, contrato **v2**) +
`server` con `GET /m/{id}`, **flujo de pago real Bre-B** (`POST /pagar` → orden `pending` + monto
único → pantalla de pago con auto-refresh → QR al conciliar), **conciliación por correo IMAP**
(`internal/concil` + `bankmail` + `imapmail`) con matching por (máquina + monto único + ventana),
**idempotencia por Message-ID**, descuento de stock (ADR-012), auditoría en `bank_movements`, y el
atajo `simular-pago` tras `-allow-sim`. Panel admin + capa SQLite (con migración) + tests
(`bankmail`, `concil`, `store`, `dsptoken`). **Verificado:** login IMAP a grabibot OK y parseo del
correo real de Bancolombia (todos los campos), flujo web e2e (pago→QR) y match→paid→stock en tests.
Siguiente (Dept. 02 §6): pantalla de "reintentar" al expirar, panel de movimientos/huérfanos, y
**deploy en VPS con dominio + TLS** (Caddy). Evolución (spec §11): QR dinámico → webhook de agregador,
manteniendo el mismo contrato `orden.pagada`.

**Fix (2026-07-16):** `dsp vectors`/tests corrompían el ÚLTIMO carácter base64url de la firma para
generar `token-firma-mala`; ese carácter solo lleva 2 bits significativos, así que a veces la firma
quedaba **intacta** (test flaky + riesgo de un vector "malo" en realidad válido). Ahora se corrompe
el PRIMER carácter (6 bits significativos). Los vectores commiteados ya eran genuinamente inválidos
(no hubo que regenerarlos).

Migración a **v2** del token registrada en [`DECISIONS.md`](../DECISIONS.md) (ADR-006):
2 items = 258 chars, holgado bajo el objetivo de ~300 del §6.
