# Paso 5b — Anti-reuso de `jti` persistente en NVS

Refinamiento del [paso 5](../paso5-dispensado-sensor/). El flujo es idéntico
(leer QR → verificar token v2 → dispensar → confirmar caída por sensor), pero el
**registro de `jti` usados** deja de vivir solo en RAM y pasa a **NVS** (la
partición no volátil de la flash del ESP32, vía la librería `Preferences`).

## Por qué

Con el registro en RAM, un **reinicio o apagón** borraba la lista de `jti`
usados: el mismo QR volvía a dar `OK` y se podía **re-dispensar**. Con NVS, el
`ALREADY_USED` **sobrevive a reinicios/cortes de energía**, que es justo lo que
el contrato (§5) y el brief de firmware exigen para el anti-reuso.

## Qué cambió respecto al paso 5

- `#include <Preferences.h>`.
- El array en RAM ahora es una **caché** de lo que hay en NVS:
  - `jti_load_from_nvs()` — en `setup()`, recarga la caché desde flash.
  - `jti_mark_used()` — escribe **primero en flash** (commit síncrono) y luego
    en la caché. Se sigue llamando en el **paso 9** de `verificar_token()`, es
    decir **antes** de accionar motores (regla clave del §5). Como la escritura
    a flash es síncrona, cuando `putString`/`putInt` retornan el dato ya está
    comprometido: si la energía se corta durante el giro del motor, al volver el
    `jti` ya figura como usado.
  - `jti_reset_all()` — borra el registro (solo para pruebas, ver abajo).
- Orden de escritura crash-safe: se guarda el `jti` (`j<i>`) y **después** el
  contador (`n`). Si se cortara la energía entre ambos, al reiniciar `n` es el
  viejo y el `jti` a medio escribir se ignora — pero eso pasa **antes** de
  dispensar, así que no hay entrega: estado consistente, sin doble dispensado.

### Layout en NVS (namespace `dsp`)

| Clave   | Tipo   | Contenido                          |
|---------|--------|------------------------------------|
| `n`     | int    | Cuántos `jti` hay guardados.       |
| `j<i>`  | string | El `jti` i-ésimo (`i = 0..n-1`).   |

Con `JTI_MAX = 64` la clave más larga es `j63` (3 chars), muy por debajo del
límite de 15 chars de las claves NVS.

## Compilar y cargar (Arduino IDE)

1. Placa: **ESP32 Dev Module**. `Preferences` viene con el core del ESP32 (no
   hay que instalar nada).
2. Abrir `paso5b-jti-nvs.ino` (los 4 archivos de Monocypher están en la carpeta).
3. Si al cargar sale `Write timeout`: ver el checklist de `hardware/inventario-actual.md`
   (mantener BOOT, desconectar motores por el GPIO12, Upload Speed 115200).

## Prueba de aceptación (la que valida esta tarea)

1. **Escanear el `token-valido`** → debe imprimir `Resultado: OK` y dispensar.
2. **Reiniciar la placa** (botón EN/RST, o desconectar y reconectar la energía).
   Al arrancar debe verse: `jti usados cargados de NVS: 1`.
3. **Escanear el MISMO QR otra vez** → debe imprimir `Resultado: ALREADY_USED`
   y **no** dispensar.

Antes (paso 5), tras el reinicio el segundo escaneo daba `OK` de nuevo. Ahora da
`ALREADY_USED`: el anti-reuso es persistente. ✔

### Repetir el ensayo desde cero

Enviar por el Monitor Serie (USB) el texto **`!reset`** y pulsar enviar: borra el
registro de `jti` en NVS y deja la máquina como recién aprovisionada. No es un
token válido, no afecta la operación normal; es solo para volver a probar.

## Pendiente (no bloquea)

- **Poda de `jti` vencidos por `exp`** cuando llegue el **RTC DS3231**: hoy el
  registro solo crece hasta `JTI_MAX = 64`. Con `exp` disponible se pueden
  descartar los `jti` ya expirados y liberar espacio. Con el volumen del piloto,
  64 es holgado; si se llena, el firmware avisa por Serial y prioriza no bloquear
  la venta (documentado en el código).
- **`NOW` sigue fijo** (`1752460900`) hasta que llegue el RTC.
