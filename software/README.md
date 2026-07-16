# software — backend y herramientas del token de dispensado

Módulo Go del [Departamento 02 · Software/Web](../departamentos/02-software-web.md).
Esta primera tanda entrega el núcleo criptográfico: **firmar y verificar** el token de
dispensado (JWS Ed25519) según [`especificaciones/contrato-token.md`](../especificaciones/contrato-token.md) v2,
generar su **QR** y producir los **vectores de prueba** para que Firmware valide sin hardware.

## Estructura

```
software/
  cmd/dsp/            CLI `dsp` (keygen, sign, verify, qr, vectors)
  cmd/server/         servidor web (página pública /m/{id} + panel admin)
  internal/dsptoken/  firma + verificación (referencia canónica del contrato)
  internal/qr/        generación de QR PNG
  internal/store/     capa de datos SQLite (máquinas, productos, órdenes)
  internal/web/       handlers HTTP + plantillas html/template
  .keys/              llaves privadas — IGNORADO por git (nunca commitear)
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
- Rutas: `GET /m/{id}` (pública) · `POST /m/{id}/simular-pago` (emite el QR) · `GET /admin`,
  `POST /admin/machines`, `POST /admin/products`, `GET /admin/m/{id}`, `POST /admin/m/{id}/slot`,
  `GET /admin/orders`.

### Ciclo web→máquina (pago simulado, para pruebas)

`GET /m/{id}` muestra el catálogo como **formulario**: el cliente elige cantidades y pulsa
**"Simular pago y generar QR"**. Ese `POST /m/{id}/simular-pago`:

1. Valida la selección contra el catálogo (slots existentes, stock suficiente).
2. Crea la **orden** (`store.CreateOrder`) con estado `paid_sim`.
3. **Firma el token v2** (`dsptoken.Sign`) con la llave privada del servidor (kid de la máquina).
4. Muestra el **QR** del token embebido en la página (`data:image/png;base64,...`) para escanearlo.

Requiere la **llave privada** cargada: `software/.keys/private-k1.key` (de `dsp keygen`) o la
env `DSP_PRIVATE_KEY`. Sin llave, el servidor arranca igual pero "simular pago" responde 503.

> **Seguridad (CLAUDE.md §4):** `simular-pago` **NO** es un pago real — es un atajo de pruebas.
> La orden queda marcada `paid_sim` para distinguirla. Bre-B real (Dept. 04) emitirá el QR solo
> tras la **notificación real** de la cuenta; nunca se confía en el comprobante del cliente.

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

Entregado: `dsp` (keygen/sign/verify/qr/vectors, contrato **v2**) + `server` con `GET /m/{id}`,
**ciclo web→máquina con pago simulado** (`POST /m/{id}/simular-pago` → orden + token firmado + QR),
panel admin mínimo + capa de datos SQLite + tests. Verificado de punta a punta: el token emitido
por la web da `OK` en el simulador con el `NOW` fijo del firmware (mismos items del pedido).
Siguiente (Dept. 02 §6): integración de **pago real Bre-B** con Dept. 04 (emitir el QR solo tras la
notificación confirmada), descontar stock al confirmar, refinar estados de orden, y **deploy en VPS
con dominio + TLS** (Caddy).

**Fix (2026-07-16):** `dsp vectors`/tests corrompían el ÚLTIMO carácter base64url de la firma para
generar `token-firma-mala`; ese carácter solo lleva 2 bits significativos, así que a veces la firma
quedaba **intacta** (test flaky + riesgo de un vector "malo" en realidad válido). Ahora se corrompe
el PRIMER carácter (6 bits significativos). Los vectores commiteados ya eran genuinamente inválidos
(no hubo que regenerarlos).

Migración a **v2** del token registrada en [`DECISIONS.md`](../DECISIONS.md) (ADR-006):
2 items = 258 chars, holgado bajo el objetivo de ~300 del §6.
