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
  internal/store/     capa de datos PostgreSQL vía pgx (máquinas, productos, órdenes, movimientos bancarios)
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

Sirve la **landing pública de la marca** en la raíz (`GET /`, ADR-023), la **página pública por
máquina** (`GET /m/{id}`) con catálogo, precios y stock, y un
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
- Rutas públicas: `GET /` (landing) · `POST /interesados` · **`GET /scan`** · `GET /m/{id}`
  · `POST /m/{id}/pagar` · `GET /m/{id}/orden/{jti}/estado` · `POST /m/{id}/orden/{jti}/reemitir`
  · `POST /m/{id}/simular-pago` (solo con `-allow-sim`).
- **Escáner del QR de la máquina (`GET /scan`, ADR-025):** abre la cámara del celular y lleva al
  cliente a la tienda de la máquina. Es una **capa nueva**: no toca el contrato del token, la
  conciliación ni `/m/{id}`. Detalles en [Escáner](#escáner-del-qr-get-scan).
- **Interesados (landing-v2):** `POST /interesados` valida nombre/tipo de espacio/ciudad/WhatsApp,
  filtra bots (honeypot + tope de 5 envíos por IP cada 10 min), **guarda el lead en la tabla
  `leads`** y redirige a `/?gracias=1`. `space_type` solo acepta `conjunto|oficina|negocio|otro`.
  PII mínima: el log solo deja `id`, origen, tipo de espacio y ciudad (el nombre y el WhatsApp
  viven en la base). Se consultan en el panel: **Interesados** (`GET /admin/leads`).
- **Re-emitir el QR:** `POST /m/{id}/orden/{jti}/reemitir` vuelve a firmar el token de una orden ya
  pagada cuyo QR venció sin dispensar. Conserva el `jti` (un solo uso, contrato §3) y solo renueva
  `exp`; no aplica a órdenes pendientes, dispensadas ni canceladas.
- Rutas admin: `GET /admin`, `POST /admin/machines`, `POST /admin/products`, `GET /admin/m/{id}`,
  `POST /admin/m/{id}/slot`, `GET /admin/orders`, `GET /admin/movements`, `GET /admin/leads`.
- **Interesados en el panel** (`GET /admin/leads`, landing-v1 §4): lista los leads de `leads`
  (fecha, nombre, tipo de espacio, ciudad, WhatsApp, origen), más recientes primero. Es **PII**:
  ruta protegida por sesión, nunca en páginas públicas.

### Escáner del QR (`GET /scan`)

Página server-rendered (`internal/web/scan.go` + `templates/scan.html`) que abre la cámara del
celular, lee el QR pegado en la máquina y redirige a su tienda. Es el destino del **"Volver a
escanear"** de las páginas de error (cubre el caso de teclear `grabi.napi.lat/M001`, sin `/m/`).

- **Decodificación en el navegador** con **jsQR 1.4.0 autohospedado** en
  `internal/web/static/vendor/` (embebido en el binario, servido en `/static/vendor/jsQR.js`).
  **Nunca por CDN**: es la página que decide a dónde navega el cliente. Procedencia y hash en
  [`static/vendor/README.md`](internal/web/static/vendor/README.md). El vídeo **no sale del
  dispositivo**: el servidor no recibe ni un frame.
- **Validación del destino (seguridad).** El contenido de un QR es texto que cualquiera puede
  imprimir y pegar encima. La página **nunca navega al URL del QR**: solo acepta un URL `http(s)`
  del **mismo host**, con ruta `/m/{id}` e `id` que case con `machineIDPattern` (`^M\d{3,}$`), y
  entonces construye ella misma la ruta **relativa** `/m/{ID}`. Cualquier otra cosa → "ese QR no es
  de una máquina GRABI" y sigue escaneando. Un QR de phishing no puede sacar al cliente del sitio.
- **Fallbacks:** sin permiso / sin `getUserMedia` → aviso + botón "Permitir cámara"; y siempre
  **"Escribir el ID manualmente"**. Ese formulario es un `GET /scan?m=M001` de verdad: **sin
  JavaScript lo valida y redirige el servidor** (misma regex, ver `handleScan`), con JS se
  resuelve sin recargar. Un `m` inválido re-pinta el formulario con el error; **nunca** redirige.
- **Auto-off:** al redirigir, al ocultarse la pestaña y en `pagehide` se paran los tracks (se
  libera la cámara y se apaga el indicador del sistema).
- **Ritmo del escaneo (ADR-025 addendum):** se decodifica como mucho **~9 veces por segundo**
  (`SCAN_MS`), no en cada frame, y **no se decodifica mientras el bloque "Escribir el ID
  manualmente" está abierto** (es cuando el usuario teclea y necesita el hilo libre; la cámara sigue
  encendida para reanudar al instante). Decodificar en cada frame dejaba el hilo principal al 100 %
  en el celular y se notaba como teclado lento.
- **Verificado (2026-08-11):** QR reales generados con `internal/qr` decodificados por el jsQR
  vendorizado, enrutando a `/m/M001` y `/m/M0042`, y **rechazando** `https://evil.example/m/M001`
  y `https://grabi.napi.lat/admin`. Estados (escaneando / sin cámara / ID inválido) revisados a
  390 px sin scroll horizontal.

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

---

## Despliegue (Docker + Postgres + AWS) — ADR-020 / ADR-021

El backend corre en **contenedor** con **PostgreSQL** (antes SQLite). Config por
**variables de entorno** (los secretos NO van al repo; en producción viven en un
`.env` git-ignored de la EC2 con permisos 600, ver ADR-021). Variables:

| Variable | Uso | Ejemplo |
|----------|-----|---------|
| `DATABASE_URL` | Conexión Postgres (obligatoria) | `postgres://user:pass@host:5432/grabi?sslmode=require` |
| `DSP_PRIVATE_KEY` | Llave privada Ed25519 (firma del QR) | *(base64; alternativa a `.keys/private-k1.key`)* |
| `ADMIN_PASS` / `ADMIN_USER` | Panel admin | `...` / `admin` |
| `GRABI_IMAP_HOST/PORT/USER/PASS` | Conciliación por correo | `imap.gmail.com` / `993` / `grabibot@gmail.com` / *(App Password)* |
| `GRABI_BREB_KEY_M001` | Llave Bre-B de cobro de la máquina | `009...` |
| `GRABI_MATCH_MODE` | `unique_amount` = fallback legado (opcional) | *(vacío = modo nombre, ADR-018)* |
| `GRABI_WHATSAPP` | Nº público de WhatsApp de la landing (opcional) | `573001234567` · *(vacío = no se muestra el botón)* |

### Correr local (Docker)

```bash
cd software
docker compose up --build          # levanta Postgres + web (con -seed -allow-sim)
# → http://localhost:8080/m/M001   |  panel: /admin/login
```

### Probar la migración (tests contra Postgres)

```bash
cd software
go mod tidy                         # 1a vez: resuelve pgx y limpia deps de SQLite
TEST_DATABASE_URL="postgres://grabi:grabi@localhost:5432/grabi_test?sslmode=disable" go test -p 1 ./...
```

**Importante:** los tests apuntan a **`grabi_test`** (base aparte), NO a `grabi`
(la de la app). Los tests hacen `TRUNCATE`, así que si se corren contra `grabi`
**borran el seed/los datos** de la app. La base `grabi_test` la crea el contenedor
en el primer arranque (`software/initdb/`). Usar **`-p 1`**: los paquetes `store`,
`concil` y `web` comparten la base de pruebas y hacen `TRUNCATE` al iniciar, así que deben
correr en serie (si no, el reset de uno pisa los datos del otro). En **CI** (`.github/workflows/ci.yml`) ya
va con `-p 1` contra un servicio `postgres:16` — ahí se valida la migración en cada push.

### Producción (AWS, ADR-020 / ADR-021)

**EC2 pequeña** (`t3`/`t4g.micro`, free tier) corriendo **este mismo `docker-compose`**
(contenedor web + Postgres), por **control total** y costo casi nulo en el piloto. Se
descartó **App Runner** (cerrado a clientes nuevos el 2026-04-30). **TLS/HTTPS** con un
reverse-proxy en la caja (Caddy/nginx + Let's Encrypt) para `grabi.napi.lat`. **Postgres**
va como contenedor (no RDS aún); **backups** con `pg_dump`→S3 por cron. **Fotos** en un
volumen persistente de la EC2 (sobreviven al redeploy → S3 opcional al escalar). Los
secretos (`DATABASE_URL`, `GRABI_IMAP_*`, `DSP_PRIVATE_KEY`, `ADMIN_PASS`, `GRABI_BREB_KEY_*`)
viven en un `.env` git-ignored de la EC2 (permisos 600), nunca en la imagen. CI/CD por
**GitHub Actions + OIDC** (sin llaves de larga vida). **Crecimiento:** ECS Fargate + RDS
cuando haya varias máquinas.
